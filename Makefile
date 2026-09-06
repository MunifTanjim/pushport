VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
BIN     := bin
PREFIX  ?= $(HOME)/.local
BINDIR  ?= $(PREFIX)/bin

.PHONY: build console generate test vet fmt fmt-check tidy check install uninstall clean help

build: ## Build the API binary into bin/
	go build $(LDFLAGS) -o $(BIN)/pushport .

console: ## Build the admin console into console/dist (deployed separately to Cloudflare Pages)
	pnpm install && pnpm console:build

generate: ## Regenerate sqlc code from queries + migrations
	go tool sqlc generate

test: ## Run all tests
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format the tree
	gofmt -w .

fmt-check: ## Fail if any file is not gofmt-clean
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi

tidy: ## Tidy go.mod/go.sum
	go mod tidy

check: fmt-check vet test ## Run fmt-check, vet, and tests

install: build ## Install the binary into BINDIR (default ~/.local/bin)
	install -d $(BINDIR)
	install -m 0755 $(BIN)/pushport $(BINDIR)/pushport

uninstall: ## Remove the installed binary from BINDIR
	rm -f $(BINDIR)/pushport

clean: ## Remove build artifacts
	rm -rf $(BIN) console/dist

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
