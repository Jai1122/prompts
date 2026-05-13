// Command parquet-to-aerospike streams rows from a directory of parquet
// shards into an Aerospike cluster as upserts.
//
// Designed to run as N parallel processes across M machines: each process
// owns a deterministic slice of files via --file-shard-index/--file-shard-total.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	aero "github.com/aerospike/aerospike-client-go/v7"
	"github.com/parquet-go/parquet-go"
)

type config struct {
	parquetDir   string
	filePattern  string
	asHosts      string
	asNamespace  string
	asSet        string
	keyColumn    string
	fileWorkers  int
	writeWorkers int
	queueSize    int
	readBatch    int
	shardIndex   int
	shardTotal   int
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
	flag.IntVar(&c.writeWorkers, "write-workers", 1024, "number of Aerospike Put goroutines")
	flag.IntVar(&c.queueSize, "queue-size", 100_000, "bounded channel size between readers and writers")
	flag.IntVar(&c.readBatch, "read-batch", 1024, "rows pulled per parquet ReadRows call")
	flag.IntVar(&c.shardIndex, "file-shard-index", 0, "this process's shard index (0-based)")
	flag.IntVar(&c.shardTotal, "file-shard-total", 1, "total shards across the fleet")
	flag.Parse()

	if c.parquetDir == "" || c.asNamespace == "" {
		flag.Usage()
		os.Exit(2)
	}
	return c
}

type rowJob struct {
	key  *aero.Key
	bins aero.BinMap
}

var (
	written atomic.Uint64
	failed  atomic.Uint64
)

func main() {
	cfg := parseFlags()

	client, err := newClient(cfg)
	if err != nil {
		log.Fatalf("aerospike connect: %v", err)
	}
	defer client.Close()

	files := discoverFiles(cfg)
	if len(files) == 0 {
		log.Fatalf("no files matched %s/%s", cfg.parquetDir, cfg.filePattern)
	}
	log.Printf("shard %d/%d owns %d files", cfg.shardIndex+1, cfg.shardTotal, len(files))

	wp := aero.NewWritePolicy(0, 0)
	wp.RecordExistsAction = aero.UPDATE
	wp.SendKey = false

	rowCh := make(chan rowJob, cfg.queueSize)

	var writerWG sync.WaitGroup
	writerWG.Add(cfg.writeWorkers)
	for i := 0; i < cfg.writeWorkers; i++ {
		go func() {
			defer writerWG.Done()
			for job := range rowCh {
				if err := client.Put(wp, job.key, job.bins); err != nil {
					failed.Add(1)
				} else {
					written.Add(1)
				}
			}
		}()
	}

	var readerWG sync.WaitGroup
	sem := make(chan struct{}, cfg.fileWorkers)
	for _, f := range files {
		readerWG.Add(1)
		go func(path string) {
			defer readerWG.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := readFile(path, cfg, rowCh); err != nil {
				log.Printf("reader %s: %v", path, err)
			}
		}(f)
	}

	done := make(chan struct{})
	go progress(done)

	readerWG.Wait()
	close(rowCh)
	writerWG.Wait()
	close(done)

	log.Printf("done: written=%d failed=%d", written.Load(), failed.Load())
	if failed.Load() > 0 {
		os.Exit(1)
	}
}

func newClient(cfg config) (*aero.Client, error) {
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
			return nil, err
		}
		hosts = append(hosts, aero.NewHost(h, port))
	}
	pol := aero.NewClientPolicy()
	pol.ConnectionQueueSize = 1024
	pol.MinConnectionsPerNode = 64
	pol.Timeout = 10 * time.Second
	return aero.NewClientWithPolicyAndHost(pol, hosts...)
}

func discoverFiles(cfg config) []string {
	matches, _ := filepath.Glob(filepath.Join(cfg.parquetDir, cfg.filePattern))
	sort.Strings(matches)
	out := make([]string, 0, len(matches)/cfg.shardTotal+1)
	for i, f := range matches {
		if i%cfg.shardTotal == cfg.shardIndex {
			out = append(out, f)
		}
	}
	return out
}

func readFile(path string, cfg config, out chan<- rowJob) error {
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
		return err
	}

	cols := pf.Schema().Columns()
	colNames := make([]string, len(cols))
	keyIdx := -1
	for i, p := range cols {
		colNames[i] = strings.Join(p, ".")
		if colNames[i] == cfg.keyColumn {
			keyIdx = i
		}
	}
	if keyIdx < 0 {
		return fmt.Errorf("key column %q not found in %v", cfg.keyColumn, colNames)
	}

	buf := make([]parquet.Row, cfg.readBatch)
	for _, rg := range pf.RowGroups() {
		rows := rg.Rows()
		for {
			n, readErr := rows.ReadRows(buf)
			for i := 0; i < n; i++ {
				var keyVal any
				bins := make(aero.BinMap, len(colNames)-1)
				for _, v := range buf[i] {
					ci := v.Column()
					if ci < 0 || ci >= len(colNames) {
						continue
					}
					gv := toGo(v)
					if ci == keyIdx {
						keyVal = gv
						continue
					}
					bins[colNames[ci]] = gv
				}
				if keyVal == nil {
					failed.Add(1)
					continue
				}
				ak, kerr := aero.NewKey(cfg.asNamespace, cfg.asSet, keyVal)
				if kerr != nil {
					failed.Add(1)
					continue
				}
				out <- rowJob{key: ak, bins: bins}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				rows.Close()
				return readErr
			}
		}
		rows.Close()
	}
	return nil
}

func toGo(v parquet.Value) any {
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
		b := v.ByteArray()
		s := make([]byte, len(b))
		copy(s, b)
		return string(s)
	default:
		return v.String()
	}
}

func progress(done <-chan struct{}) {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	start := time.Now()
	var last uint64
	for {
		select {
		case <-done:
			return
		case <-t.C:
			w := written.Load()
			rate := float64(w-last) / 5.0
			last = w
			log.Printf("elapsed=%s written=%d failed=%d rate=%.0f/s",
				time.Since(start).Truncate(time.Second), w, failed.Load(), rate)
		}
	}
}
