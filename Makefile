.PHONY: all build build-prod run test clean docker-build docker-up docker-down migrate frontend-dev frontend-build

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=whatomate
BINARY_PATH=./cmd/whatomate
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# Docker parameters
DOCKER_COMPOSE=docker compose -f docker/docker-compose.yml

all: build

# Build the backend (development - without frontend)
build:
	$(GOBUILD) -o $(BINARY_NAME) $(BINARY_PATH)

# Build production binary with embedded frontend
build-prod: frontend-build embed-frontend
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(BINARY_PATH)
	@echo "Production binary built: $(BINARY_NAME)"
	@echo "Version: $(VERSION)"
	@ls -lh $(BINARY_NAME)

# Copy frontend build to embed directory
embed-frontend:
	@echo "Copying frontend build to embed directory..."
	@rm -rf internal/frontend/dist/*
	@cp -r frontend/dist/* internal/frontend/dist/
	@echo "Frontend embedded successfully"

# Run the backend locally
run:
	$(GOCMD) run $(BINARY_PATH)/main.go server -config config.toml

# Run with migrations
run-migrate:
	$(GOCMD) run $(BINARY_PATH)/main.go server -config config.toml -migrate

# Run tests. Uses gotestsum when available for live progress + a clear
# failure summary at the end. Falls back to the built-in `go test -v` so
# nothing breaks for devs who haven't installed it.
# Install:  go install gotest.tools/gotestsum@latest
test:
	@if command -v gotestsum >/dev/null 2>&1; then \
		gotestsum --format testname --hide-summary=skipped -- ./...; \
	else \
		echo "(install gotestsum for nicer output: go install gotest.tools/gotestsum@latest)"; \
		$(GOTEST) -v ./...; \
	fi

# Run tests with coverage. Same gotestsum fallback as `make test`.
test-coverage:
	@if command -v gotestsum >/dev/null 2>&1; then \
		gotestsum --format testname --hide-summary=skipped -- -coverprofile=coverage.out ./...; \
	else \
		echo "(install gotestsum for nicer output: go install gotest.tools/gotestsum@latest)"; \
		$(GOTEST) -v -coverprofile=coverage.out ./...; \
	fi
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Update dependencies
deps-update:
	$(GOMOD) tidy
	$(GOGET) -u ./...

# Docker commands
docker-build:
	$(DOCKER_COMPOSE) build

docker-up:
	$(DOCKER_COMPOSE) up -d

docker-down:
	$(DOCKER_COMPOSE) down

docker-logs:
	$(DOCKER_COMPOSE) logs -f

docker-restart:
	$(DOCKER_COMPOSE) restart

# Database migrations
migrate:
	$(GOCMD) run $(BINARY_PATH)/main.go server -config config.toml -migrate

# Frontend commands
frontend-install:
	cd frontend && npm install

frontend-dev:
	@if [ ! -d "frontend/node_modules" ]; then \
		echo "Installing frontend dependencies..."; \
		cd frontend && npm install; \
	fi
	cd frontend && npm run dev

frontend-build:
	@if [ ! -d "frontend/node_modules" ]; then \
		echo "Installing frontend dependencies..."; \
		cd frontend && npm install; \
	fi
	cd frontend && npm run build

frontend-preview:
	cd frontend && npm run preview

# Development - run both backend and frontend
dev:
	@echo "Starting backend and frontend in development mode..."
	@make run-migrate &
	@make frontend-dev

# Lint
lint:
	golangci-lint run ./...

# Format code
fmt:
	$(GOCMD) fmt ./...

# Generate swagger docs (if using)
swagger:
	swag init -g cmd/whatomate/main.go -o api/docs

# Help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Production:"
	@echo "  build-prod     - Build single binary with embedded frontend"
	@echo ""
	@echo "Development:"
	@echo "  build          - Build the backend binary (without frontend)"
	@echo "  run            - Run the backend locally"
	@echo "  run-migrate    - Run the backend with database migrations"
	@echo "  dev            - Run both backend and frontend in development mode"
	@echo ""
	@echo "Frontend:"
	@echo "  frontend-install - Install frontend dependencies"
	@echo "  frontend-dev   - Run frontend in development mode"
	@echo "  frontend-build - Build frontend for production"
	@echo ""
	@echo "Testing:"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build   - Build Docker images"
	@echo "  docker-up      - Start Docker containers"
	@echo "  docker-down    - Stop Docker containers"
	@echo "  docker-logs    - View Docker logs"
	@echo ""
	@echo "Other:"
	@echo "  clean          - Remove build artifacts"
	@echo "  deps           - Download dependencies"
	@echo "  lint           - Run linter"
	@echo "  fmt            - Format code"
