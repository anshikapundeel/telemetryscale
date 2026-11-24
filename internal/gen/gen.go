// Package gen produces synthetic telemetry with a controllable series cardinality.
package gen

import (
	"fmt"
	"math/rand"
	"time"
)

// Sample is a metric point (Body == "") or a log line (Body != "").
type Sample struct {
	Tenant string
	Name   string
	Labels map[string]string
	Value  float64
	Body   string
	TS     time.Time
}

type Generator struct {
	Tenant      string
	Cardinality int // number of unique label combinations
	r           *rand.Rand
}

func New(tenant string, cardinality int, seed int64) *Generator {
	if cardinality < 1 {
		cardinality = 1
	}
	return &Generator{Tenant: tenant, Cardinality: cardinality, r: rand.New(rand.NewSource(seed))}
}

// Next returns one metric sample. Series identity is (name, pod); raising
// Cardinality raises unique pods, which is what blows up TSDB head memory.
func (g *Generator) Next() Sample {
	id := g.r.Intn(g.Cardinality)
	return Sample{
		Tenant: g.Tenant,
		Name:   "ts_synthetic_requests_total",
		Labels: map[string]string{"pod": fmt.Sprintf("pod-%d", id), "svc": fmt.Sprintf("svc-%d", id%50)},
		Value:  g.r.Float64() * 100,
		TS:     time.Now(),
	}
}

var (
	severities = []string{"INFO", "INFO", "INFO", "INFO", "WARN", "ERROR", "DEBUG"}
	bodies     = []string{
		"request completed in %dms",
		"connection reset by peer after %d retries",
		"cache miss for key user:%d",
		"kerberos ticket renewal failed attempt=%d",
		"disk usage at %d percent on /var",
		"ldap bind timeout after %dms",
	}
)

// NextLog returns one synthetic log line with a trace id for correlation.
func (g *Generator) NextLog() Sample {
	id := g.r.Intn(g.Cardinality)
	return Sample{
		Tenant: g.Tenant,
		Name:   "log",
		Labels: map[string]string{
			"service":  fmt.Sprintf("svc-%d", id%50),
			"severity": severities[g.r.Intn(len(severities))],
			"host":     fmt.Sprintf("host-%d", id%200),
			"trace_id": fmt.Sprintf("%016x%016x", g.r.Uint64(), g.r.Uint64()),
		},
		Body: fmt.Sprintf(bodies[g.r.Intn(len(bodies))], g.r.Intn(5000)),
		TS:   time.Now(),
	}
}
