# warmruby: a command that runs a Ruby script on CRuby 3.4 compiled to
# WebAssembly, on wazero, for cameo's benchmarks. A module of its own, so
# wazero stays out of cameo's go.mod.

GO := GOWORK=off go

.PHONY: default help prerequisites deps build tests lint clean install

default: build
	@echo "Build succeeded. Show help via 'make help'."

help: ## Show this help
	@grep -Eh '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | sort | awk -F ':.*## ' '{printf "  %-18s %s\n", $$1, $$2}'

prerequisites: ## Install the tools needed to build this command
	@command -v go >/dev/null 2>&1 || { \
	  echo "go is not installed. See https://go.dev/dl/." >&2; \
	  exit 1; \
	}

deps: prerequisites ## Resolve all dependencies
	$(GO) mod download

build: prerequisites ## Build the warmruby binary into bin/
	$(GO) build -o bin/warmruby .

tests: lint ## Static checks; the command is exercised by the benchmarks

lint: prerequisites ## Run static checks
	@unformatted=$$(gofmt -l .); if [ -n "$$unformatted" ]; then \
	  echo "not gofmt-clean, run: gofmt -w $$unformatted" >&2; exit 1; fi
	$(GO) vet ./...

install: prerequisites ## Install the warmruby binary into GOBIN (or ~/go/bin)
	$(GO) install .

clean: ## Remove build output; the module cache stays
	rm -rf bin
