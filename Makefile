.PHONY: all build test clean docker-build docker-test

BINARY_NAME=debpub
BIN_DIR=bin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "1.0.0-dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS=-s -w \
	-X debpub/internal/version.Version=$(VERSION) \
	-X debpub/internal/version.GitCommit=$(GIT_COMMIT) \
	-X debpub/internal/version.BuildDate=$(BUILD_DATE)

all: test build

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
	docker compose -f docker-compose.test.yml up --build --abort-on-container-exit test-runner
	docker compose -f docker-compose.test.yml down

clean:
	rm -rf $(BIN_DIR)
