#!/usr/bin/env bash
# Step series cardinality at a fixed rate; record Mimir memory after each step.
set -euo pipefail
cd "$(dirname "$0")/.."; mkdir -p bench/results; go build -o bin/loadgen ./cmd/loadgen
for card in 1000 10000 100000 1000000; do
  ./bin/loadgen -rate 20000 -duration 45s -cardinality $card -sink otlp -listen :0 | tee -a bench/results/cardinality.txt
  docker stats --no-stream --format "cardinality=$card mimir_mem={{.MemUsage}}" "$(docker compose ps -q mimir)" | tee -a bench/results/cardinality.txt
done
