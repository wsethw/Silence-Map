.PHONY: tidy fmt test build run up down logs db-refresh

tidy:
	go mod tidy

fmt:
	go fmt ./...

test:
	go test ./...

build:
	go build ./cmd/server

run:
	go run ./cmd/server

up:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f api db

db-refresh:
	docker compose exec db psql -U postgres -d silence_map -c "REFRESH MATERIALIZED VIEW place_aggregates;"
