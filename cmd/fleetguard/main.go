// fleetguard runs Linux infrastructure health checks across a target list,
// exposes results as Prometheus gauges, and exits non-zero in -once mode on failure.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/anshikapundeel/telemetryscale/internal/fleet"
	"github.com/anshikapundeel/telemetryscale/internal/metrics"
)

type target struct {
	Check  string `json:"check"`
	Target string `json:"target"`
	// optional per-check parameters
	Server       string  `json:"server,omitempty"`         // dns: resolver host:port
	MinDays      int     `json:"min_days,omitempty"`       // tls
	Insecure     bool    `json:"insecure,omitempty"`       // tls (lab certs)
	MaxAgeMin    int     `json:"max_age_min,omitempty"`    // puppet
	PoolSize     int     `json:"pool_size,omitempty"`      // dhcp
	MaxUtil      float64 `json:"max_util,omitempty"`       // dhcp
	BindDN       string  `json:"bind_dn,omitempty"`        // ldap
	Password     string  `json:"password,omitempty"`       // ldap
	MinTicketMin int     `json:"min_ticket_min,omitempty"` // kerberos
}

type config struct {
	Interval string   `json:"interval"`
	Workers  int      `json:"workers"`
	Timeout  string   `json:"timeout"`
	Targets  []target `json:"targets"`
}

func build(t target) (fleet.Job, error) {
	var c fleet.Check
	switch t.Check {
	case "tls":
		c = fleet.TLSCheck{MinDays: t.MinDays, Insecure: t.Insecure}
	case "dns":
		c = fleet.DNSCheck{Server: t.Server, MaxLatency: 500 * time.Millisecond}
	case "smtp":
		c = fleet.SMTPCheck{}
	case "haproxy":
		c = fleet.HAProxyCheck{}
	case "puppet":
		c = fleet.PuppetCheck{MaxAge: time.Duration(t.MaxAgeMin) * time.Minute}
	case "dhcp":
		c = fleet.DHCPCheck{PoolSize: t.PoolSize, MaxUtil: t.MaxUtil}
	case "kerberos":
		c = fleet.KerberosCheck{MinTicketLife: time.Duration(t.MinTicketMin) * time.Minute}
	case "ldap":
		c = fleet.LDAPCheck{BindDN: t.BindDN, Password: t.Password}
	default:
		return fleet.Job{}, fmt.Errorf("unknown check %q", t.Check)
	}
	return fleet.Job{Check: c, Target: t.Target}, nil
}

func main() {
	cfgPath := flag.String("config", "deploy/fleet/fleetguard.json", "config file")
	once := flag.Bool("once", false, "run all checks once, print, exit 1 if any fail")
	listen := flag.String("listen", ":2113", "metrics listen addr")
	flag.Parse()

	raw, err := os.ReadFile(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	var cfg config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		log.Fatal(err)
	}
	var jobs []fleet.Job
	for _, t := range cfg.Targets {
		j, err := build(t)
		if err != nil {
			log.Fatal(err)
		}
		jobs = append(jobs, j)
	}
	interval, _ := time.ParseDuration(cfg.Interval)
	if interval <= 0 {
		interval = 30 * time.Second
	}
	timeout, _ := time.ParseDuration(cfg.Timeout)
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 8
	}

	reg := metrics.NewRegistry()
	run := func() bool {
		rs := fleet.RunAll(context.Background(), jobs, cfg.Workers, timeout)
		fleet.Record(reg, rs)
		allOK := true
		for _, r := range rs {
			status := "OK  "
			if !r.OK {
				status, allOK = "FAIL", false
			}
			fmt.Printf("%s %-9s %-34s %s (%v)\n", status, r.Check, r.Target, r.Detail, r.Duration.Round(time.Millisecond))
		}
		return allOK
	}
	if *once {
		if !run() {
			os.Exit(1)
		}
		return
	}
	go func() { log.Fatal(http.ListenAndServe(*listen, reg.Handler())) }()
	for {
		run()
		time.Sleep(interval)
	}
}
