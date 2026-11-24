# TelemetryScale

One platform for **building, measuring, and operating observability infrastructure**:
a Go ingest pipeline and benchmark harness, a Mimir/ClickHouse/ELK/OpenTelemetry stack with
generated SLO burn-rate alerts, FleetGuard Linux-service checks, LLM-assisted incident triage,
and a Temporal remediation workflow.

**Question it answers:** when load, series cardinality, or a backend outage changes, where does the
observability platform itself break, how does it fail, and how do you detect and recover?

See `docs/ARCHITECTURE.md` for the design and `docs/COVERAGE.md` for exactly what is tested vs. configured.

## Components
| Path | What |
|---|---|
| `internal/pipeline` | bounded queue, batching, worker pool, backpressure (Block / DropNewest), retry with jittered backoff, optional tracing |
| `internal/sink` | ClickHouse (metrics + logs), Logstash, OTLP/HTTP metrics (tenant header), fault-injecting fake |
| `internal/trace` | minimal OTLP/HTTP JSON span exporter |
| `internal/slo`, `cmd/sloctl` | SLO spec to multi-window, multi-burn-rate Mimir rules |
| `internal/cardwatch`, `cmd/cardwatch` | high-cardinality label detection via Mimir's cardinality API |
| `cmd/logquery` | ELK vs ClickHouse query latency (p50/p95/p99) and storage comparison |
| `internal/fleet`, `cmd/fleetguard` | TLS, DNS, Kerberos, LDAP, DHCP, Puppet, HAProxy, SMTP checks run concurrently, exported as gauges |
| `internal/triage`, `cmd/triage` | alert context to triage note (LLM with untrusted-input fencing; offline fallback) |
| `remediation/` | Temporal workflow: approval signal, retries, rollback, duplicate suppression (separate Go module) |
| `bench/` | ramp, outage, noisy-tenant, cardinality, and log-comparison experiments |
| `docs/` | architecture, runbook, postmortem template, coverage |

## Quick start
    make test                 # core tests + remediation module
    make up                   # Mimir, ClickHouse, OTel Collector, Prometheus agent, Grafana
    make rules                # generate and load SLO burn-rate rules into Mimir
    make run                  # loadgen via OTLP with tracing
    make up-fleet && make fixtures && make fleet-once
    open http://localhost:3000   # dashboard: TelemetryScale

Temporal (optional): `brew install temporal`, then `temporal server start-dev`, and in `remediation/`
run `go run ./cmd/worker` and `go run ./cmd/start -service loadgen`.

## Experiments (results go in `bench/results/`)
    ./bench/ramp.sh            # throughput ramp
    ./bench/outage.sh          # kill ClickHouse mid-run, measure loss and recovery
    ./bench/noisy-tenant.sh    # tenant-b floods cardinality; is tenant-a affected?
    ./bench/cardinality.sh     # series count vs Mimir memory
    ./bench/logs.sh            # ELK vs ClickHouse: ingest, query latency, bytes per event

## Results
Not yet recorded. Each experiment will be added with the command, config, hardware, raw output, and an interpretation.
