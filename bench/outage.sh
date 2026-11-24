#!/usr/bin/env bash
# Backend outage + recovery. Stops ClickHouse for OUTAGE seconds mid-run and reports loss.
# Run with different MAX_RETRIES / MAX_BACKOFF to find where data starts being dropped.
set -euo pipefail
cd "$(dirname "$0")/.."; mkdir -p bench/results; go build -o bin/loadgen ./cmd/loadgen
OUTAGE=${OUTAGE:-20}; MAX_RETRIES=${MAX_RETRIES:-5}; MAX_BACKOFF=${MAX_BACKOFF:-5s}
./bin/loadgen -rate 20000 -duration 70s -sink clickhouse -max-retries "$MAX_RETRIES" -max-backoff "$MAX_BACKOFF" -listen :2112 \
  | tee -a bench/results/outage.txt &
PID=$!
sleep 15; echo "[chaos] stopping clickhouse for ${OUTAGE}s"; docker compose stop clickhouse
sleep "$OUTAGE"; echo "[chaos] starting clickhouse"; docker compose start clickhouse
wait $PID
