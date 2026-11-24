// sloctl generates Mimir/Prometheus rule files from an SLO spec.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/anshikapundeel/telemetryscale/internal/slo"
)

func main() {
	in := flag.String("f", "deploy/slo/slo.json", "SLO spec")
	out := flag.String("o", "-", "output file (- for stdout)")
	flag.Parse()
	s, err := slo.Load(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if *out == "-" {
		fmt.Print(s.Rules())
		return
	}
	if err := os.WriteFile(*out, []byte(s.Rules()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
