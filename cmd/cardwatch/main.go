// cardwatch reports labels with runaway cardinality for a Mimir tenant.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/anshikapundeel/telemetryscale/internal/cardwatch"
)

func main() {
	base := flag.String("mimir", "http://localhost:9009", "Mimir base URL")
	tenant := flag.String("tenant", "tenant-a", "tenant id")
	thr := flag.Int("threshold", 10000, "flag labels with more distinct values than this")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ls, err := cardwatch.Fetch(ctx, http.DefaultClient, *base, *tenant, 50)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	off := cardwatch.Offenders(ls, *thr)
	if len(off) == 0 {
		fmt.Printf("tenant=%s: no label above %d distinct values\n", *tenant, *thr)
		return
	}
	for _, l := range off {
		fmt.Printf("HIGH CARDINALITY tenant=%s label=%s distinct_values=%d\n", *tenant, l.Name, l.Values)
	}
	os.Exit(2)
}
