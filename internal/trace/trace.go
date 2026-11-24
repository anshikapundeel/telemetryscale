// Package trace is a minimal OTLP/HTTP (JSON) span exporter using only the stdlib.
// A nil *Tracer is valid and records nothing, so call sites need no guards.
package trace

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Tracer struct {
	url, service string
	client       *http.Client
	ch           chan rec
	done         chan struct{}
	wg           sync.WaitGroup
}

type rec struct {
	traceID, spanID, name string
	start, end            time.Time
	attrs                 map[string]string
	failed                bool
}

type Span struct {
	t *Tracer
	r rec
}

func New(url, service string) *Tracer {
	t := &Tracer{url: url, service: service, client: &http.Client{Timeout: 5 * time.Second},
		ch: make(chan rec, 8192), done: make(chan struct{})}
	t.wg.Add(1)
	go t.loop()
	return t
}

func id(n int) string { b := make([]byte, n); _, _ = rand.Read(b); return hex.EncodeToString(b) }

func (t *Tracer) Start(name string) *Span {
	if t == nil {
		return nil
	}
	return &Span{t: t, r: rec{traceID: id(16), spanID: id(8), name: name, start: time.Now(), attrs: map[string]string{}}}
}

func (s *Span) Set(k, v string) {
	if s != nil {
		s.r.attrs[k] = v
	}
}

func (s *Span) End(err error) {
	if s == nil {
		return
	}
	s.r.end, s.r.failed = time.Now(), err != nil
	select {
	case s.t.ch <- s.r:
	default: // never block the data path on tracing
	}
}

// Close flushes remaining spans.
func (t *Tracer) Close() {
	if t == nil {
		return
	}
	close(t.done)
	t.wg.Wait()
}

func (t *Tracer) loop() {
	defer t.wg.Done()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var buf []rec
	flush := func() {
		if len(buf) > 0 {
			t.export(buf)
			buf = buf[:0]
		}
	}
	for {
		select {
		case r := <-t.ch:
			buf = append(buf, r)
			if len(buf) >= 256 {
				flush()
			}
		case <-tick.C:
			flush()
		case <-t.done:
			for {
				select {
				case r := <-t.ch:
					buf = append(buf, r)
				default:
					flush()
					return
				}
			}
		}
	}
}

type kv struct {
	Key   string         `json:"key"`
	Value map[string]any `json:"value"`
}

func (t *Tracer) export(rs []rec) {
	spans := make([]map[string]any, 0, len(rs))
	for _, r := range rs {
		attrs := make([]kv, 0, len(r.attrs))
		for k, v := range r.attrs {
			attrs = append(attrs, kv{k, map[string]any{"stringValue": v}})
		}
		code := 1
		if r.failed {
			code = 2
		}
		spans = append(spans, map[string]any{
			"traceId": r.traceID, "spanId": r.spanID, "name": r.name, "kind": 1,
			"startTimeUnixNano": strconv.FormatInt(r.start.UnixNano(), 10),
			"endTimeUnixNano":   strconv.FormatInt(r.end.UnixNano(), 10),
			"attributes":        attrs, "status": map[string]any{"code": code},
		})
	}
	body, _ := json.Marshal(map[string]any{"resourceSpans": []any{map[string]any{
		"resource":   map[string]any{"attributes": []kv{{"service.name", map[string]any{"stringValue": t.service}}}},
		"scopeSpans": []any{map[string]any{"scope": map[string]any{"name": "telemetryscale"}, "spans": spans}},
	}}})
	resp, err := t.client.Post(t.url, "application/json", bytes.NewReader(body))
	if err == nil {
		resp.Body.Close()
	}
}
