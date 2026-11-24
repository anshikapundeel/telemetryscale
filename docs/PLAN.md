# Benchmark plan
| Experiment | Variable | Measure |
|---|---|---|
| Throughput ramp | samples/sec | delivered/s, queue depth, p50/p95/p99 batch latency |
| Cardinality | unique series | Mimir ingester memory, ClickHouse insert rate |
| Outage | stop backend N seconds | loss, retries, time-to-drain after recovery |
| Backpressure | Block vs DropNewest | producer stall time vs dropped samples |
| ELK vs ClickHouse | log volume | ingest rate, disk bytes/event, query latency |
| Noisy tenant | tenant-b over its limit | tenant-a latency and rejected samples |
Each result gets: command, config, hardware, raw output, and one paragraph of interpretation.
