// Package sink defines where batches go. ClickHouseSink speaks the HTTP
// interface (JSONEachRow); FakeSink injects latency/failures for tests.
package sink

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/anshikapundeel/telemetryscale/internal/gen"
)

type Sink interface {
	Send(ctx context.Context, batch []gen.Sample) error
}

var ErrUnavailable = errors.New("sink unavailable")

type ClickHouseSink struct {
	URL    string // e.g. http://localhost:8123
	Table  string // e.g. telemetry.metrics_raw
	Client *http.Client
}

func (s *ClickHouseSink) Send(ctx context.Context, batch []gen.Sample) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, b := range batch {
		_ = enc.Encode(map[string]any{
			"ts": chTime(b.TS), "tenant": b.Tenant, "name": b.Name,
			"labels": b.Labels, "value": b.Value,
		})
	}
	q := url.QueryEscape(fmt.Sprintf("INSERT INTO %s FORMAT JSONEachRow", s.Table))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL+"/?query="+q, &buf)
	if err != nil {
		return err
	}
	resp, err := s.Client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("clickhouse status %d", resp.StatusCode)
	}
	return nil
}

// FakeSink: configurable latency and failure; toggle Down to simulate an outage.
type FakeSink struct {
	Latency  time.Duration
	FailRate float64
	Down     atomic.Bool
	Received atomic.Uint64
}

func (f *FakeSink) Send(ctx context.Context, batch []gen.Sample) error {
	if f.Latency > 0 {
		select {
		case <-time.After(f.Latency):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if f.Down.Load() || rand.Float64() < f.FailRate {
		return ErrUnavailable
	}
	f.Received.Add(uint64(len(batch)))
	return nil
}

// chTime formats a timestamp for ClickHouse DateTime64(3) JSON input (UTC).
func chTime(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05.000") }
