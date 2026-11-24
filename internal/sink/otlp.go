package sink

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"github.com/anshikapundeel/telemetryscale/internal/gen"
)

// OTLPMetricsSink sends gauge data points as OTLP/HTTP JSON. The tenant is carried
// in X-Scope-OrgID; the Collector's headers_setter forwards it to Mimir.
type OTLPMetricsSink struct {
	URL    string // e.g. http://localhost:4318/v1/metrics
	Client *http.Client
}

type attr struct {
	Key   string            `json:"key"`
	Value map[string]string `json:"value"`
}

func (s *OTLPMetricsSink) Send(ctx context.Context, batch []gen.Sample) error {
	if len(batch) == 0 {
		return nil
	}
	byName := map[string][]map[string]any{}
	for _, b := range batch {
		keys := make([]string, 0, len(b.Labels))
		for k := range b.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		attrs := make([]attr, 0, len(keys))
		for _, k := range keys {
			attrs = append(attrs, attr{k, map[string]string{"stringValue": b.Labels[k]}})
		}
		byName[b.Name] = append(byName[b.Name], map[string]any{
			"attributes": attrs, "timeUnixNano": strconv.FormatInt(b.TS.UnixNano(), 10), "asDouble": b.Value,
		})
	}
	metrics := make([]any, 0, len(byName))
	for name, dps := range byName {
		metrics = append(metrics, map[string]any{"name": name, "gauge": map[string]any{"dataPoints": dps}})
	}
	body, _ := json.Marshal(map[string]any{"resourceMetrics": []any{map[string]any{
		"resource":     map[string]any{"attributes": []attr{{"service.name", map[string]string{"stringValue": "telemetryscale-loadgen"}}}},
		"scopeMetrics": []any{map[string]any{"metrics": metrics}},
	}}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Scope-OrgID", batch[0].Tenant)
	resp, err := s.Client.Do(req)
	if err != nil {
		return ErrUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return &StatusError{Code: resp.StatusCode}
	}
	return nil
}

type StatusError struct{ Code int }

func (e *StatusError) Error() string { return "http status " + strconv.Itoa(e.Code) }
