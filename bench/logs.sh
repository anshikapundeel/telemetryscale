#!/usr/bin/env bash
# Same logs into ClickHouse and ELK, then compare query latency and storage.
set -euo pipefail
cd "$(dirname "$0")/.."; mkdir -p bench/results; go build -o bin/loadgen ./cmd/loadgen; go build -o bin/logquery ./cmd/logquery
docker compose --profile elk up -d; sleep 60
./bin/loadgen -mode logs -rate 20000 -duration 60s -sink clickhouse -listen :0 | tee -a bench/results/logs.txt
./bin/loadgen -mode logs -rate 20000 -duration 60s -sink logstash   -listen :0 | tee -a bench/results/logs.txt
sleep 10; ./bin/logquery -n 30 | tee -a bench/results/logs.txt
