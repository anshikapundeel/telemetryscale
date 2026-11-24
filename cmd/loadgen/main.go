// loadgen drives the pipeline at a target rate and exposes /metrics.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/anshikapundeel/telemetryscale/internal/gen"
	"github.com/anshikapundeel/telemetryscale/internal/metrics"
	"github.com/anshikapundeel/telemetryscale/internal/pipeline"
	"github.com/anshikapundeel/telemetryscale/internal/sink"
)

func main() {
	rate := flag.Int("rate", 10000, "target samples/sec")
	dur := flag.Duration("duration", 30*time.Second, "run time")
	card := flag.Int("cardinality", 1000, "unique series")
	tenant := flag.String("tenant", "tenant-a", "tenant id")
	target := flag.String("sink", "fake", "fake | clickhouse")
	chURL := flag.String("ch-url", "http://localhost:8123", "clickhouse http url")
	policy := flag.String("policy", "block", "block | drop")
	workers := flag.Int("workers", 4, "sender workers")
	batch := flag.Int("batch", 1000, "batch size")
	qsize := flag.Int("queue", 50000, "queue capacity")
	listen := flag.String("listen", ":2112", "metrics listen addr")
	flag.Parse()

	reg := metrics.NewRegistry()
	go func() { log.Fatal(http.ListenAndServe(*listen, reg.Handler())) }()

	var out sink.Sink = &sink.FakeSink{Latency: 5 * time.Millisecond}
	if *target == "clickhouse" {
		out = &sink.ClickHouseSink{URL: *chURL, Table: "telemetry.metrics_raw", Client: &http.Client{Timeout: 10 * time.Second}}
	}
	pol := pipeline.Block
	if *policy == "drop" {
		pol = pipeline.DropNewest
	}
	p := pipeline.New(pipeline.Config{
		QueueSize: *qsize, BatchSize: *batch, FlushInterval: 200 * time.Millisecond, Workers: *workers,
		MaxRetries: 5, BaseBackoff: 100 * time.Millisecond, MaxBackoff: 5 * time.Second,
		SendTimeout: 5 * time.Second, Policy: pol,
	}, out, reg)

	ctx, cancel := context.WithTimeout(context.Background(), *dur)
	defer cancel()
	p.Start(context.Background())

	g := gen.New(*tenant, *card, time.Now().UnixNano())
	tick := time.NewTicker(10 * time.Millisecond) // pace in 10ms slices
	defer tick.Stop()
	perTick := *rate / 100
	var offered, accepted int
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-tick.C:
			for i := 0; i < perTick; i++ {
				if p.Enqueue(ctx, g.Next()) {
					accepted++
				}
				offered++
			}
		}
	}
	p.Stop()
	fmt.Printf("offered=%d accepted=%d delivered=%d dropped=%d\n", offered, accepted,
		reg.Counter("ts_samples_sent_total").Value(), reg.Counter("ts_samples_dropped_total").Value())
}
