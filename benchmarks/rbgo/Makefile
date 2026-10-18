# rbgo — an ark component: a command that runs a Ruby script on go-embedded-ruby, for
# cameo's benchmarks.
#
# Every ark component exposes the same entry points, whatever language it is
# written in: prerequisites, deps, build, tests, integration-tests. See
# /docs/component-makefiles.md.
#
# go-embedded-ruby needs Go 1.27.2, newer than the workspace's, so every target
# builds the module on its own: GOWORK=off, and GOTOOLCHAIN=auto fetches the
# toolchain go.mod names.

GO := GOWORK=off GOTOOLCHAIN=auto go

.PHONY: default help prerequisites deps build tests integration-tests lint clean install

default: build
	@echo "Build succeeded. Show help via 'make help'."

help: ## Show this help
	@grep -Eh '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | sort | awk -F ':.*## ' '{printf "  %-18s %s\n", $$1, $$2}'

prerequisites: ## Install the tools needed to build this component
	@command -v go >/dev/null 2>&1 || { \
	  echo "go is not installed. See https://go.dev/dl/." >&2; \
	  exit 1; \
	}
	@echo "rbgo: go is the only tool this component needs"

deps: prerequisites ## Resolve all dependencies
	$(GO) mod download

build: prerequisites ## Build the rbgo binary into bin/
	$(GO) build -o bin/rbgo .

tests: prerequisites ## Run all unit tests (this command has none; it is exercised by the benchmarks)
	$(GO) vet ./...

integration-tests: prerequisites ## Run shell-based integration tests (this part has none)
	@echo "rbgo has no integration tests"

lint: prerequisites ## Run static checks
	$(GO) vet ./...

install: prerequisites ## Install the rbgo binary into GOBIN (or ~/go/bin)
	$(GO) install .

clean: ## Remove build output
	rm -rf bin
