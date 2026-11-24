package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/anshikapundeel/telemetryscale/internal/gen"
	"github.com/anshikapundeel/telemetryscale/internal/metrics"
	"github.com/anshikapundeel/telemetryscale/internal/sink"
)

func cfg(p Policy) Config {
	return Config{QueueSize: 100, BatchSize: 10, FlushInterval: 10 * time.Millisecond, Workers: 2,
		MaxRetries: 5, BaseBackoff: time.Millisecond, MaxBackoff: 10 * time.Millisecond,
		SendTimeout: time.Second, Policy: p}
}

func TestDeliversEverythingWhenHealthy(t *testing.T) {
	fs := &sink.FakeSink{}
	p := New(cfg(Block), fs, metrics.NewRegistry())
	ctx := context.Background()
	p.Start(ctx)
	g := gen.New("t", 10, 1)
	for i := 0; i < 1000; i++ {
		p.Enqueue(ctx, g.Next())
	}
	p.Stop()
	if got := fs.Received.Load(); got != 1000 {
		t.Fatalf("want 1000 delivered, got %d", got)
	}
}

func TestRetriesAcrossOutage(t *testing.T) {
	fs := &sink.FakeSink{}
	fs.Down.Store(true)
	c := cfg(Block)
	c.MaxRetries = 1000
	p := New(c, fs, metrics.NewRegistry())
	ctx := context.Background()
	p.Start(ctx)
	g := gen.New("t", 10, 1)
	for i := 0; i < 50; i++ {
		p.Enqueue(ctx, g.Next())
	}
	time.Sleep(50 * time.Millisecond)
	fs.Down.Store(false) // recovery
	p.Stop()
	if got := fs.Received.Load(); got != 50 {
		t.Fatalf("data lost across outage: want 50, got %d", got)
	}
}

func TestDropNewestShedsLoadWhenFull(t *testing.T) {
	fs := &sink.FakeSink{Latency: 200 * time.Millisecond}
	c := cfg(DropNewest)
	c.QueueSize, c.Workers = 10, 1
	p := New(c, fs, metrics.NewRegistry())
	ctx := context.Background()
	p.Start(ctx)
	g := gen.New("t", 10, 1)
	dropped := 0
	for i := 0; i < 500; i++ {
		if !p.Enqueue(ctx, g.Next()) {
			dropped++
		}
	}
	p.Stop()
	if dropped == 0 {
		t.Fatal("expected drops under overload with DropNewest")
	}
}
