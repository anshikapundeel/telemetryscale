# Coverage vs. the target role

Verification levels: **tested** = unit tests pass in CI; **config** = written but not yet run against live services;
**unbuilt-here** = code written, not compiled in the authoring sandbox (CI is the first check).

| Requirement | Where | Status |
|---|---|---|
| Production-grade concurrent Go | `internal/pipeline`, `internal/fleet` (worker pools, bounded queues, race-tested) | tested |
| Metrics: Grafana Mimir (multi-tenant) | `deploy/mimir`, `internal/sink/otlp.go`, `bench/noisy-tenant.sh` | sink tested; Mimir wiring config |
| High-throughput telemetry store: ClickHouse | `deploy/clickhouse/init.sql`, `internal/sink` ClickHouse sinks, `bench/ramp.sh` | sinks tested; schema config |
| Logging: ELK | `docker-compose.yml` (elk profile), Logstash sink, `cmd/logquery`, `bench/logs.sh` | sink tested; stack config |
| Tracing: OpenTelemetry | `internal/trace` (OTLP JSON spans), Collector config | exporter tested; Collector config |
| SLIs / SLOs / alerting | `deploy/slo/slo.json`, `internal/slo`, `cmd/sloctl`, `bench/load-rules.sh` | generator tested; ruler load config |
| Capacity planning | `bench/ramp.sh`, `bench/cardinality.sh` | scripts; results pending your runs |
| High-cardinality detection | `internal/cardwatch`, `cmd/cardwatch` | tested against fake API; verify response shape on real Mimir |
| Linux internals / services: TLS, DNS, Kerberos, LDAP, DHCP, Puppet, HAProxy, SMTP | `internal/fleet`, `cmd/fleetguard`, `docker-compose.fleet.yml` | checks tested with in-process fakes; lab is config |
| Incident response, blameless postmortems | `docs/RUNBOOK.md`, `docs/postmortems/TEMPLATE.md`, `bench/outage.sh` | template; the first real postmortem comes from your outage run |
| GenAI for observability (triage, log analysis) | `internal/triage`, `cmd/triage` | tested with a fake API; prompt-injection fencing |
| Temporal / workflow systems | `remediation/` | unbuilt-here |
| Highly available design, failure handling | retry/backoff, backpressure, outage test, HAProxy check | tested (unit); outage experiment pending |
| Containers / orchestration | Compose stack | config (Kubernetes manifests not included) |

## What this does not prove
Running a lab is not operating production at scale. Present this as a project, and be ready to explain
what you measured, what surprised you, and what you would change.
