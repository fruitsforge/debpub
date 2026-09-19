.PHONY: all build test clean docker-build docker-test

BINARY_NAME=debpub
BIN_DIR=bin

all: test build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) .

test:
	go test -v -race ./...

docker-build:
	docker build -t $(BINARY_NAME):latest .

docker-test:
	docker compose -f docker-compose.test.yml up --build --abort-on-container-exit test-runner
	docker compose -f docker-compose.test.yml down

clean:
	rm -rf $(BIN_DIR)
