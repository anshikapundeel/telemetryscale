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
	"github.com/anshikapundeel/telemetryscale/internal/trace"
)

func main() {
	rate := flag.Int("rate", 10000, "target samples/sec")
	dur := flag.Duration("duration", 30*time.Second, "run time")
	card := flag.Int("cardinality", 1000, "unique series / label combinations")
	tenant := flag.String("tenant", "tenant-a", "tenant id")
	mode := flag.String("mode", "metrics", "metrics | logs")
	target := flag.String("sink", "fake", "fake | clickhouse | otlp | logstash")
	chURL := flag.String("ch-url", "http://localhost:8123", "ClickHouse HTTP URL")
	otlpURL := flag.String("otlp-url", "http://localhost:4318/v1/metrics", "OTLP/HTTP metrics URL")
	lsURL := flag.String("logstash-url", "http://localhost:8080", "Logstash http input URL")
	traceURL := flag.String("trace-url", "", "OTLP/HTTP traces URL; empty disables tracing")
	policy := flag.String("policy", "block", "block | drop")
	workers := flag.Int("workers", 4, "sender workers")
	batch := flag.Int("batch", 1000, "batch size")
	qsize := flag.Int("queue", 50000, "queue capacity")
	listen := flag.String("listen", ":2112", "metrics listen addr")
	maxRetries := flag.Int("max-retries", 5, "send retries before a batch is dropped")
	maxBackoff := flag.Duration("max-backoff", 5*time.Second, "retry backoff cap")
	flag.Parse()

	reg := metrics.NewRegistry()
	go func() { log.Fatal(http.ListenAndServe(*listen, reg.Handler())) }()

	hc := &http.Client{Timeout: 10 * time.Second}
	var out sink.Sink = &sink.FakeSink{Latency: 5 * time.Millisecond}
	switch *target {
	case "clickhouse":
		if *mode == "logs" {
			out = &sink.ClickHouseLogsSink{URL: *chURL, Client: hc}
		} else {
			out = &sink.ClickHouseSink{URL: *chURL, Table: "telemetry.metrics_raw", Client: hc}
		}
	case "otlp":
		out = &sink.OTLPMetricsSink{URL: *otlpURL, Client: hc}
	case "logstash":
		out = &sink.LogstashSink{URL: *lsURL, Client: hc}
	}
	pol := pipeline.Block
	if *policy == "drop" {
		pol = pipeline.DropNewest
	}
	var tr *trace.Tracer
	if *traceURL != "" {
		tr = trace.New(*traceURL, "telemetryscale-loadgen")
	}
	p := pipeline.New(pipeline.Config{
		QueueSize: *qsize, BatchSize: *batch, FlushInterval: 200 * time.Millisecond, Workers: *workers,
		MaxRetries: *maxRetries, BaseBackoff: 100 * time.Millisecond, MaxBackoff: *maxBackoff,
		SendTimeout: 5 * time.Second, Policy: pol, Tracer: tr,
	}, out, reg)

	depth := reg.Gauge("ts_queue_depth")
	go func() {
		for range time.Tick(500 * time.Millisecond) {
			depth.Set(float64(p.QueueDepth()))
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), *dur)
	defer cancel()
	p.Start(context.Background())

	g := gen.New(*tenant, *card, time.Now().UnixNano())
	next := g.Next
	if *mode == "logs" {
		next = g.NextLog
	}
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
				if p.Enqueue(ctx, next()) {
					accepted++
				}
				offered++
			}
		}
	}
	p.Stop()
	tr.Close()
	fmt.Printf("tenant=%s mode=%s sink=%s offered=%d accepted=%d delivered=%d dropped=%d retries=%d\n",
		*tenant, *mode, *target, offered, accepted,
		reg.Counter("ts_samples_sent_total").Value(), reg.Counter("ts_samples_dropped_total").Value(),
		reg.Counter("ts_batch_retries_total").Value())
}
