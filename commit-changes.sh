#!/usr/bin/env bash
# Run in your existing telemetryscale clone after copying these files over it.
set -euo pipefail
git add internal/gen internal/metrics internal/sink internal/trace internal/pipeline cmd/loadgen
git commit -m "feat: OTLP metrics and traces, log sinks (ClickHouse, Logstash), gauges, retry flags" || true
git add internal/slo cmd/sloctl deploy/slo internal/cardwatch cmd/cardwatch internal/stats cmd/logquery
git commit -m "feat: SLO burn-rate rule generator, cardinality watcher, ELK vs ClickHouse query bench" || true
git add internal/fleet cmd/fleetguard deploy/fleet docker-compose.fleet.yml lab
git commit -m "feat: FleetGuard checks (TLS, DNS, Kerberos, LDAP, DHCP, Puppet, HAProxy, SMTP) and lab" || true
git add internal/triage cmd/triage remediation
git commit -m "feat: LLM-assisted triage with injection fencing; Temporal remediation workflow" || true
git add -A
git commit -m "chore: compose (Prometheus agent, Grafana dashboard), bench scripts, docs, CI, Makefile" || true
git push
