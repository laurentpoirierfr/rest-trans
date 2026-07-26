# rest-trans Makefile

APP_NAME := rest-trans
IMAGE_NAME := rest-trans
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Go
.PHONY: build run clean fmt lint vet

build:
	CGO_ENABLED=0 go build -o bin/$(APP_NAME) ./cmd/rest-trans

run:
	go run ./cmd/rest-trans/

clean:
	rm -rf bin/

fmt:
	gofmt -s -w .

lint: vet
	@echo "Linting..."
	@golangci-lint run ./... 2>/dev/null || echo "golangci-lint not installed, skipping"

vet:
	go vet ./...

# Tests
.PHONY: test test-unit test-integration test-verbose test-race

test:
	go test ./tests/ -count=1

test-unit:
	go test ./internal/... -count=1

test-integration:
	go test ./tests/ -v -count=1

test-verbose:
	go test ./tests/ -v -count=1

test-race:
	go test ./tests/ -race -count=1

test-short:
	go test ./tests/ -short -count=1

# Docker
.PHONY: docker-build docker-run docker-stop docker-logs docker-compose-up docker-compose-down

docker-build:
	docker build -t $(IMAGE_NAME):$(VERSION) -t $(IMAGE_NAME):latest .

docker-run: docker-build
	docker run -d \
		--name $(APP_NAME) \
		-p 3000:3000 \
		-e DB_HOST=host.docker.internal \
		-e DB_PORT=5432 \
		-e DB_USER=postgres \
		-e DB_PASS=postgres \
		-e DB_NAME=app \
		$(IMAGE_NAME):latest

docker-stop:
	docker stop $(APP_NAME) 2>/dev/null || true
	docker rm $(APP_NAME) 2>/dev/null || true

docker-logs:
	docker logs -f $(APP_NAME)

docker-compose-up:
	docker compose -f infras/compose.yaml up -d --build

docker-compose-down:
	docker compose -f infras/compose.yaml down

docker-compose-logs:
	docker compose -f infras/compose.yaml logs -f

# Développement
.PHONY: dev dev-docker dev-test

dev:
	DB_HOST=localhost DB_PORT=5432 DB_USER=postgres DB_PASS=postgres DB_NAME=app go run ./cmd/rest-trans/

dev-docker: docker-compose-up
	@echo "Waiting for PostgreSQL..."
	@sleep 3
	@echo "Server starting..."
	@DB_HOST=localhost DB_PORT=5432 DB_USER=postgres DB_PASS=postgres DB_NAME=app go run ./cmd/rest-trans/

dev-test: test-integration

# Introspection / Info
.PHONY: info openapi

info:
	@curl -s http://localhost:3000/info | python3 -m json.tool 2>/dev/null || curl -s http://localhost:3000/info

openapi:
	@curl -s http://localhost:3000/openapi.json | python3 -m json.tool 2>/dev/null || curl -s http://localhost:3000/openapi.json

# Aide
.PHONY: help

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build & Run:"
	@echo "  build              Build binary"
	@echo "  run                Run locally"
	@echo "  clean              Remove binaries"
	@echo ""
	@echo "Code Quality:"
	@echo "  fmt                Format code"
	@echo "  lint               Run linter"
	@echo "  vet                Run go vet"
	@echo ""
	@echo "Tests:"
	@echo "  test               Run integration tests"
	@echo "  test-unit          Run unit tests only"
	@echo "  test-integration   Run integration tests (verbose)"
	@echo "  test-verbose       Run tests with verbose output"
	@echo "  test-race          Run tests with race detector"
	@echo "  test-short         Run tests in short mode"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build       Build Docker image"
	@echo "  docker-run         Run container"
	@echo "  docker-stop        Stop container"
	@echo "  docker-logs        View logs"
	@echo "  docker-compose-up  Start with Docker Compose"
	@echo "  docker-compose-down Stop Docker Compose"
	@echo ""
	@echo "Development:"
	@echo "  dev                Run locally with env vars"
	@echo "  dev-docker         Start DB + run app"
	@echo "  info               Fetch /info endpoint"
	@echo "  openapi            Fetch /openapi.json"
