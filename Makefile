# cameo — a small language with Ruby's syntax, run by an interpreter in Go.
#
# The same entry points as every ark component: prerequisites, deps, build,
# tests, integration-tests, lint. `make check` runs them all, as CI does.

BINARY := bin/cameo

.PHONY: default help prerequisites deps build tests integration-tests lint check ruby-spec spec-status clean install proto proto-check


# Every test runs in UTC, the Go tests and the shell tests alike: a test
# that holds cameo to what Ruby prints for a local time must not depend on
# the machine's zone. tests/main_test.go pins the zone for a direct
# `go test ./tests` as well.
export TZ = UTC
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
	@unformatted=$$(gofmt -l src tests examples); if [ -n "$$unformatted" ]; then \
	  echo "not gofmt-clean, run: gofmt -w $$unformatted" >&2; exit 1; fi
	go mod tidy -diff
	go test ./...
	cd examples/grpc && go mod tidy -diff && go vet ./... && go test ./...

# examples/grpc is a module of its own, so that cameo's go.mod does not
# require gRPC; the tests target runs its tests from inside it.

# The shell tests drive the built binary, which reaches them through CAMEO_BIN.
integration-tests: prerequisites build ## Run the shell-based integration tests
	@for t in tests/integration/test_*.sh; do \
	  echo "==> $$t"; CAMEO_BIN=$(CURDIR)/$(BINARY) bash $$t || exit 1; \
	done

lint: prerequisites ## Run static checks
	go vet ./...

check: lint tests integration-tests ## Everything CI runs

# Not part of check: it needs a Ruby and a ruby/spec copy.
spec-status: build ## Run ruby/spec's language and core, and rewrite specs/status.tsv
	ruby specs/run.rb --cameo $(BINARY) --ruby ruby --status specs/status.tsv

# Not part of check either: it clones or pulls github.com/ruby/spec.
ruby-spec: ## Clone ruby/spec into ruby-spec/ of the main repository, or pull it
	ruby specs/checkout.rb

# The messages of src/cameopb are written in .proto files and their Go code
# is generated and committed, so building cameo needs Go alone. Changing a
# .proto needs this target, which fetches the protoc release it pins;
# protoc-gen-go is built at the version of protobuf-go in go.mod.
PROTOC_VERSION := 33.0
PROTOC_OS := $(if $(filter Darwin,$(shell uname -s)),osx,linux)
PROTOC_ARCH := $(if $(filter arm64 aarch64,$(shell uname -m)),aarch_64,x86_64)
PROTOC_DIR := bin/protoc-$(PROTOC_VERSION)
PROTOC := $(PROTOC_DIR)/bin/protoc
PROTO_FILES := $(wildcard src/cameopb/cameo/*.proto)

$(PROTOC):
	@command -v curl >/dev/null 2>&1 && command -v unzip >/dev/null 2>&1 || { \
	  echo "make proto needs curl and unzip to fetch protoc $(PROTOC_VERSION)." >&2; exit 1; }
	mkdir -p $(PROTOC_DIR)
	curl -sSfL -o $(PROTOC_DIR)/protoc.zip \
	  https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_VERSION)-$(PROTOC_OS)-$(PROTOC_ARCH).zip
	unzip -q -o $(PROTOC_DIR)/protoc.zip -d $(PROTOC_DIR)
	rm $(PROTOC_DIR)/protoc.zip

proto: prerequisites $(PROTOC) ## Generate the Go code of the messages in src/cameopb and examples/ex1 from their .proto files
	go build -o bin/protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go
	$(PROTOC) --plugin=protoc-gen-go=bin/protoc-gen-go -I src/cameopb \
	  --go_out=src/cameopb --go_opt=module=github.com/cameo-lang/cameo/src/cameopb \
	  $(patsubst src/cameopb/%,%,$(PROTO_FILES))
	$(PROTOC) --plugin=protoc-gen-go=bin/protoc-gen-go -I examples/ex1 \
	  --go_out=examples/ex1/greetpb --go_opt=paths=source_relative greet.proto

# Not part of check, which needs no tool but Go; CI runs it as a step.
proto-check: proto ## Fail when the committed Go code of src/cameopb or examples/ex1 differs from what make proto generates
	@git diff --exit-code -- src/cameopb examples/ex1/greetpb || { \
	  echo "src/cameopb or examples/ex1/greetpb is not what its .proto files generate: run make proto and commit the result." >&2; exit 1; }
	@untracked=$$(git ls-files --others --exclude-standard -- src/cameopb examples/ex1/greetpb); if [ -n "$$untracked" ]; then \
	  echo "make proto wrote files that are not committed: $$untracked" >&2; exit 1; fi

install: prerequisites ## Install the cameo binary into GOBIN (or ~/go/bin)
	go install ./src/cmd/cameo

clean: ## Remove build output
	rm -rf bin
