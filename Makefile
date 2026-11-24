.PHONY: up up-elk down test run bench
up:      ; docker compose up -d
up-elk:  ; docker compose --profile elk up -d
down:    ; docker compose --profile elk down -v
test:    ; go test -race ./...
run:     ; go run ./cmd/loadgen -rate 20000 -duration 30s
