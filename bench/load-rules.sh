#!/usr/bin/env bash
# Generate SLO burn-rate rules and load them into Mimir's ruler for tenant-a.
set -euo pipefail
cd "$(dirname "$0")/.."
go run ./cmd/sloctl -o /tmp/slo-rules.yaml
curl -sS -X POST -H "Content-Type: application/yaml" -H "X-Scope-OrgID: tenant-a" \
  --data-binary @/tmp/slo-rules.yaml http://localhost:9009/prometheus/config/v1/rules/slo && echo "rules loaded"
