# Developer & Contributor Guide (`DEVELOPMENT.md`)

This guide is intended for developers, maintainers, and contributors working on the `debpub` codebase.

---

## 1. Prerequisites

- **Go**: Version `1.27+`.
- **Docker & Docker Compose**: For hermetic builds and automated integration testing with MinIO and SFTP.
- **Make**: For running developer workflow targets.
- **Git**: For version control.

---

## 2. Project Architecture & Layout

The project follows the standard Go project layout and clean architecture principles:

```text
debpub/
├── cmd/
│   └── debpub/             # CLI commands, Cobra tree, and flag bindings
│       ├── root.go         # Root command, global flags, and debpub.json precedence resolution
│       ├── publish.go      # publish subcommand orchestrator
│       └── main.go         # Thin entrypoint invoking cmd.Execute()
├── internal/
│   ├── compress/           # Pure Go streaming compression engines (gzip, bzip2, xz)
│   ├── config/             # Configuration domain models and debpub.json loader
│   ├── debian/             # Pure Debian format models, deb822 streaming parser, version ordering
│   ├── lock/               # Distributed locking protocol, TTL manager, stale recovery
│   ├── repo/               # 4-phase staged publishing workflow engine
│   └── storage/            # Unified storage backends (AWS S3, SFTP, Local filesystem)
├── Dockerfile              # Multi-stage build (builder, test-env, final minimal image)
├── docker-compose.test.yml # Ephemeral integration test harness (MinIO + SFTP + test-runner)
├── Makefile                # Standard developer targets
├── DEVELOPMENT.md          # This developer guide
├── README.md               # User-facing guide and CLI documentation
└── LICENSE                 # Apache 2.0 License
```

---

## 3. Building `debpub`

### Native Local Build
Compile the static binary locally:

```bash
make build
```
The compiled binary will be placed at `bin/debpub`.

Verify the build:
```bash
./bin/debpub --help
```

### Hermetic Docker Build
Build the container image using the multi-stage `Dockerfile`:

```bash
make docker-build
```
This produces a minimal `< 20MB` container image (`debpub:latest`) containing only the statically linked binary and CA certificates.

---

## 4. Running Tests

### Unit Tests with Race Detector
All packages must pass tests with Go's race detector enabled (`-race`):

```bash
make test
```
Or directly:
```bash
go test -v -race ./...
```

### Testing Individual Packages
```bash
# Test compression engines (roundtrip verification for gz, bz2, and xz)
go test -v ./internal/compress

# Test Debian domain logic (deb822 parsing, version comparison, index serialization)
go test -v ./internal/debian

# Test distributed locking (acquisition, timeouts, stale recovery, concurrent workers)
go test -v ./internal/lock

# Test publisher end-to-end (staged pool and index upload sequence)
go test -v ./internal/repo

# Test storage backends
go test -v ./internal/storage
```

---

## 5. Integration Testing with MinIO and SFTP

To test distributed locking, S3 conditional writes (`If-None-Match: "*"`), and SFTP atomic file operations without relying on live cloud infrastructure, an ephemeral Docker Compose test environment is provided:

```bash
make docker-test
```

This command:
1. Builds the `test-env` stage in `Dockerfile` (containing Debian utilities like `dpkg`, `apt`, `apt-ftparchive`, `gpg`).
2. Starts a live local **MinIO** instance on port `9000` with test credentials.
3. Starts a live local **SFTP server** on port `2222`.
4. Runs integration test routines against the live backends.
5. Cleans up all test containers on completion.

---

## 6. Coding Standards & Guidelines

Before submitting PRs, ensure code conforms to the project conventions:

1. **Idiomatic Go**:
   - Structured logging via standard `log/slog`.
   - Error wrapping using `fmt.Errorf("...: %w", err)`.
   - Table-driven unit tests.
   - Context propagation (`ctx context.Context` as the first argument in I/O operations).
2. **Git History Preservation**:
   - Always use `git mv`, `git cp`, or `git rm` when moving or reorganizing files to maintain linear commit history.
3. **Zero Database State**:
   - `debpub` must remain 100% stateless. The Debian repository's `Packages` manifest is the sole canonical database. Do not introduce local LevelDB/SQLite databases.
4. **Anti-Corruption Protocol**:
   - Always preserve the 4-phase staged upload sequence in `internal/repo/publisher.go` (Payloads -> Indices -> Manifests -> Unlock).
