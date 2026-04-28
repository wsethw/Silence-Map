.PHONY: tidy fmt test frontend-test build run up down logs

tidy:
	go mod tidy

fmt:
	go fmt ./...

test:
	go test ./...
	node web/index.test.js

frontend-test:
	node web/index.test.js

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
