CREATE DATABASE IF NOT EXISTS telemetry;

CREATE TABLE IF NOT EXISTS telemetry.logs
(
    ts          DateTime64(3) CODEC(Delta, ZSTD),
    tenant      LowCardinality(String),
    service     LowCardinality(String),
    severity    LowCardinality(String),
    host        LowCardinality(String),
    trace_id    String,
    body        String CODEC(ZSTD(3)),
    attrs       Map(String, String)
)
ENGINE = MergeTree
PARTITION BY toDate(ts)
ORDER BY (tenant, service, severity, ts)
TTL toDateTime(ts) + INTERVAL 7 DAY;

CREATE TABLE IF NOT EXISTS telemetry.spans
(
    ts          DateTime64(6),
    tenant      LowCardinality(String),
    service     LowCardinality(String),
    trace_id    String,
    span_id     String,
    parent_id   String,
    name        LowCardinality(String),
    duration_ns UInt64,
    status      LowCardinality(String)
)
ENGINE = MergeTree
PARTITION BY toDate(ts)
ORDER BY (tenant, service, trace_id, ts)
TTL toDateTime(ts) + INTERVAL 3 DAY;

-- Raw metrics sink used by cmd/loadgen (HTTP JSONEachRow). ts is epoch millis.
CREATE TABLE IF NOT EXISTS telemetry.metrics_raw
(
    ts      DateTime64(3) CODEC(Delta, ZSTD),
    tenant  LowCardinality(String),
    name    LowCardinality(String),
    labels  Map(String, String),
    value   Float64 CODEC(Gorilla, ZSTD)
)
ENGINE = MergeTree
PARTITION BY toDate(ts)
ORDER BY (tenant, name, ts)
TTL toDateTime(ts) + INTERVAL 7 DAY;
