.PHONY: up up-elk up-fleet down test fixtures rules run fleet-once triage
up:        ; docker compose up -d
up-elk:    ; docker compose --profile elk up -d
up-fleet:  ; docker compose -f docker-compose.fleet.yml up -d
down:      ; docker compose --profile elk down -v; docker compose -f docker-compose.fleet.yml down -v
test:      ; go vet ./... && go test -race -count=1 ./... && cd remediation && go mod tidy && go vet ./... && go test -count=1 ./...
fixtures:  ; sed -i.bak "s/last_run: .*/last_run: $$(date +%s)/" lab/puppet/last_run_summary.yaml && rm -f lab/puppet/last_run_summary.yaml.bak
rules:     ; ./bench/load-rules.sh
run:       ; go run ./cmd/loadgen -rate 20000 -duration 30s -sink otlp -trace-url http://localhost:4318/v1/traces
fleet-once:; go run ./cmd/fleetguard -once
triage:    ; go run ./cmd/triage
