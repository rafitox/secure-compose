.PHONY: build install clean test lint fmt run help

# Build settings
BINARY_NAME=secure-compose
INSTALL_PATH=$(HOME)/.local/bin
VERSION?=v0.3.0
LDFLAGS=-ldflags "-X github.com/rafitox/secure-compose/internal/cli.Version=$(VERSION)"

# Go settings
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .

build-all: ## Build for all platforms
	@echo "Building for linux/amd64..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .
	@echo "Building for darwin/amd64..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 .
	@echo "Building for darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .
	@echo "Done!"

install: build ## Install to ~/.local/bin
	@echo "Installing to $(INSTALL_PATH)/$(BINARY_NAME)..."
	@mkdir -p $(INSTALL_PATH)
	cp $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "Installed! Add $(INSTALL_PATH) to your PATH if needed."

install-system: build ## Install system-wide (requires sudo)
	@echo "Installing to /usr/local/bin/$(BINARY_NAME)..."
	@sudo cp $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@sudo chmod +x /usr/local/bin/$(BINARY_NAME)

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f $(BINARY_NAME)-*
	@rm -f cover.out
	@echo "Done!"

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=cover.out ./...
	$(GOCMD) tool cover -html=cover.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint: ## Run linter
	@echo "Running linter..."
	$(GOLINT) run ./...

fmt: ## Format code
	@echo "Formatting..."
	$(GOFMT) -s -w .

run: build ## Build and run
	@echo "Running..."
	./$(BINARY_NAME) $(ARGS)

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

check: deps lint test ## Run all checks (deps, lint, test)

# Development helpers
dev-deps: ## Install development dependencies
	@echo "Installing dev tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

bump-version: ## Bump version (usage: make bump-version VERSION=1.2.3)
	@sed -i 's/Version=.*"/Version="$(VERSION)"/' Makefile
	@echo "Version bumped to $(VERSION)"
