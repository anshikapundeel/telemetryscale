package sink

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anshikapundeel/telemetryscale/internal/gen"
)

func TestOTLPSinkSendsTenantHeaderAndGauge(t *testing.T) {
	var hdr, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr = r.Header.Get("X-Scope-OrgID")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer srv.Close()
	s := &OTLPMetricsSink{URL: srv.URL, Client: srv.Client()}
	g := gen.New("tenant-b", 5, 1)
	if err := s.Send(context.Background(), []gen.Sample{g.Next()}); err != nil {
		t.Fatal(err)
	}
	if hdr != "tenant-b" || !strings.Contains(body, `"gauge"`) || !strings.Contains(body, "ts_synthetic_requests_total") {
		t.Fatalf("bad request hdr=%q body=%s", hdr, body)
	}
}

func TestClickHouseLogsSinkBuildsInsert(t *testing.T) {
	var q, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query().Get("query")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer srv.Close()
	s := &ClickHouseLogsSink{URL: srv.URL, Client: srv.Client()}
	g := gen.New("t", 5, 1)
	if err := s.Send(context.Background(), []gen.Sample{g.NextLog()}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q, "telemetry.logs") || !strings.Contains(body, `"severity"`) {
		t.Fatalf("q=%q body=%s", q, body)
	}
}

func TestServerErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	defer srv.Close()
	s := &LogstashSink{URL: srv.URL, Client: &http.Client{Timeout: time.Second}}
	if err := s.Send(context.Background(), []gen.Sample{gen.New("t", 1, 1).NextLog()}); err == nil {
		t.Fatal("expected error on 500")
	}
}
