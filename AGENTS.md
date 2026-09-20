# `debpub` AI Guidelines & Architecture Reference

Welcome to `debpub`. This document provides essential context, architectural patterns, and quality guidelines for AI agents and human contributors working on this repository.

---

## 1. Project Overview & Architecture

`debpub` is a modern, high-performance, stateless Debian repository packaging and publishing tool written in **Go 1.27+**.

Key architectural pillars:
- **Stateless Repository**: The remote `Packages` manifest on S3, SFTP, or local disk is the sole source of truth. No LevelDB, SQLite, or database tarballs.
- **Distributed Locking (`--lock`)**: Safe concurrent publishing across ephemeral CI/CD runners using atomic lock acquisition (`If-None-Match: "*"` on S3, atomic POSIX rename on SFTP/filesystem) with configurable TTL and stale lock recovery.
- **Multi-Stream Compression**: Concurrent on-the-fly generation of `Packages`, `Packages.gz`, `Packages.bz2`, and `Packages.xz`.
- **Single Static Binary**: Built with `CGO_ENABLED=0`, producing a minimal standalone binary (< 20MB) with zero runtime dependencies.

### Directory Layout
```text
debpub/
├── cmd/debpub/         # Cobra CLI commands, flag bindings, debpub.json configuration
├── internal/
│   ├── compress/       # Streaming compression engines (gzip, bzip2, xz)
│   ├── config/         # Config domain models and JSON loader
│   ├── debian/         # Debian control/deb822 parser, version ordering, index & release builder
│   ├── lock/           # Distributed locking manager, atomic operations, stale recovery
│   ├── repo/           # Staged publishing workflow engine
│   ├── storage/        # Storage abstraction & implementations (AWS S3, SFTP, Local filesystem)
│   ├── testutil/       # Test helper utilities (e.g. synthetic deb generation)
│   └── version/        # Build metadata & semantic versioning
├── test/integration/   # End-to-end integration test runner against MinIO & SFTP
├── Dockerfile          # Multi-stage build (builder, test-env, Ubuntu 26.04 final runtime)
├── Makefile            # Standard developer targets (build, test, lint, docker-build)
└── .agents/            # Project AI rules and skills
```

---

## 2. Active Development Rules

When contributing or generating code, AI agents must strictly follow the rules in `.agents/rules/`:
- [rules.md](.agents/rules/rules.md): Global principles (Do No Harm, Documentation Integrity, Conventional Commits).
- [rules-go.md](.agents/rules/rules-go.md): Go idiomatic standards (Go 1.22+, `log/slog`, table-driven tests, `%w` error wrapping, context propagation).
- [rules-shell.md](.agents/rules/rules-shell.md): Shell script safety (`set -euo pipefail`), cross-platform macOS/Linux portability, ShellCheck adherence.
- [rules-debian.md](.agents/rules/rules-debian.md): Debian packaging standards (RFC 822/deb822, pool/dists layout, GPG signing, version comparison).

---

## 3. Active Project Skills

Domain-specific procedural runbooks are located in `.agents/skills/`:
- `go-developer`: Senior-grade Go patterns, memory efficiency, and testing.
- `debian-packaging`: Debian repository generation, control fields, and archive hierarchy.
- `docker`: Multi-stage builds, non-root user, minimal attack surface.
- `shell-developer`: Portable bash & POSIX automation.
- Curated Go engineering skills: `golang-cli`, `golang-concurrency`, `golang-context`, `golang-error-handling`, `golang-testing`, `golang-benchmark`.
