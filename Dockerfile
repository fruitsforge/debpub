# syntax=docker/dockerfile:1

ARG GO_VERSION=1.27.1
ARG UBUNTU_VERSION=26.04

# ==========================================
# Stage 1: Build static Go binary
# ==========================================
FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=1.0.0-dev
ARG GIT_COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X debpub/internal/version.Version=${VERSION} -X debpub/internal/version.GitCommit=${GIT_COMMIT} -X debpub/internal/version.BuildDate=${BUILD_DATE}" \
    -o /bin/debpub .

# ==========================================
# Stage 2: Integration test environment
# ==========================================
FROM ubuntu:${UBUNTU_VERSION} AS test-env

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y --no-install-recommends \
    apt-utils \
    dpkg-dev \
    gnupg \
    ca-certificates \
    curl \
    openssh-client \
    jq \
    git \
    && rm -rf /var/lib/apt/lists/*

# Copy Go toolchain from builder stage
COPY --from=builder /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"

COPY --from=builder /bin/debpub /usr/local/bin/debpub

WORKDIR /workspace
ENTRYPOINT ["debpub"]

# ==========================================
# Stage 3: Production Runtime Image (Ubuntu 26.04 LTS)
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
