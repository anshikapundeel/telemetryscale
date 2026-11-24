#!/usr/bin/env bash
# tenant-b floods high-cardinality series into Mimir while tenant-a stays steady.
# Success = tenant-a unaffected; tenant-b rejected by per-tenant limits (see deploy/mimir/runtime.yaml).
set -euo pipefail
cd "$(dirname "$0")/.."; mkdir -p bench/results; go build -o bin/loadgen ./cmd/loadgen
go build -o bin/cardwatch ./cmd/cardwatch
./bin/loadgen -tenant tenant-a -rate 5000  -duration 60s -cardinality 1000   -sink otlp -listen :2112 | tee -a bench/results/noisy-a.txt &
./bin/loadgen -tenant tenant-b -rate 50000 -duration 60s -cardinality 500000 -sink otlp -listen :2114 | tee -a bench/results/noisy-b.txt &
wait
./bin/cardwatch -tenant tenant-b -threshold 10000 || true
