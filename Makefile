APP_NAME=app
CMD_DIR=./cmd/app
BIN_DIR=./bin
VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD)
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -s -w \
	-X main.Version=$(VERSION) \
	-X main.Commit=$(COMMIT) \
	-X main.BuildDate=$(BUILD_DATE)

.PHONY: help build run clean test coverage fmt vet lint check tidy deps

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "%-25s %s\n", $$1, $$2}'

version: ## Show build metadata
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Build:   $(BUILD_DATE)"

docker-build: ## Build docker image
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(APP_NAME):latest .

build: ## Build application
	@mkdir -p $(BIN_DIR)
	go build \
		-ldflags "$(LDFLAGS)" \
		-o $(BIN_DIR)/$(APP_NAME) \
		$(CMD_DIR)

release: ## Build release binary
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build \
		-ldflags "$(LDFLAGS)" \
		-o dist/$(APP_NAME) \
		$(CMD_DIR)

run: ## Run application
	go run $(CMD_DIR) serve

test: ## Run tests
	go test ./...

coverage: ## Run coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

fmt: ## Format code
	go fmt ./...

vet: ## Vet code
	go vet ./...

lint: ## Run golangci-lint
	golangci-lint run

check: fmt vet lint test ## Full validation

tidy: ## Go mod tidy
	go mod tidy

deps: ## Download dependencies
	go mod download

clean: ## Remove build artifacts
	rm -rf bin dist coverage.out

migrate-up: ## Apply migrations
	go run $(CMD_DIR) migrate up

migrate-down: ## Rollback one migration
	go run $(CMD_DIR) migrate down --steps 1

migrate-create: ## Create migration
	go run $(CMD_DIR) migrate create $(name)