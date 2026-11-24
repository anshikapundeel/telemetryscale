// Package pipeline is the core of TelemetryScale: bounded queue -> batcher ->
// worker pool -> sink, with explicit backpressure policy and retry w/ backoff.
package pipeline

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/anshikapundeel/telemetryscale/internal/gen"
	"github.com/anshikapundeel/telemetryscale/internal/metrics"
	"github.com/anshikapundeel/telemetryscale/internal/sink"
)

type Policy int

const (
	Block      Policy = iota // producer waits (propagates backpressure upstream)
	DropNewest               // shed load at the edge, count the drops
)

type Config struct {
	QueueSize     int
	BatchSize     int
	FlushInterval time.Duration
	Workers       int
	MaxRetries    int
	BaseBackoff   time.Duration
	MaxBackoff    time.Duration
	SendTimeout   time.Duration
	Policy        Policy
}

type Pipeline struct {
	cfg                                 Config
	in                                  chan gen.Sample
	out                                 sink.Sink
	wg                                  sync.WaitGroup
	mOK, mFail, mDrop, mSamples, mRetry *metrics.Counter
	lat                                 *metrics.Histogram
}

func New(cfg Config, out sink.Sink, reg *metrics.Registry) *Pipeline {
	return &Pipeline{
		cfg: cfg, in: make(chan gen.Sample, cfg.QueueSize), out: out,
		mOK:      reg.Counter(`ts_batches_total{result="ok"}`),
		mFail:    reg.Counter(`ts_batches_total{result="failed"}`),
		mRetry:   reg.Counter(`ts_batch_retries_total`),
		mDrop:    reg.Counter(`ts_samples_dropped_total`),
		mSamples: reg.Counter(`ts_samples_sent_total`),
		lat:      reg.Histogram("ts_batch_seconds", []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5}),
	}
}

// Enqueue applies the backpressure policy. It returns false if the sample was dropped.
func (p *Pipeline) Enqueue(ctx context.Context, s gen.Sample) bool {
	if p.cfg.Policy == DropNewest {
		select {
		case p.in <- s:
			return true
		default:
			p.mDrop.Add(1)
			return false
		}
	}
	select {
	case p.in <- s:
		return true
	case <-ctx.Done():
		return false
	}
}

func (p *Pipeline) QueueDepth() int { return len(p.in) }

func (p *Pipeline) Start(ctx context.Context) {
	for i := 0; i < p.cfg.Workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx)
	}
}

// Stop closes the queue and waits for workers to drain it (graceful shutdown).
func (p *Pipeline) Stop() { close(p.in); p.wg.Wait() }

func (p *Pipeline) worker(ctx context.Context) {
	defer p.wg.Done()
	batch := make([]gen.Sample, 0, p.cfg.BatchSize)
	t := time.NewTicker(p.cfg.FlushInterval)
	defer t.Stop()
	flush := func() {
		if len(batch) == 0 {
			return
		}
		p.send(ctx, batch)
		batch = batch[:0]
	}
	for {
		select {
		case s, ok := <-p.in:
			if !ok {
				flush()
				return
			}
			batch = append(batch, s)
			if len(batch) >= p.cfg.BatchSize {
				flush()
			}
		case <-t.C:
			flush()
		}
	}
}

// send retries with capped exponential backoff and full jitter.
func (p *Pipeline) send(ctx context.Context, batch []gen.Sample) {
	start := time.Now()
	backoff := p.cfg.BaseBackoff
	for attempt := 0; ; attempt++ {
		sctx, cancel := context.WithTimeout(ctx, p.cfg.SendTimeout)
		err := p.out.Send(sctx, batch)
		cancel()
		if err == nil {
			p.mOK.Add(1)
			p.mSamples.Add(uint64(len(batch)))
			p.lat.Observe(time.Since(start).Seconds())
			return
		}
		if attempt >= p.cfg.MaxRetries || ctx.Err() != nil {
			p.mFail.Add(1)
			p.mDrop.Add(uint64(len(batch)))
			p.lat.Observe(time.Since(start).Seconds())
			return
		}
		p.mRetry.Add(1)
		sleep := time.Duration(rand.Int63n(int64(backoff) + 1))
		select {
		case <-time.After(sleep):
		case <-ctx.Done():
		}
		if backoff *= 2; backoff > p.cfg.MaxBackoff {
			backoff = p.cfg.MaxBackoff
		}
	}
}
