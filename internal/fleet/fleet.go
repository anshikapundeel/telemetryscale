// Package fleet implements FleetGuard: concurrent health checks for the Linux
// infrastructure services an SRE team owns (TLS, DNS, Kerberos, LDAP, DHCP,
// Puppet, HAProxy, SMTP). Each check returns a structured Result.
package fleet

import (
	"context"
	"sync"
	"time"

	"github.com/anshikapundeel/telemetryscale/internal/metrics"
)

type Result struct {
	Check    string
	Target   string
	OK       bool
	Detail   string
	Duration time.Duration
}

type Check interface {
	Name() string
	Run(ctx context.Context, target string) Result
}

type Job struct {
	Check  Check
	Target string
}

// RunAll executes jobs on a bounded worker pool and returns results in input order.
func RunAll(ctx context.Context, jobs []Job, workers int, perCheckTimeout time.Duration) []Result {
	if workers < 1 {
		workers = 1
	}
	out := make([]Result, len(jobs))
	idx := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range idx {
				cctx, cancel := context.WithTimeout(ctx, perCheckTimeout)
				start := time.Now()
				r := jobs[i].Check.Run(cctx, jobs[i].Target)
				cancel()
				r.Check, r.Target, r.Duration = jobs[i].Check.Name(), jobs[i].Target, time.Since(start)
				out[i] = r
			}
		}()
	}
	for i := range jobs {
		idx <- i
	}
	close(idx)
	wg.Wait()
	return out
}

// Record publishes results as gauges so the SLO rules can consume them.
func Record(reg *metrics.Registry, rs []Result) {
	for _, r := range rs {
		up := 0.0
		if r.OK {
			up = 1
		}
		lbl := `{check="` + r.Check + `",target="` + r.Target + `"}`
		reg.Gauge("fleetguard_check_up" + lbl).Set(up)
		reg.Gauge("fleetguard_check_duration_seconds" + lbl).Set(r.Duration.Seconds())
	}
}

func ok(detail string) Result   { return Result{OK: true, Detail: detail} }
func fail(detail string) Result { return Result{OK: false, Detail: detail} }
