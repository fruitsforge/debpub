.PHONY: all build test clean docker-build docker-test release

BINARY_NAME=debpub
BIN_DIR=bin

BASE_VERSION ?= 0.0.1
GIT_TAG ?= $(shell git describe --tags --exact-match 2>/dev/null)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DIRTY ?= $(shell git diff --quiet 2>/dev/null && echo "" || echo "-dirty")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

ifneq ($(GIT_TAG),)
	VERSION ?= $(GIT_TAG)
else
	VERSION ?= $(BASE_VERSION)+$(GIT_COMMIT)$(DIRTY)
endif

LDFLAGS=-s -w \
	-X debpub/internal/version.Version=$(VERSION) \
	-X debpub/internal/version.GitCommit=$(GIT_COMMIT) \
	-X debpub/internal/version.BuildDate=$(BUILD_DATE)

all: test build

lint:
	golangci-lint run ./cmd/... ./internal/...

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) .

test:
	go test -v -race ./...

docker-build:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t $(BINARY_NAME):latest .

docker-test:
	docker compose -f docker-compose.test.yml up --build --abort-on-container-exit --exit-code-from test-runner
	docker compose -f docker-compose.test.yml down -v

clean:
	rm -rf $(BIN_DIR)

release:
	@./scripts/release.sh $(if $(filter true,$(DRY_RUN)),--dry-run,) $(if $(filter true,$(PUSH)),--push,) $(VERSION)
