// Package metrics is a tiny stdlib-only Prometheus text exposition (no deps).
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
)

type Counter struct{ v atomic.Uint64 }

func (c *Counter) Add(n uint64)  { c.v.Add(n) }
func (c *Counter) Value() uint64 { return c.v.Load() }

// Histogram with fixed upper bounds (seconds), cumulative on export.
type Histogram struct {
	Bounds  []float64
	buckets []atomic.Uint64
	count   atomic.Uint64
	sumNs   atomic.Uint64
}

func NewHistogram(bounds []float64) *Histogram {
	return &Histogram{Bounds: bounds, buckets: make([]atomic.Uint64, len(bounds)+1)}
}

func (h *Histogram) Observe(sec float64) {
	i := sort.SearchFloat64s(h.Bounds, sec)
	h.buckets[i].Add(1)
	h.count.Add(1)
	h.sumNs.Add(uint64(sec * 1e9))
}

type Registry struct {
	mu       sync.Mutex
	counters map[string]*Counter
	hists    map[string]*Histogram
}

func NewRegistry() *Registry {
	return &Registry{counters: map[string]*Counter{}, hists: map[string]*Histogram{}}
}

// Counter name may include labels, e.g. `ts_batches_total{result="ok"}`.
func (r *Registry) Counter(name string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.counters[name]
	if !ok {
		c = &Counter{}
		r.counters[name] = c
	}
	return c
}

func (r *Registry) Histogram(name string, bounds []float64) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.hists[name]
	if !ok {
		h = NewHistogram(bounds)
		r.hists[name] = h
	}
	return h
}

func (r *Registry) Expose(w io.Writer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.counters))
	for n := range r.counters {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(w, "%s %d\n", n, r.counters[n].Value())
	}
	for n, h := range r.hists {
		var cum uint64
		for i, b := range h.Bounds {
			cum += h.buckets[i].Load()
			fmt.Fprintf(w, "%s_bucket{le=\"%g\"} %d\n", n, b, cum)
		}
		cum += h.buckets[len(h.Bounds)].Load()
		fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n", n, cum)
		fmt.Fprintf(w, "%s_count %d\n%s_sum %g\n", n, h.count.Load(), n, float64(h.sumNs.Load())/1e9)
	}
}

func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		r.Expose(w)
	})
}
