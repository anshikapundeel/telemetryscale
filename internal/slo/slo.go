// Package slo turns SLO definitions into Prometheus/Mimir recording and multi-window,
// multi-burn-rate alerting rules (Google SRE Workbook, ch. 5).
package slo

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type SLO struct {
	Name      string  `json:"name"`
	Objective float64 `json:"objective"` // percent, e.g. 99.9
	Good      string  `json:"good"`      // PromQL with {{window}}
	Total     string  `json:"total"`     // PromQL with {{window}}
}

type Window struct {
	Long     string  `json:"long"`
	Short    string  `json:"short"`
	Factor   float64 `json:"factor"`
	Severity string  `json:"severity"`
}

type Spec struct {
	SLOs    []SLO    `json:"slos"`
	Windows []Window `json:"windows"`
}

func Load(path string) (*Spec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Spec
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, s.validate()
}

func (s *Spec) validate() error {
	for _, o := range s.SLOs {
		if o.Objective <= 0 || o.Objective >= 100 {
			return fmt.Errorf("slo %q: objective must be in (0,100), got %v", o.Name, o.Objective)
		}
		if !strings.Contains(o.Good, "{{window}}") && !strings.Contains(o.Total, "{{window}}") {
			return fmt.Errorf("slo %q: good/total must use {{window}}", o.Name)
		}
	}
	if len(s.Windows) == 0 {
		return fmt.Errorf("no burn-rate windows defined")
	}
	return nil
}

// Budget is the allowed error ratio: objective 99.9 -> 0.001.
func Budget(objective float64) float64 { return (100 - objective) / 100 }

// Rules renders a Prometheus rule file.
func (s *Spec) Rules() string {
	var b strings.Builder
	b.WriteString("groups:\n")
	for _, o := range s.SLOs {
		wins := uniqueWindows(s.Windows)
		fmt.Fprintf(&b, "  - name: slo-%s\n    rules:\n", o.Name)
		for _, w := range wins {
			good := strings.ReplaceAll(o.Good, "{{window}}", w)
			total := strings.ReplaceAll(o.Total, "{{window}}", w)
			fmt.Fprintf(&b, "      - record: slo:sli_error_ratio:rate%s\n        expr: 1 - ((%s) / (%s))\n        labels:\n          slo: %s\n", w, good, total, o.Name)
		}
		budget := Budget(o.Objective)
		for _, w := range s.Windows {
			thr := fmt.Sprintf("%.6g", w.Factor*budget)
			fmt.Fprintf(&b, "      - alert: SLOBurnRate\n        expr: slo:sli_error_ratio:rate%s{slo=\"%s\"} > %s and slo:sli_error_ratio:rate%s{slo=\"%s\"} > %s\n",
				w.Long, o.Name, thr, w.Short, o.Name, thr)
			fmt.Fprintf(&b, "        labels:\n          severity: %s\n          slo: %s\n          burn_window: %s\n", w.Severity, o.Name, w.Long)
			fmt.Fprintf(&b, "        annotations:\n          summary: \"%s is burning error budget %gx faster than allowed over %s\"\n", o.Name, w.Factor, w.Long)
		}
	}
	return b.String()
}

func uniqueWindows(ws []Window) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range ws {
		for _, x := range []string{w.Long, w.Short} {
			if !seen[x] {
				seen[x] = true
				out = append(out, x)
			}
		}
	}
	return out
}
