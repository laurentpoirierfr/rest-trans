# Compose management for this project
# Supports both Podman and Docker automatically.

ENGINE ?= $(shell if command -v podman >/dev/null 2>&1; then echo podman; elif command -v docker >/dev/null 2>&1; then echo docker; else echo docker; fi)
COMPOSE ?= $(if $(filter podman,$(ENGINE)),podman compose,docker compose)
COMPOSE_FILE ?= $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/compose.yaml)
PROJECT_NAME ?= rest-trans

.PHONY: help up down build rebuild logs ps restart stop shell-db shell-app clean

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  make up          Start services (build if needed)"
	@echo "  make down        Stop and remove services"
	@echo "  make build       Build images"
	@echo "  make rebuild     Rebuild and recreate services"
	@echo "  make logs        Follow container logs"
	@echo "  make ps          List running containers"
	@echo "  make restart     Restart services"
	@echo "  make stop        Stop services"
	@echo "  make shell-db    Open a shell in the database container"
	@echo "  make shell-app   Open a shell in the app container"
	@echo "  make clean       Stop containers and remove volumes"
	@echo ""
	@echo "Using compose engine: $(ENGINE)"

up:
	$(COMPOSE) -f $(COMPOSE_FILE) -p $(PROJECT_NAME) up -d --build

down:
	$(COMPOSE) -f $(COMPOSE_FILE) -p $(PROJECT_NAME) down

build:
	$(COMPOSE) -f $(COMPOSE_FILE) -p $(PROJECT_NAME) build

rebuild:
	$(COMPOSE) -f $(COMPOSE_FILE) -p $(PROJECT_NAME) up -d --build --force-recreate

logs:
	$(COMPOSE) -f $(COMPOSE_FILE) -p $(PROJECT_NAME) logs -f

ps:
	$(COMPOSE) -f $(COMPOSE_FILE) -p $(PROJECT_NAME) ps

restart:
	$(COMPOSE) -f $(COMPOSE_FILE) -p $(PROJECT_NAME) restart

stop:
	$(COMPOSE) -f $(COMPOSE_FILE) -p $(PROJECT_NAME) stop

shell-db:
	$(COMPOSE) -f $(COMPOSE_FILE) -p $(PROJECT_NAME) exec db sh

shell-app:
	$(COMPOSE) -f $(COMPOSE_FILE) -p $(PROJECT_NAME) exec app sh

clean:
	$(COMPOSE) -f $(COMPOSE_FILE) -p $(PROJECT_NAME) down -v --remove-orphans
