BINARY   := spin
MODULE   := github.com/attamusc/spin
BUILD_DIR := dist

# Go toolchain
GO       := go
GOFLAGS  ?=
LDFLAGS  ?=

# Default install location
PREFIX   ?= /usr/local

.PHONY: all build install uninstall test test-verbose test-race lint vet vuln fmt clean tidy run help

## —— Build ——————————————————————————————————————————————

all: build ## Build the binary (default)

build: ## Compile the binary into ./dist/
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY) .

install: build ## Install the binary to $(PREFIX)/bin
	install -d $(PREFIX)/bin
	install -m 755 $(BUILD_DIR)/$(BINARY) $(PREFIX)/bin/$(BINARY)

uninstall: ## Remove the installed binary
	rm -f $(PREFIX)/bin/$(BINARY)

## —— Development ————————————————————————————————————————

run: ## Build and run (pass ARGS="dev" etc.)
	$(GO) run . $(ARGS)

fmt: ## Format all Go source files
	$(GO) fmt ./...

vet: ## Run go vet
	$(GO) vet ./...

lint: vet ## Run vet and pinned staticcheck (requires Go 1.26 toolchain)
	$(GO) run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...

vuln: ## Check reachable vulnerabilities using pinned govulncheck
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

## —— Testing ———————————————————————————————————————————

test: ## Run all tests
	$(GO) test ./... -count=1

test-verbose: ## Run all tests with verbose output
	$(GO) test ./... -count=1 -v

test-race: ## Run all tests with the race detector
	$(GO) test ./... -count=1 -race

## —— Maintenance ———————————————————————————————————————

tidy: ## Tidy and verify go.mod / go.sum
	$(GO) mod tidy
	$(GO) mod verify

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)
	$(GO) clean -cache -testcache

## —— Help ——————————————————————————————————————————————

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
