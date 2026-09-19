# Stage 1: Build static Go binary
FROM golang:1.24-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/debpub .

# Stage 2: Integration test environment with genuine Debian tools
FROM debian:bookworm-slim AS test-env

RUN apt-get update && apt-get install -y --no-install-recommends \
    apt-utils \
    apt-ftparchive \
    dpkg-dev \
    gnupg \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/debpub /usr/local/bin/debpub

WORKDIR /workspace
ENTRYPOINT ["debpub"]

# Stage 3: Minimal runtime image (< 20MB)
FROM alpine:3.21 AS final

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/debpub /usr/local/bin/debpub

ENTRYPOINT ["debpub"]
CMD ["--help"]
