package trace

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExportsSpanAsOTLPJSON(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got <- string(b)
	}))
	defer srv.Close()
	tr := New(srv.URL, "test-svc")
	s := tr.Start("pipeline.send")
	s.Set("batch.size", "10")
	s.End(errors.New("boom"))
	tr.Close()
	body := <-got
	for _, want := range []string{`"resourceSpans"`, `"test-svc"`, `"pipeline.send"`, `"batch.size"`, `"code":2`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
}

func TestNilTracerIsSafe(t *testing.T) {
	var tr *Tracer
	s := tr.Start("x")
	s.Set("a", "b")
	s.End(nil)
	tr.Close()
}
