# TelemetryScale

A Go telemetry ingestion pipeline and benchmark harness for evaluating the
reliability of observability infrastructure (Grafana Mimir, ClickHouse, ELK, OpenTelemetry).

**Question it answers:** when ingest load, series cardinality, or a backend outage
changes, where does the observability platform itself break, and how does it fail?

## Status
- [x] Bounded queue -> batcher -> worker pool -> sink, with Block / DropNewest backpressure
- [x] Capped exponential backoff with full jitter, graceful drain on shutdown
- [x] Stdlib-only /metrics (batch result counters, latency histogram) feeding the SLO definitions
- [x] Tests: healthy delivery, no data loss across an outage, load shedding when full
- [x] Compose stack: Mimir (multi-tenant), ClickHouse, OTel Collector, Grafana, ELK (profile)
- [ ] OTLP metrics sink to Mimir via the Collector
- [ ] ELK vs ClickHouse log ingest/query benchmark
- [ ] SLO burn-rate rule generator + Grafana dashboard
- [ ] Failure injection scripts, noisy-tenant experiment
- [ ] FleetGuard slice: FreeIPA lab + Go checks (Kerberos, LDAP, DNS, TLS expiry)

## Run
    make test
    make up                       # Mimir, ClickHouse, OTel Collector, Grafana
    go run ./cmd/loadgen -rate 20000 -duration 30s -sink clickhouse -cardinality 10000
    curl localhost:2112/metrics

## Layout
    cmd/loadgen            load generator CLI
    internal/pipeline      queue, batching, workers, retry/backoff, backpressure
    internal/sink          ClickHouse HTTP sink, FakeSink (latency/failure injection)
    internal/metrics       dependency-free Prometheus exposition
    deploy/                compose configs: mimir, clickhouse, otel, grafana, logstash, slo
    bench/                 benchmark scripts and results (reproducible)
    docs/                  design notes and trade-offs
