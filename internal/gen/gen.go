// Package gen produces synthetic telemetry with a controllable series cardinality.
package gen

import (
	"fmt"
	"math/rand"
	"time"
)

type Sample struct {
	Tenant string
	Name   string
	Labels map[string]string
	Value  float64
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

// Next returns one sample. Series identity is (name, pod); raising Cardinality
// raises the number of unique pods, which is what blows up TSDB head memory.
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
