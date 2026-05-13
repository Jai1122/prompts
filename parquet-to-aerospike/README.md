# parquet-to-aerospike

Streams rows from a directory of sharded parquet files into an Aerospike cluster as upserts.

## Build

```bash
cd parquet-to-aerospike
go mod tidy
go build -o p2a .
```

## Run (single process)

```bash
./p2a \
  -parquet-dir /data/events \
  -file-pattern 'part-*.parquet' \
  -as-hosts 'as-1.prod:3000,as-2.prod:3000,as-3.prod:3000' \
  -as-namespace events \
  -as-set raw \
  -key-column event_id \
  -file-workers 16 \
  -write-workers 1024
```

## Scaling to 10B rows in 30 min (~5.5M writes/sec)

That throughput is far beyond what a single client box can sustain — NIC, CPU,
and Aerospike per-node client-fd limits all become the wall. Run the script as
a **fleet** of N processes (one per box), each owning a deterministic slice of
the parquet files:

```bash
# on box 0 of 10
./p2a ... -file-shard-total 10 -file-shard-index 0

# on box 1 of 10
./p2a ... -file-shard-total 10 -file-shard-index 1
# ...
```

File assignment is `index = i % shard-total` over the alphabetically sorted
glob, so two processes never read the same file.

### Sizing rule of thumb

| Knob | Start | Push up when... | Push down when... |
| --- | --- | --- | --- |
| `-write-workers` | 512 | CPU < 60%, AS not saturated | retries spike, `DEVICE_OVERLOAD` errors |
| `-file-workers`  | NumCPU | reader queue often empty | RSS climbing too fast |
| `-queue-size`    | 50000 | writers idle waiting for rows | memory pressure |
| `-write-timeout` | 200ms | high tail latency on AS | retries from premature timeouts |

Watch the `progress:` log line for `rate=...`. Multiply by your fleet size to
project total throughput; if you're below target, add boxes before tuning.

## Design notes

- **One goroutine per file, bounded by `-file-workers`** — parquet readers are
  CPU-bound (decompress + decode); too much fan-out thrashes cache.
- **Bounded channel of `rowJob`** — backpressure: readers block when writers
  can't keep up, no unbounded buffering.
- **Per-record `client.Put` with custom retry** — Aerospike has no batch-write
  API for arbitrary records the way it does for batch-read. The Go client
  multiplexes concurrent Puts onto its per-node connection pool, so a pool of
  hundreds of writer goroutines is the right pattern.
- **`RecordExistsAction = UPDATE`** matches the upsert requirement and makes
  the job idempotent — safe to re-run failed shards.
- **`MaxRetries = 0` on the WritePolicy** because we retry in-process with our
  own exponential backoff and dedicated `retried` counter for observability.
- **Connection pool sized for concurrency** — `ConnectionQueueSize=1024`,
  `MinConnectionsPerNode=64` to avoid cold-start stalls.
- **Sorted+modulo file sharding** is deterministic across boxes without a
  coordinator — re-running shard `i` reprocesses the same files.

## What's intentionally not here

- No checkpointing across runs. Upserts make re-runs safe; if you need
  resume-from-failure semantics, add a per-file "done" marker (e.g. write a
  sentinel record or a sidecar file after successful close).
- No schema transforms. Every non-key column maps 1:1 to a bin of the same
  name. Add a mapper in `parquetValueToGo` / the bin-build loop if you need
  renames or type coercion.
- No TLS / auth on the Aerospike client. Add `ClientPolicy.TlsConfig`,
  `User`, `Password` if your cluster requires them.
