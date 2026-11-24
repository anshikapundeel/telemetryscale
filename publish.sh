#!/usr/bin/env bash
# Run from the repo root on your machine. Uses your own git identity and GitHub login.
set -euo pipefail
REPO="${1:-telemetryscale}"
USER_NAME="${2:-anshikapundeel}"

[ -d .git ] || git init -b main
git add .gitignore go.mod LICENSE README.md Makefile
git commit -m "chore: project skeleton, license, makefile" || true
git add internal cmd
git commit -m "feat: bounded-queue ingest pipeline with backpressure, retry/backoff, metrics" || true
git add deploy docker-compose.yml
git commit -m "feat: compose stack (Mimir, ClickHouse, OTel, Grafana, ELK) and SLO definitions" || true
git add docs .github
git commit -m "docs: benchmark plan; ci: vet and race tests" || true

if command -v gh >/dev/null 2>&1; then
  gh repo create "$USER_NAME/$REPO" --public --source=. --remote=origin --push \
    --description "Go telemetry ingestion pipeline and benchmark harness for Mimir, ClickHouse, ELK and OpenTelemetry"
else
  git remote add origin "git@github.com:$USER_NAME/$REPO.git" 2>/dev/null || true
  echo "No gh CLI found. Create an empty repo named $REPO on github.com, then run: git push -u origin main"
fi
