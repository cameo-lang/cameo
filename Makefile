# cameo — a small language with Ruby's syntax, run by an interpreter in Go.
#
# The same entry points as every ark component: prerequisites, deps, build,
# tests, integration-tests, lint. `make check` runs them all, as CI does.

BINARY := bin/cameo

.PHONY: default help prerequisites deps build tests integration-tests lint check conformance clean install

default: build
	@echo "Build succeeded. Run tests via 'make tests'. Show help via 'make help'."

help: ## Show this help
	@grep -Eh '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | sort | awk -F ':.*## ' '{printf "  %-18s %s\n", $$1, $$2}'

prerequisites: ## Install the tools needed to build cameo
	@command -v go >/dev/null 2>&1 || { \
	  echo "go is not installed. See https://go.dev/dl/ (cameo builds with the version in go.mod)." >&2; \
	  exit 1; \
	}
	@echo "cameo: go $$(go env GOVERSION) is the only tool cameo needs"

deps: prerequisites ## Resolve all dependencies
	go mod download

# No staleness check of our own: the Go toolchain knows the import graph and
# caches on it, and a no-op rebuild costs a fraction of a second.
build: prerequisites ## Build the cameo binary into bin/
	go build -o $(BINARY) ./src/cmd/cameo

tests: prerequisites ## Run the Go tests, and check formatting and go.mod
	@unformatted=$$(gofmt -l src tests); if [ -n "$$unformatted" ]; then \
	  echo "not gofmt-clean, run: gofmt -w $$unformatted" >&2; exit 1; fi
	go mod tidy -diff
	go test ./...

# The shell tests drive the built binary, which reaches them through CAMEO_BIN.
integration-tests: prerequisites build ## Run the shell-based integration tests
	@for t in tests/integration/test_*.sh; do \
	  echo "==> $$t"; CAMEO_BIN=$(CURDIR)/$(BINARY) bash $$t || exit 1; \
	done

lint: prerequisites ## Run static checks
	go vet ./...

check: lint tests integration-tests ## Everything CI runs

# Not part of check, which needs no tool but Go: this one needs a Ruby.
conformance: prerequisites ## Run every example and CRuby case compiled and interpreted, and compare
	ruby tests/conformance/run.rb

# Not part of check either: it clones or pulls github.com/ruby/spec.
ruby-spec: ## Clone ruby/spec into ruby-spec/ of the main repository, or pull it
	ruby specs/checkout.rb

install: prerequisites ## Install the cameo binary into GOBIN (or ~/go/bin)
	go install ./src/cmd/cameo

clean: ## Remove build output
	rm -rf bin
