
.PHONY: help dev up down build test

# Container runtime configuration
CONTAINER_RUNTIME ?= podman
COMPOSE_CMD ?= $(CONTAINER_RUNTIME)-compose

# Auto-detect if using docker-compose or podman-compose
ifeq ($(CONTAINER_RUNTIME),podman)
	COMPOSE_CMD ?= podman-compose
else
	COMPOSE_CMD ?= docker-compose
endif

# Check for docker compose v2 (docker compose vs docker-compose)
ifeq ($(shell command -v docker-compose 2> /dev/null),)
ifeq ($(shell command -v podman-compose 2> /dev/null),)
	# Try docker compose v2 if docker-compose v1 not found
	COMPOSE_CMD := docker compose
endif
endif

help:
	@echo "SVGStat - SVG Analytics Platform"
	@echo ""
	@echo "Container Runtime: $(CONTAINER_RUNTIME)"
	@echo "Compose Command: $(COMPOSE_CMD)"
	@echo ""
	@echo "Available commands:"
	@echo "  dev         - Start development environment (Containers + Go server)"
	@echo "  watch       - Start development with hot reload (using air)"
	@echo "  up          - Start services only"
	@echo "  down        - Stop services"
	@echo "  build       - Build the application"
	@echo "  test        - Run tests"
	@echo "  clean       - Remove volumes and data"
	@echo "  migrate-up  - Apply database migrations"
	@echo "  migrate-status - Show migration status"
	@echo "  sync        - Manually sync Redis data to PostgreSQL"
	@echo ""
	@echo "Environment Variables:"
	@echo "  CONTAINER_RUNTIME - Set to 'podman' (default) or 'docker'"
	@echo "  COMPOSE_CMD       - Override compose command if needed"
	@echo ""
	@echo "Examples:"
	@echo "  make dev                    - Use default Podman"
	@echo "  make dev CONTAINER_RUNTIME=docker - Use Docker"
	@echo "  make watch                  - Start with hot reload"
	@echo "  make migrate-up             - Run database migrations"

dev:
	@echo "Starting development environment with $(CONTAINER_RUNTIME)..."
	$(COMPOSE_CMD) up -d
	@echo "Waiting for services to be ready..."
	sleep 5
	@echo "Starting Go server..."
	go run cmd/api/main.go

watch:
	@echo "Starting development environment with $(CONTAINER_RUNTIME)..."
	$(COMPOSE_CMD) up -d
	@echo "Waiting for services to be ready..."
	sleep 5
	@echo "Starting Go server with hot reload (air)..."
	air

migrate-up:
	@echo "Applying database migrations..."
	go run cmd/migrate/main.go up

migrate-status:
	@echo "Checking migration status..."
	go run cmd/migrate/main.go status

sync:
	@echo "Syncing Redis data to PostgreSQL..."
	go run cmd/sync/main.go

up:
	@echo "Starting services with $(CONTAINER_RUNTIME)..."
	$(COMPOSE_CMD) up -d

down:
	@echo "Stopping services..."
	$(COMPOSE_CMD) down

build:
	@echo "Building application..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o bin/svgstat cmd/api/main.go

test:
	@echo "Running tests..."
	go test -v ./...

clean:
	@echo "Cleaning up..."
	$(COMPOSE_CMD) down -v
	rm -f bin/svgstat

