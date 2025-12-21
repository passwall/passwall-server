# Makefile for PassWall Server
.PHONY: help build test lint generate up down clean image-build image-publish install-tools run dev logs

# Variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%S)
COMMIT_ID := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
DOCKER_IMAGE ?= passwall/passwall-server
DOCKER_TAG ?= latest
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BUILD_DIR := build
DOCKER_DIR := $(BUILD_DIR)/docker

# Build flags
GO_BUILD_LDFLAGS := -s -w
GO_BUILD_LDFLAGS += -X github.com/passwall/passwall-server/pkg/buildvars.Version=$(VERSION)
GO_BUILD_LDFLAGS += -X github.com/passwall/passwall-server/pkg/buildvars.BuildTime=$(BUILD_TIME)
GO_BUILD_LDFLAGS += -X github.com/passwall/passwall-server/pkg/buildvars.CommitID=$(COMMIT_ID)
GO_BUILD_TAGS := netgo osusergo

# Colors for output
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

##@ General

help: ## Display this help message
	@echo "$(BLUE)PassWall Server - Makefile Commands$(NC)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make $(GREEN)<target>$(NC)\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(GREEN)%-18s$(NC) %s\n", $$1, $$2 } /^##@/ { printf "\n$(BLUE)%s$(NC)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

generate: ## Run go generate
	@echo "$(BLUE)Running go generate...$(NC)"
	@go generate ./...
	@echo "$(GREEN)✓ Generate completed$(NC)"

install-tools: ## Install development tools (golangci-lint, gocov)
	@echo "$(BLUE)Installing development tools...$(NC)"
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.62.2; \
	}
	@command -v gocov >/dev/null 2>&1 || { \
		echo "Installing gocov..."; \
		go install github.com/axw/gocov/gocov@latest; \
	}
	@echo "$(GREEN)✓ Tools installed$(NC)"

lint: ## Run golangci-lint
	@echo "$(BLUE)Running linter...$(NC)"
	@golangci-lint run --timeout 15m ./...
	@echo "$(GREEN)✓ Lint passed$(NC)"

test: ## Run tests
	@echo "$(BLUE)Running tests...$(NC)"
	@go test -v -race -cover -coverprofile=coverage.out ./...
	@echo "$(GREEN)✓ Tests completed$(NC)"

test-coverage: test ## Run tests with coverage report
	@echo "$(BLUE)Generating coverage report...$(NC)"
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓ Coverage report generated: coverage.html$(NC)"

##@ Build

build: generate ## Build server and CLI binaries
	@echo "$(BLUE)Building PassWall Server...$(NC)"
	@echo "Version: $(YELLOW)$(VERSION)$(NC)"
	@echo "Commit: $(YELLOW)$(COMMIT_ID)$(NC)"
	@echo "Build Time: $(YELLOW)$(BUILD_TIME)$(NC)"
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -tags "$(GO_BUILD_TAGS)" \
		-ldflags "$(GO_BUILD_LDFLAGS)" -o $(BUILD_DIR)/passwall-server ./cmd/passwall-server
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -tags "$(GO_BUILD_TAGS)" \
		-ldflags "$(GO_BUILD_LDFLAGS)" -o $(BUILD_DIR)/passwall-cli ./cmd/passwall-cli
	@echo "$(BLUE)Copying config files...$(NC)"
	@cp -r config $(BUILD_DIR)/config 2>/dev/null || echo "$(YELLOW)⚠ No config folder found$(NC)"
	@echo "$(GREEN)✓ Build completed successfully$(NC)"
	@echo "Binaries: $(YELLOW)$(BUILD_DIR)/passwall-server$(NC) and $(YELLOW)$(BUILD_DIR)/passwall-cli$(NC)"
	@echo "Binaries: $(YELLOW)$(BUILD_DIR)/passwall-server$(NC) and $(YELLOW)$(BUILD_DIR)/passwall-cli$(NC)"

build-linux: ## Build for Linux
	@echo "$(BLUE)Building for Linux...$(NC)"
	@GOOS=linux GOARCH=amd64 $(MAKE) build
	@echo "$(GREEN)✓ Linux build completed$(NC)"

build-darwin: ## Build for macOS
	@echo "$(BLUE)Building for macOS...$(NC)"
	@GOOS=darwin GOARCH=arm64 $(MAKE) build
	@echo "$(GREEN)✓ macOS build completed$(NC)"

build-all: build-linux build-darwin ## Build for all platforms

clean: ## Clean build artifacts
	@echo "$(BLUE)Cleaning build artifacts...$(NC)"
	@rm -rf $(BUILD_DIR)/passwall-server $(BUILD_DIR)/passwall-cli
	@rm -f coverage.out coverage.html
	@rm -f *-cover.out
	@echo "$(GREEN)✓ Cleaned$(NC)"

##@ Docker

image-build: ## Build Docker image
	@echo "$(BLUE)Building Docker image...$(NC)"
	@echo "Image: $(YELLOW)$(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"
	@docker build -f $(DOCKER_DIR)/Dockerfile -t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg COMMIT_ID=$(COMMIT_ID) .
	@echo "$(GREEN)✓ Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"

image-publish: image-build ## Build and publish Docker image to Docker Hub
	@echo "$(BLUE)Publishing Docker image to Docker Hub...$(NC)"
	@docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):$(VERSION)
	@docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	@docker push $(DOCKER_IMAGE):$(VERSION)
	@echo "$(GREEN)✓ Docker image published:$(NC)"
	@echo "  - $(YELLOW)$(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"
	@echo "  - $(YELLOW)$(DOCKER_IMAGE):$(VERSION)$(NC)"

##@ Docker Compose

up: ## Start all services with Docker Compose
	@echo "$(BLUE)Starting PassWall Server with Docker Compose...$(NC)"
	@mkdir -p $(DOCKER_DIR)/postgres-data
	@test -f $(DOCKER_DIR)/config.yml || cp config/config.yml $(DOCKER_DIR)/config.yml
	@test -f $(DOCKER_DIR)/passwall-server.log || touch $(DOCKER_DIR)/passwall-server.log
	@cd $(DOCKER_DIR) && PATH="/Users/yakuter/.docker/bin:$$PATH" docker compose up -d --build
	@echo "$(GREEN)✓ Services started$(NC)"
	@echo "Server running at: $(YELLOW)http://localhost:3625$(NC)"
	@echo "PostgreSQL: $(YELLOW)localhost:5432$(NC)"
	@echo "View logs with: $(YELLOW)make logs$(NC)"

down: ## Stop all Docker services
	@echo "$(BLUE)Stopping services...$(NC)"
	@cd $(DOCKER_DIR) && PATH="/Users/yakuter/.docker/bin:$$PATH" docker compose down
	@echo "$(GREEN)✓ Services stopped$(NC)"

restart: down up ## Restart all services

logs: ## Show logs from Docker Compose services
	@cd $(DOCKER_DIR) && PATH="/Users/yakuter/.docker/bin:$$PATH" docker compose logs -f

ps: ## Show running Docker Compose services
	@cd $(DOCKER_DIR) && PATH="/Users/yakuter/.docker/bin:$$PATH" docker compose ps

##@ Local Development

run: build ## Build and run server locally (without Docker)
	@echo "$(BLUE)Starting PassWall Server locally...$(NC)"
	@echo "$(YELLOW)⚠ Make sure PostgreSQL is running (make up or external)$(NC)"
	@$(BUILD_DIR)/passwall-server

dev: ## Run server in development mode with auto-reload (requires air)
	@echo "$(BLUE)Starting development server...$(NC)"
	@echo "$(YELLOW)⚠ Make sure PostgreSQL is running (make up or external)$(NC)"
	@command -v air >/dev/null 2>&1 || { \
		echo "$(YELLOW)Installing air for hot reload...$(NC)"; \
		go install github.com/air-verse/air@latest; \
	}
	@air

create-user: build ## Create a new user with CLI
	@echo "$(BLUE)Creating new user...$(NC)"
	@$(BUILD_DIR)/passwall-cli

##@ Database

db-up: ## Start only PostgreSQL database
	@echo "$(BLUE)Starting PostgreSQL...$(NC)"
	@docker compose -f $(DOCKER_DIR)/docker-compose.yml up -d postgres
	@echo "$(GREEN)✓ PostgreSQL started$(NC)"

db-down: ## Stop PostgreSQL database
	@echo "$(BLUE)Stopping PostgreSQL...$(NC)"
	@docker compose -f $(DOCKER_DIR)/docker-compose.yml stop postgres
	@echo "$(GREEN)✓ PostgreSQL stopped$(NC)"

db-logs: ## Show PostgreSQL logs
	@docker compose -f $(DOCKER_DIR)/docker-compose.yml logs -f postgres

db-reset: ## Reset database (removes all data)
	@echo "$(RED)⚠️  This will delete all database data!$(NC)"
	@echo "$(YELLOW)Press Ctrl+C to cancel or Enter to continue...$(NC)"
	@read
	@docker compose -f $(DOCKER_DIR)/docker-compose.yml down -v
	@echo "$(GREEN)✓ Database reset completed$(NC)"

##@ Volume Management

volumes-list: ## List all Docker volumes
	@echo "$(BLUE)Docker volumes:$(NC)"
	@docker volume ls | grep passwall || echo "$(YELLOW)No PassWall volumes found$(NC)"

volumes-inspect: ## Inspect volume details
	@echo "$(BLUE)PostgreSQL volume:$(NC)"
	@docker volume inspect passwall-server_postgres_data 2>/dev/null || echo "$(YELLOW)Volume not found$(NC)"
	@echo ""
	@echo "$(BLUE)PassWall data volume:$(NC)"
	@docker volume inspect passwall-server_passwall_data 2>/dev/null || echo "$(YELLOW)Volume not found$(NC)"

volumes-backup: ## Backup volumes to local directory
	@echo "$(BLUE)Creating backup...$(NC)"
	@mkdir -p backups
	@docker run --rm -v passwall-server_postgres_data:/data -v $$(pwd)/backups:/backup alpine tar czf /backup/postgres-$$(date +%Y%m%d-%H%M%S).tar.gz -C /data .
	@docker run --rm -v passwall-server_passwall_data:/data -v $$(pwd)/backups:/backup alpine tar czf /backup/passwall-$$(date +%Y%m%d-%H%M%S).tar.gz -C /data .
	@echo "$(GREEN)✓ Backup completed in ./backups/$(NC)"

volumes-clean: ## Remove all PassWall volumes (DANGER: deletes all data)
	@echo "$(RED)⚠️  WARNING: This will delete ALL PassWall data!$(NC)"
	@echo "$(YELLOW)Press Ctrl+C to cancel or Enter to continue...$(NC)"
	@read
	@docker compose -f $(DOCKER_DIR)/docker-compose.yml down -v
	@docker volume rm passwall-server_postgres_data passwall-server_passwall_data 2>/dev/null || true
	@echo "$(GREEN)✓ Volumes cleaned$(NC)"

##@ CI/CD

ci: install-tools lint test build ## Run CI pipeline (lint, test, build)
	@echo "$(GREEN)✓ CI pipeline completed successfully$(NC)"

check: lint test ## Run checks (lint and test)
	@echo "$(GREEN)✓ All checks passed$(NC)"

##@ Information

version: ## Show version information
	@echo "Version: $(YELLOW)$(VERSION)$(NC)"
	@echo "Commit: $(YELLOW)$(COMMIT_ID)$(NC)"
	@echo "Build Time: $(YELLOW)$(BUILD_TIME)$(NC)"
	@echo "Go Version: $(YELLOW)$(shell go version)$(NC)"

info: version ## Show build information
	@echo "GOOS: $(YELLOW)$(GOOS)$(NC)"
	@echo "GOARCH: $(YELLOW)$(GOARCH)$(NC)"
	@echo "Docker Image: $(YELLOW)$(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"
