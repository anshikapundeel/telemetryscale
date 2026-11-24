#!/usr/bin/env bash
# Throughput ramp against ClickHouse. Output: bench/results/ramp.txt
set -euo pipefail
cd "$(dirname "$0")/.."; mkdir -p bench/results; go build -o bin/loadgen ./cmd/loadgen
for rate in 5000 10000 20000 40000 80000 160000; do
  echo "== rate=$rate =="; ./bin/loadgen -rate $rate -duration 30s -sink clickhouse -cardinality 10000 -listen :0 | tee -a bench/results/ramp.txt
done
