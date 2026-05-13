// Command parquet-to-aerospike streams rows from a directory of parquet
// shards into an Aerospike cluster as upserts.
//
// Designed to be run as N parallel processes across M machines: each process
// is given a slice of files (--file-shard-index / --file-shard-total) and
// fans rows out to a pool of Aerospike write workers.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	aero "github.com/aerospike/aerospike-client-go/v7"
	"github.com/aerospike/aerospike-client-go/v7/types"
	"github.com/parquet-go/parquet-go"
)

type config struct {
	parquetDir    string
	filePattern   string
	asHosts       string
	asNamespace   string
	asSet         string
	keyColumn     string
	fileWorkers   int
	writeWorkers  int
	queueSize     int
	readBatch     int
	writeTimeout  time.Duration
	socketTimeout time.Duration
	maxRetries    int
	shardIndex    int
	shardTotal    int
	progressEvery time.Duration
}

func parseFlags() config {
	var c config
	flag.StringVar(&c.parquetDir, "parquet-dir", "", "directory containing parquet shards (required)")
	flag.StringVar(&c.filePattern, "file-pattern", "*.parquet", "glob pattern for shard files")
	flag.StringVar(&c.asHosts, "as-hosts", "127.0.0.1:3000", "comma-separated Aerospike host:port seed list")
	flag.StringVar(&c.asNamespace, "as-namespace", "", "Aerospike namespace (required)")
	flag.StringVar(&c.asSet, "as-set", "", "Aerospike set name")
	flag.StringVar(&c.keyColumn, "key-column", "id", "parquet column whose value becomes the Aerospike user key")
	flag.IntVar(&c.fileWorkers, "file-workers", runtime.NumCPU(), "number of parquet files read concurrently")
	flag.IntVar(&c.writeWorkers, "write-workers", 512, "number of Aerospike Put goroutines (tune to cluster capacity)")
	flag.IntVar(&c.queueSize, "queue-size", 50_000, "bounded channel size between readers and writers")
	flag.IntVar(&c.readBatch, "read-batch", 1024, "rows pulled per parquet ReadRows call")
	flag.DurationVar(&c.writeTimeout, "write-timeout", 200*time.Millisecond, "per-record total timeout")
	flag.DurationVar(&c.socketTimeout, "socket-timeout", 100*time.Millisecond, "per-record socket timeout")
	flag.IntVar(&c.maxRetries, "max-retries", 5, "max retry attempts on retryable Aerospike errors")
	flag.IntVar(&c.shardIndex, "file-shard-index", 0, "this process's shard index (0-based) for horizontal scaling")
	flag.IntVar(&c.shardTotal, "file-shard-total", 1, "total number of shards across the fleet")
	flag.DurationVar(&c.progressEvery, "progress-every", 5*time.Second, "progress log interval")
	flag.Parse()

	if c.parquetDir == "" || c.asNamespace == "" {
		flag.Usage()
		os.Exit(2)
	}
	if c.shardIndex < 0 || c.shardIndex >= c.shardTotal {
		log.Fatalf("invalid shard: index=%d total=%d", c.shardIndex, c.shardTotal)
	}
	return c
}

type stats struct {
	written  atomic.Uint64
	failed   atomic.Uint64
	retried  atomic.Uint64
	rowsRead atomic.Uint64
}

func main() {
	cfg := parseFlags()
	log.Printf("starting migration: shard %d/%d dir=%s ns=%s set=%s",
		cfg.shardIndex+1, cfg.shardTotal, cfg.parquetDir, cfg.asNamespace, cfg.asSet)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client, err := newAerospikeClient(cfg)
	if err != nil {
		log.Fatalf("aerospike connect: %v", err)
	}
	defer client.Close()

	files, err := discoverFiles(cfg)
	if err != nil {
		log.Fatalf("file discovery: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("no files matched %s/%s for shard %d/%d",
			cfg.parquetDir, cfg.filePattern, cfg.shardIndex+1, cfg.shardTotal)
	}
	log.Printf("shard owns %d parquet files", len(files))

	rowCh := make(chan rowJob, cfg.queueSize)
	var st stats

	// Writer pool: many goroutines doing single-key Puts in parallel. The
	// Aerospike Go client multiplexes these onto its per-node connection pool.
	var writerWG sync.WaitGroup
	writerWG.Add(cfg.writeWorkers)
	writePolicy := buildWritePolicy(cfg)
	for i := 0; i < cfg.writeWorkers; i++ {
		go func() {
			defer writerWG.Done()
			runWriter(ctx, client, writePolicy, rowCh, &st, cfg.maxRetries)
		}()
	}

	// Reader pool: one goroutine per file, bounded by fileWorkers.
	var readerWG sync.WaitGroup
	fileSem := make(chan struct{}, cfg.fileWorkers)
	for _, f := range files {
		readerWG.Add(1)
		go func(path string) {
			defer readerWG.Done()
			select {
			case fileSem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-fileSem }()
			if err := readParquetFile(ctx, path, cfg, rowCh, &st); err != nil {
				log.Printf("reader %s: %v", path, err)
			}
		}(f)
	}

	progDone := make(chan struct{})
	go reportProgress(ctx, &st, cfg.progressEvery, progDone)

	readerWG.Wait()
	close(rowCh)
	writerWG.Wait()
	close(progDone)

	log.Printf("done: rowsRead=%d written=%d failed=%d retried=%d",
		st.rowsRead.Load(), st.written.Load(), st.failed.Load(), st.retried.Load())
	if st.failed.Load() > 0 {
		os.Exit(1)
	}
}

func newAerospikeClient(cfg config) (*aero.Client, error) {
	hosts := make([]*aero.Host, 0)
	for _, hp := range strings.Split(cfg.asHosts, ",") {
		hp = strings.TrimSpace(hp)
		if hp == "" {
			continue
		}
		h, p, ok := strings.Cut(hp, ":")
		if !ok {
			return nil, fmt.Errorf("bad host:port %q", hp)
		}
		var port int
		if _, err := fmt.Sscanf(p, "%d", &port); err != nil {
			return nil, fmt.Errorf("bad port in %q: %w", hp, err)
		}
		hosts = append(hosts, aero.NewHost(h, port))
	}

	policy := aero.NewClientPolicy()
	policy.ConnectionQueueSize = 1024
	policy.MinConnectionsPerNode = 64
	policy.Timeout = 10 * time.Second
	policy.LimitConnectionsToQueueSize = true

	return aero.NewClientWithPolicyAndHost(policy, hosts...)
}

func buildWritePolicy(cfg config) *aero.WritePolicy {
	wp := aero.NewWritePolicy(0, 0) // generation=any, expiration=namespace default
	wp.RecordExistsAction = aero.UPDATE
	wp.SendKey = false
	wp.TotalTimeout = cfg.writeTimeout
	wp.SocketTimeout = cfg.socketTimeout
	wp.MaxRetries = 0 // we manage retries
	wp.CommitLevel = aero.COMMIT_MASTER
	return wp
}

func discoverFiles(cfg config) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(cfg.parquetDir, cfg.filePattern))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	out := make([]string, 0, len(matches)/cfg.shardTotal+1)
	for i, f := range matches {
		if i%cfg.shardTotal == cfg.shardIndex {
			out = append(out, f)
		}
	}
	return out, nil
}

type rowJob struct {
	key  *aero.Key
	bins aero.BinMap
}

func readParquetFile(ctx context.Context, path string, cfg config, out chan<- rowJob, st *stats) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	pf, err := parquet.OpenFile(f, stat.Size())
	if err != nil {
		return fmt.Errorf("open parquet: %w", err)
	}

	cols := pf.Schema().Columns()
	colNames := make([]string, len(cols))
	keyColIdx := -1
	for i, path := range cols {
		colNames[i] = strings.Join(path, ".")
		if colNames[i] == cfg.keyColumn {
			keyColIdx = i
		}
	}
	if keyColIdx < 0 {
		return fmt.Errorf("key column %q not found; available: %v", cfg.keyColumn, colNames)
	}

	for _, rg := range pf.RowGroups() {
		if err := readRowGroup(ctx, rg, colNames, keyColIdx, cfg, out, st); err != nil {
			return err
		}
	}
	return nil
}

func readRowGroup(ctx context.Context, rg parquet.RowGroup, colNames []string, keyColIdx int,
	cfg config, out chan<- rowJob, st *stats) error {

	rows := rg.Rows()
	defer rows.Close()

	buf := make([]parquet.Row, cfg.readBatch)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		n, readErr := rows.ReadRows(buf)
		for i := 0; i < n; i++ {
			row := buf[i]
			var keyVal any
			bins := make(aero.BinMap, len(colNames)-1)
			for _, v := range row {
				ci := v.Column()
				if ci < 0 || ci >= len(colNames) {
					continue
				}
				goVal := parquetValueToGo(v)
				if ci == keyColIdx {
					keyVal = goVal
					continue
				}
				bins[colNames[ci]] = goVal
			}
			if keyVal == nil {
				st.failed.Add(1)
				continue
			}
			ak, kerr := aero.NewKey(cfg.asNamespace, cfg.asSet, keyVal)
			if kerr != nil {
				st.failed.Add(1)
				continue
			}
			st.rowsRead.Add(1)
			select {
			case out <- rowJob{key: ak, bins: bins}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

// parquetValueToGo unboxes a parquet.Value into a Go scalar that Aerospike's
// BinMap accepts directly (int64, float64, string, []byte, bool).
func parquetValueToGo(v parquet.Value) any {
	if v.IsNull() {
		return nil
	}
	switch v.Kind() {
	case parquet.Boolean:
		return v.Boolean()
	case parquet.Int32:
		return int(v.Int32())
	case parquet.Int64:
		return v.Int64()
	case parquet.Float:
		return float64(v.Float())
	case parquet.Double:
		return v.Double()
	case parquet.ByteArray, parquet.FixedLenByteArray:
		// Most string-typed parquet columns surface as ByteArray; we copy
		// because the underlying buffer is reused by the reader.
		b := v.ByteArray()
		s := make([]byte, len(b))
		copy(s, b)
		return string(s)
	default:
		return v.String()
	}
}

func runWriter(ctx context.Context, client *aero.Client, wp *aero.WritePolicy,
	in <-chan rowJob, st *stats, maxRetries int) {
	for job := range in {
		if ctx.Err() != nil {
			return
		}
		if err := putWithRetry(client, wp, job, maxRetries, st); err != nil {
			st.failed.Add(1)
		} else {
			st.written.Add(1)
		}
	}
}

func putWithRetry(client *aero.Client, wp *aero.WritePolicy, job rowJob, maxRetries int, st *stats) error {
	backoff := 2 * time.Millisecond
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := client.Put(wp, job.key, job.bins)
		if err == nil {
			return nil
		}
		if !isRetryable(err) {
			return err
		}
		if attempt == maxRetries {
			return err
		}
		st.retried.Add(1)
		time.Sleep(backoff)
		backoff *= 2
		if backoff > 200*time.Millisecond {
			backoff = 200 * time.Millisecond
		}
	}
	return nil
}

func isRetryable(err error) bool {
	var aerr aero.Error
	if !errors.As(err, &aerr) {
		return true // network / unknown — retry
	}
	return aerr.Matches(
		types.TIMEOUT,
		types.NO_AVAILABLE_CONNECTIONS_TO_NODE,
		types.NETWORK_ERROR,
		types.SERVER_NOT_AVAILABLE,
		types.DEVICE_OVERLOAD,
		types.KEY_BUSY,
	)
}

func reportProgress(ctx context.Context, st *stats, every time.Duration, done <-chan struct{}) {
	t := time.NewTicker(every)
	defer t.Stop()
	start := time.Now()
	var lastWritten uint64
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case now := <-t.C:
			w := st.written.Load()
			delta := w - lastWritten
			lastWritten = w
			rate := float64(delta) / every.Seconds()
			elapsed := now.Sub(start).Truncate(time.Second)
			log.Printf("progress: elapsed=%s rowsRead=%d written=%d failed=%d retried=%d rate=%.0f/s",
				elapsed, st.rowsRead.Load(), w, st.failed.Load(), st.retried.Load(), rate)
		}
	}
}
