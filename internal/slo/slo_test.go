package slo

import (
	"math"
	"strings"
	"testing"
)

func spec() *Spec {
	return &Spec{
		SLOs:    []SLO{{Name: "api", Objective: 99.9, Good: "g[{{window}}]", Total: "t[{{window}}]"}},
		Windows: []Window{{Long: "1h", Short: "5m", Factor: 14.4, Severity: "page"}},
	}
}

func TestBudget(t *testing.T) {
	if math.Abs(Budget(99.9)-0.001) > 1e-12 {
		t.Fatal("budget wrong")
	}
}

func TestRulesContainMultiWindowBurnRate(t *testing.T) {
	r := spec().Rules()
	for _, want := range []string{
		"record: slo:sli_error_ratio:rate1h", "record: slo:sli_error_ratio:rate5m",
		"expr: 1 - ((g[1h]) / (t[1h]))",
		`slo:sli_error_ratio:rate1h{slo="api"} > 0.0144 and slo:sli_error_ratio:rate5m{slo="api"} > 0.0144`,
		"severity: page",
	} {
		if !strings.Contains(r, want) {
			t.Fatalf("missing %q in:\n%s", want, r)
		}
	}
}

func TestValidateRejectsBadObjective(t *testing.T) {
	s := spec()
	s.SLOs[0].Objective = 100
	if err := s.validate(); err == nil {
		t.Fatal("expected error for objective 100")
	}
}
