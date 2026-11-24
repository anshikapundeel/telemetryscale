package sink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/anshikapundeel/telemetryscale/internal/gen"
)

// ClickHouseLogsSink inserts log samples into telemetry.logs via HTTP JSONEachRow.
type ClickHouseLogsSink struct {
	URL    string
	Table  string // default telemetry.logs
	Client *http.Client
}

func (s *ClickHouseLogsSink) Send(ctx context.Context, batch []gen.Sample) error {
	table := s.Table
	if table == "" {
		table = "telemetry.logs"
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, b := range batch {
		attrs := map[string]string{}
		for k, v := range b.Labels {
			if k != "service" && k != "severity" && k != "host" && k != "trace_id" {
				attrs[k] = v
			}
		}
		_ = enc.Encode(map[string]any{
			"ts": chTime(b.TS), "tenant": b.Tenant, "service": b.Labels["service"],
			"severity": b.Labels["severity"], "host": b.Labels["host"],
			"trace_id": b.Labels["trace_id"], "body": b.Body, "attrs": attrs,
		})
	}
	q := url.QueryEscape(fmt.Sprintf("INSERT INTO %s FORMAT JSONEachRow", table))
	return post(ctx, s.Client, s.URL+"/?query="+q, "application/x-ndjson", &buf)
}

// LogstashSink posts newline-delimited JSON to a Logstash http input (json_lines codec).
type LogstashSink struct {
	URL    string // e.g. http://localhost:8080
	Client *http.Client
}

func (s *LogstashSink) Send(ctx context.Context, batch []gen.Sample) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, b := range batch {
		_ = enc.Encode(map[string]any{
			"@timestamp": b.TS.UTC().Format(time.RFC3339Nano), "tenant": b.Tenant,
			"service": b.Labels["service"], "severity": b.Labels["severity"],
			"host": b.Labels["host"], "trace_id": b.Labels["trace_id"], "message": b.Body,
		})
	}
	return post(ctx, s.Client, s.URL, "application/x-ndjson", &buf)
}

func post(ctx context.Context, c *http.Client, u, ct string, body *bytes.Buffer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ct)
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d from %s", resp.StatusCode, u)
	}
	return nil
}
