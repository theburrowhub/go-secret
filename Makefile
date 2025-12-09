# ══════════════════════════════════════════════════════════════════════════════
#  🔐 go-secrets - GCP Secret Manager TUI
# ══════════════════════════════════════════════════════════════════════════════

# Build configuration
BINARY_NAME := go-secrets
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE  := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION  := $(shell go version | cut -d ' ' -f 3)

# Go configuration
GOFLAGS     := -trimpath
LDFLAGS     := -s -w \
               -X main.Version=$(VERSION) \
               -X main.Commit=$(COMMIT) \
               -X main.BuildDate=$(BUILD_DATE)

# Directories
BUILD_DIR   := ./build
DIST_DIR    := ./dist

# Colors for pretty output
CYAN    := \033[36m
GREEN   := \033[32m
YELLOW  := \033[33m
RED     := \033[31m
MAGENTA := \033[35m
BOLD    := \033[1m
RESET   := \033[0m

# ══════════════════════════════════════════════════════════════════════════════
#  Default target
# ══════════════════════════════════════════════════════════════════════════════

.DEFAULT_GOAL := help

# ══════════════════════════════════════════════════════════════════════════════
#  Help
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: help
help: ## Show this help message
	@echo ""
	@echo "$(BOLD)$(CYAN)🔐 go-secrets$(RESET) $(YELLOW)v$(VERSION)$(RESET)"
	@echo "$(CYAN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(RESET)"
	@echo ""
	@echo "$(BOLD)Usage:$(RESET)  make $(GREEN)<target>$(RESET)"
	@echo ""
	@echo "$(BOLD)Targets:$(RESET)"
	@awk 'BEGIN {FS = ":.*##"} \
		/^[a-zA-Z_-]+:.*##/ { \
			printf "  $(GREEN)%-15s$(RESET) %s\n", $$1, $$2 \
		} \
		/^## / { \
			printf "\n$(BOLD)%s$(RESET)\n", substr($$0, 4) \
		}' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(CYAN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(RESET)"
	@echo "$(BOLD)Build Info:$(RESET)"
	@echo "  Version:    $(YELLOW)$(VERSION)$(RESET)"
	@echo "  Commit:     $(YELLOW)$(COMMIT)$(RESET)"
	@echo "  Go:         $(YELLOW)$(GO_VERSION)$(RESET)"
	@echo ""

# ══════════════════════════════════════════════════════════════════════════════
## Development
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: build
build: ## Build the binary
	@echo "$(CYAN)▶$(RESET) Building $(BOLD)$(BINARY_NAME)$(RESET)..."
	@go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) .
	@echo "$(GREEN)✓$(RESET) Built $(BOLD)./$(BINARY_NAME)$(RESET)"

.PHONY: run
run: build ## Build and run the application
	@echo "$(CYAN)▶$(RESET) Running $(BOLD)$(BINARY_NAME)$(RESET)..."
	@./$(BINARY_NAME) $(ARGS)

.PHONY: dev
dev: ## Run with hot reload (requires air: go install github.com/air-verse/air@latest)
	@command -v air >/dev/null 2>&1 || { \
		echo "$(YELLOW)⚠$(RESET) Installing air for hot reload..."; \
		go install github.com/air-verse/air@latest; \
	}
	@echo "$(CYAN)▶$(RESET) Starting development mode with hot reload..."
	@air

.PHONY: watch
watch: ## Watch for changes and rebuild (requires entr)
	@command -v entr >/dev/null 2>&1 || { \
		echo "$(RED)✗$(RESET) entr not found. Install with: brew install entr"; \
		exit 1; \
	}
	@echo "$(CYAN)▶$(RESET) Watching for changes..."
	@find . -name '*.go' | entr -c make build

# ══════════════════════════════════════════════════════════════════════════════
## Quality
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: fmt
fmt: ## Format code
	@echo "$(CYAN)▶$(RESET) Formatting code..."
	@go fmt ./...
	@echo "$(GREEN)✓$(RESET) Code formatted"

.PHONY: lint
lint: ## Run linter (requires golangci-lint)
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "$(YELLOW)⚠$(RESET) Installing golangci-lint..."; \
		go install github.com/golangci-lint/golangci-lint/cmd/golangci-lint@latest; \
	}
	@echo "$(CYAN)▶$(RESET) Running linter..."
	@golangci-lint run ./...
	@echo "$(GREEN)✓$(RESET) Linting passed"

.PHONY: vet
vet: ## Run go vet
	@echo "$(CYAN)▶$(RESET) Running go vet..."
	@go vet ./...
	@echo "$(GREEN)✓$(RESET) Vet passed"

.PHONY: test
test: ## Run tests
	@echo "$(CYAN)▶$(RESET) Running tests..."
	@go test -v -race -cover ./...
	@echo "$(GREEN)✓$(RESET) Tests passed"

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "$(CYAN)▶$(RESET) Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)✓$(RESET) Coverage report: $(BOLD)coverage.html$(RESET)"

.PHONY: check
check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)
	@echo "$(GREEN)✓$(RESET) All checks passed!"

# ══════════════════════════════════════════════════════════════════════════════
## Dependencies
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: deps
deps: ## Download dependencies
	@echo "$(CYAN)▶$(RESET) Downloading dependencies..."
	@go mod download
	@echo "$(GREEN)✓$(RESET) Dependencies downloaded"

.PHONY: tidy
tidy: ## Tidy go.mod
	@echo "$(CYAN)▶$(RESET) Tidying go.mod..."
	@go mod tidy
	@echo "$(GREEN)✓$(RESET) go.mod tidied"

.PHONY: update
update: ## Update all dependencies
	@echo "$(CYAN)▶$(RESET) Updating dependencies..."
	@go get -u ./...
	@go mod tidy
	@echo "$(GREEN)✓$(RESET) Dependencies updated"

.PHONY: vendor
vendor: ## Vendor dependencies
	@echo "$(CYAN)▶$(RESET) Vendoring dependencies..."
	@go mod vendor
	@echo "$(GREEN)✓$(RESET) Dependencies vendored"

# ══════════════════════════════════════════════════════════════════════════════
## Release
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: release
release: clean ## Build release binaries for all platforms
	@echo "$(CYAN)▶$(RESET) Building release binaries..."
	@mkdir -p $(DIST_DIR)
	@echo "  $(MAGENTA)→$(RESET) darwin/amd64"
	@GOOS=darwin GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@echo "  $(MAGENTA)→$(RESET) darwin/arm64"
	@GOOS=darwin GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .
	@echo "  $(MAGENTA)→$(RESET) linux/amd64"
	@GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 .
	@echo "  $(MAGENTA)→$(RESET) linux/arm64"
	@GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 .
	@echo "  $(MAGENTA)→$(RESET) windows/amd64"
	@GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe .
	@echo "$(GREEN)✓$(RESET) Release binaries built in $(BOLD)$(DIST_DIR)/$(RESET)"
	@ls -lh $(DIST_DIR)/

.PHONY: checksums
checksums: release ## Generate checksums for release binaries
	@echo "$(CYAN)▶$(RESET) Generating checksums..."
	@cd $(DIST_DIR) && shasum -a 256 * > checksums.txt
	@echo "$(GREEN)✓$(RESET) Checksums generated"
	@cat $(DIST_DIR)/checksums.txt

# ══════════════════════════════════════════════════════════════════════════════
## Install
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: install
install: build ## Install binary to GOPATH/bin
	@echo "$(CYAN)▶$(RESET) Installing $(BOLD)$(BINARY_NAME)$(RESET)..."
	@go install $(GOFLAGS) -ldflags "$(LDFLAGS)" .
	@echo "$(GREEN)✓$(RESET) Installed to $(BOLD)$(shell go env GOPATH)/bin/$(BINARY_NAME)$(RESET)"

.PHONY: uninstall
uninstall: ## Uninstall binary from GOPATH/bin
	@echo "$(CYAN)▶$(RESET) Uninstalling $(BOLD)$(BINARY_NAME)$(RESET)..."
	@rm -f $(shell go env GOPATH)/bin/$(BINARY_NAME)
	@echo "$(GREEN)✓$(RESET) Uninstalled"

# ══════════════════════════════════════════════════════════════════════════════
## Utilities
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(CYAN)▶$(RESET) Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@rm -f coverage.out coverage.html
	@echo "$(GREEN)✓$(RESET) Cleaned"

.PHONY: info
info: ## Show build information
	@echo ""
	@echo "$(BOLD)$(CYAN)🔐 go-secrets$(RESET) Build Information"
	@echo "$(CYAN)━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━$(RESET)"
	@echo "  Binary:     $(YELLOW)$(BINARY_NAME)$(RESET)"
	@echo "  Version:    $(YELLOW)$(VERSION)$(RESET)"
	@echo "  Commit:     $(YELLOW)$(COMMIT)$(RESET)"
	@echo "  Build Date: $(YELLOW)$(BUILD_DATE)$(RESET)"
	@echo "  Go Version: $(YELLOW)$(GO_VERSION)$(RESET)"
	@echo "  GOOS:       $(YELLOW)$(shell go env GOOS)$(RESET)"
	@echo "  GOARCH:     $(YELLOW)$(shell go env GOARCH)$(RESET)"
	@echo "  GOPATH:     $(YELLOW)$(shell go env GOPATH)$(RESET)"
	@echo ""

.PHONY: loc
loc: ## Count lines of code
	@echo "$(CYAN)▶$(RESET) Counting lines of code..."
	@echo ""
	@find . -name '*.go' -not -path './vendor/*' | xargs wc -l | tail -1 | awk '{printf "  $(BOLD)Total:$(RESET) $(GREEN)%d$(RESET) lines\n", $$1}'
	@echo ""
	@echo "$(BOLD)By file:$(RESET)"
	@find . -name '*.go' -not -path './vendor/*' | xargs wc -l | sort -n | head -20 | awk '{printf "  %6d  %s\n", $$1, $$2}'

.PHONY: tree
tree: ## Show project structure
	@command -v tree >/dev/null 2>&1 || { \
		echo "$(RED)✗$(RESET) tree not found. Install with: brew install tree"; \
		exit 1; \
	}
	@tree -I 'vendor|dist|build|.git' --dirsfirst -C

# ══════════════════════════════════════════════════════════════════════════════
#  Air configuration (for hot reload)
# ══════════════════════════════════════════════════════════════════════════════

.PHONY: init-air
init-air: ## Initialize air configuration for hot reload
	@echo "$(CYAN)▶$(RESET) Creating .air.toml..."
	@echo 'root = "."' > .air.toml
	@echo 'tmp_dir = "tmp"' >> .air.toml
	@echo '' >> .air.toml
	@echo '[build]' >> .air.toml
	@echo '  bin = "./tmp/main"' >> .air.toml
	@echo '  cmd = "go build -o ./tmp/main ."' >> .air.toml
	@echo '  delay = 1000' >> .air.toml
	@echo '  exclude_dir = ["vendor", "tmp", "dist", "build"]' >> .air.toml
	@echo '  include_ext = ["go"]' >> .air.toml
	@echo '  kill_delay = "2s"' >> .air.toml
	@echo '  stop_on_error = true' >> .air.toml
	@echo '' >> .air.toml
	@echo '[misc]' >> .air.toml
	@echo '  clean_on_exit = true' >> .air.toml
	@echo "$(GREEN)✓$(RESET) Created .air.toml"

