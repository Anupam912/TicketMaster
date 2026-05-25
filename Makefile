.PHONY: help build up down logs restart migrate clean dev test

# Default target
help:
	@echo "TicketMaster - Event Booking System"
	@echo ""
	@echo "Usage:"
	@echo "  make build      Build Docker images"
	@echo "  make up         Start all services"
	@echo "  make down       Stop all services"
	@echo "  make restart    Restart all services"
	@echo "  make logs       View logs (follow mode)"
	@echo "  make migrate    Run database migrations"
	@echo "  make clean      Remove containers, volumes, and images"
	@echo "  make dev        Start in development mode (with logs)"
	@echo "  make ps         Show running containers"
	@echo "  make shell-app  Open shell in app container"
	@echo "  make shell-db   Open psql in postgres container"
	@echo "  make shell-redis Open redis-cli in redis container"

# Build Docker images
build:
	docker compose build

# Start all services (detached)
up:
	docker compose up -d
	@echo ""
	@echo "Services starting..."
	@echo "  API:      http://localhost:8080"
	@echo "  Postgres: localhost:5432"
	@echo "  Redis:    localhost:6379"
	@echo ""
	@echo "Run 'make migrate' to initialize the database"
	@echo "Run 'make logs' to view logs"

# Stop all services
down:
	docker compose down

# Restart services
restart: down up

# View logs
logs:
	docker compose logs -f

# View logs for specific service
logs-app:
	docker compose logs -f app

logs-db:
	docker compose logs -f postgres

logs-redis:
	docker compose logs -f redis

# Run database migrations
migrate:
	docker compose run --rm migrate

# Clean up everything (containers, volumes, images)
clean:
	docker compose down -v --rmi local
	@echo "Cleaned up containers, volumes, and local images"

# Development mode - build and start with logs
dev: build up
	docker compose logs -f app

# Show running containers
ps:
	docker compose ps

# Open shell in app container
shell-app:
	docker compose exec app sh

# Open psql in postgres container
shell-db:
	docker compose exec postgres psql -U postgres -d event_ticketing

# Open redis-cli in redis container
shell-redis:
	docker compose exec redis redis-cli

# Health check
health:
	@echo "Checking service health..."
	@docker compose ps
	@echo ""
	@echo "API Health:"
	@curl -sf http://localhost:8080/health && echo "OK" || echo "API not responding"
