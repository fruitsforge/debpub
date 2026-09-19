# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27.1
ARG UBUNTU_VERSION=24.04

# ==========================================
# Stage 1: Build static Go binary
# ==========================================
FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/debpub .

# ==========================================
# Stage 2: Integration test environment
# ==========================================
FROM ubuntu:${UBUNTU_VERSION} AS test-env

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    apt-utils \
    apt-ftparchive \
    dpkg-dev \
    gnupg \
    ca-certificates \
    curl \
    openssh-client \
    jq \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/debpub /usr/local/bin/debpub

WORKDIR /workspace
ENTRYPOINT ["debpub"]

# ==========================================
# Stage 3: Production Runtime Image (Ubuntu 24.04 LTS)
# Includes GPG, SSH/SFTP client, MinIO client (mc), curl, jq, and dpkg
# ==========================================
FROM ubuntu:${UBUNTU_VERSION} AS final

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    gnupg \
    openssh-client \
    dpkg \
    curl \
    jq \
    tzdata \
    && curl -sSL https://dl.min.io/client/mc/release/linux-amd64/mc -o /usr/local/bin/mc 2>/dev/null || true \
    && chmod +x /usr/local/bin/mc 2>/dev/null || true \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/debpub /usr/local/bin/debpub

WORKDIR /workspace

ENTRYPOINT ["debpub"]
CMD ["--help"]
