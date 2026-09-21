# debpub

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.27%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![AI-assisted](https://img.shields.io/badge/AI--assisted-pair--programming-8E75B2?logo=google&logoColor=white)](https://github.com/fruitsforge/debpub#authors--development)

> **Modern, stateless Debian repository packaging and publishing tool written in Go.**  
> Built for cloud-native CI/CD, ephemeral runners, and high-concurrency environments with distributed locking and full compression support (`gz`, `bz2`, `xz`).

---

## Why `debpub`?

Debian repository management in modern CI/CD pipelines (such as **Azure DevOps Pipelines**, **GitHub Actions**, **GitLab CI**, and **AWS CodeBuild**) has historically required compromising between two paradigms:

1. **The Heavy / Database Dilemma (Aptly)**:  
   Full-featured tools like Aptly rely on an internal embedded database (LevelDB). In ephemeral multi-runner CI/CD environments, this requires either maintaining a 24/7 centralized API server or syncing/locking fragile database tarballs (`aptly-db.tar.gz`) across independent builds. If multiple runners deploy simultaneously, they can clobber each other's state.
2. **The Dependency & Compression Dilemma (`deb-s3`)**:  
   The Ruby `deb-s3` gem pioneered the elegant **stateless index** architecture, but it requires installing Ruby, bundler, and native gem dependencies inside every CI container. Furthermore, it lacks native `Packages.bz2` or `.xz` compression without manual patching, and has fallen out of active maintenance.

### The `debpub` Solution: Stateless & Cloud Native

`debpub` combines the **stateless index-driven architecture** of `deb-s3` with the **speed, reliability, and single static binary distribution** of Go:

- 🚀 **Stateless Index Architecture**: The Debian repository's `Packages` manifest in S3, SFTP, or on disk **is** the database. No LevelDB, SQLite, or database tarballs to backup or restore.
- 🖥️ **Embedded Repository Browser & REST API (`debpub serve`)**: Built-in zero-dependency web server and interactive UI to browse packages, inspect metadata, filter versions, and synchronize indexes with automatic codename/component discovery.
- 🔒 **Distributed Locking (`--lock`)**: Safe concurrent writes from multiple independent CI/CD runners using atomic `.lock` metadata with configurable TTL, automatic stale lock recovery, and S3 conditional writes (`If-None-Match: "*"`).
- 📦 **Modern Compression**: Out-of-the-box generation and decompression of `Packages`, `Packages.gz`, **`Packages.bz2`**, and `Packages.xz`.
- ⚡ **Zero Runtime Dependencies**: A single self-contained static binary (< 20 MB). Runs in `scratch`, `alpine`, or minimal CI containers without Ruby, Python, or external packages.
- 🌐 **Multi-Storage Protocols**: First-class support for **AWS S3** (and S3-compatible endpoints like MinIO, Ceph, Cloudflare R2), **SFTP** (SSH file transfer for Linux servers and mirrors), and **Local Filesystem** (`file://`).

---

## Installation & Distribution

### Standalone Binary
Download the precompiled static binary for your architecture from [GitHub Releases](https://github.com/fruitsforge/debpub/releases):

```bash
# Example for Linux amd64
curl -sSL -o /usr/local/bin/debpub https://github.com/fruitsforge/debpub/releases/latest/download/debpub_0.0.3_linux_amd64.tar.gz
chmod +x /usr/local/bin/debpub
```

### Docker
Run `debpub` directly inside any container pipeline:

```bash
docker run --rm -v $(pwd):/workspace ghcr.io/fruitsforge/debpub:latest publish --config debpub.json ./package.deb
```

---

## Usage Guide

`debpub` can be configured entirely via command-line flags, or via a declarative JSON configuration file (`debpub.json`).

### 1. Publishing to AWS S3 (or S3-Compatible Storage)

Publish a Debian package to an S3 bucket with distributed locking:

```bash
debpub publish \
  --storage s3 \
  --bucket my-debian-repo \
  --prefix debian \
  --codename stable \
  --component main \
  ./mypackage_1.0.0_amd64.deb
```

#### MinIO / Ceph / Cloudflare R2:
```bash
debpub publish \
  --storage s3 \
  --bucket my-repo \
  --s3-endpoint http://minio.local:9000 \
  --s3-force-path-style \
  --codename bookworm \
  ./mypackage_1.0.0_amd64.deb
```

#### Named AWS Profile & Specific Region:
```bash
debpub publish \
  --storage s3 \
  --bucket my-debian-repo \
  --s3-profile production \
  --s3-region eu-central-1 \
  --codename bookworm \
  ./mypackage_1.0.0_amd64.deb
```

### 2. Publishing to Local Filesystem or HTTP Server Directory

Ideal for local testing, bind mounts, or static web servers (Nginx/Apache):

```bash
debpub publish \
  --storage file \
  --dir /var/www/repos/apt \
  --codename stable \
  --component main \
  ./mypackage_1.0.0_amd64.deb
```

### 3. Publishing to Remote Server via SFTP

For publishing directly to remote Linux servers, appliances, or mirrors over SSH:

```bash
debpub publish \
  --storage sftp \
  --sftp-host repo.internal.net \
  --sftp-user deployer \
  --sftp-key ~/.ssh/id_ed25519 \
  --prefix /var/www/debian \
  --codename stable \
  ./mypackage_1.0.0_amd64.deb
```

### 4. GPG Manifest Signing (Dual Manifest: `InRelease` + `Release.gpg`)

To automatically sign your repository manifest with your GPG private key:

```bash
debpub publish \
  --storage s3 \
  --bucket my-debian-repo \
  --codename stable \
  --sign \
  --gpg-key "support@example.com" \
  --gpg-passphrase "mysecretkeypassphrase" \
  ./mypackage_1.0.0_amd64.deb
```

### 5. Publishing Multiple Files, Globs, or Entire Directories

`debpub` natively supports batch publishing in a single atomic transaction. You can supply multiple files, quoted or unquoted glob patterns, and directories:

#### Multiple Explicit Packages:
```bash
debpub publish \
  --storage s3 \
  --bucket my-debian-repo \
  --codename stable \
  ./pkg1_1.0.0_amd64.deb ./pkg2_2.1.0_amd64.deb ./pkg3_0.5.0_all.deb
```

#### Wildcard / Glob Patterns:
```bash
# Quoted patterns are expanded by debpub directly (ideal for CI/CD matrices or Windows):
debpub publish \
  --storage s3 \
  --bucket my-debian-repo \
  --codename stable \
  "./dist/*.deb"
```

#### Recursive Directory Scanning:
```bash
# Recursively discovers all .deb, .udeb, and .ddeb packages in the directory tree:
debpub publish \
  --storage s3 \
  --bucket my-debian-repo \
  --codename stable \
  ./build/output/
```

> [!TIP]
> **Why Batch Publishing is Superior in CI/CD**:
> Publishing 10 packages in a single batch operation acquires the repository lock **once**, generates and compresses index files (`Packages.gz`, `Packages.xz`, etc.) **once**, and writes an atomic `Release` manifest. This eliminates locking contention between concurrent runner jobs and saves up to 90% of S3 API PUT calls compared to running single-package upload commands in a loop.

### 6. Browsing Repository Packages via Web UI & REST API (`debpub serve`)

`debpub` includes a built-in zero-dependency HTTP/HTTPS server and single-page web UI to browse repositories, inspect package details (control fields, architectures, dependencies, SHA-256 hashes), and dynamically sync index files.

#### Local Directory:
```bash
debpub serve --storage file --dir /var/www/repos/apt --server-port 8080
```

#### AWS S3 Repository (with automatic codename & component auto-discovery):
```bash
debpub serve \
  --storage s3 \
  --bucket repo-deb.dev.example.com \
  --s3-profile dev \
  --server-port 8090
```

#### Remote SFTP Repository:
```bash
debpub serve \
  --storage sftp \
  --sftp-host repo.example.com \
  --sftp-user debian \
  --sftp-key ~/.ssh/id_ed25519 \
  --prefix /var/www/debian \
  --server-port 8080
```

#### HTTPS with Custom TLS Certificates:
```bash
debpub serve \
  --storage s3 \
  --bucket my-debian-repo \
  --server-port 8443 \
  --server-tls-cert /path/to/cert.pem \
  --server-tls-key /path/to/key.pem
```

Open **`http://localhost:8080/`** (or your configured port) in your browser.

#### REST API Endpoints:
The daemon exposes clean REST endpoints:
- `GET /api/info?codename=<name>&component=<comp>`: Returns repository metadata, active targets, and total package/version counts.
- `GET /api/packages?codename=<name>&component=<comp>&arch=<arch>&q=<search>`: Lists package summary cards with all available versions.
- `GET /api/packages/{name}?codename=<name>&component=<comp>&version=<ver>`: Returns complete Debian control paragraph for a package.
- `POST /api/fetch?codename=<name>&component=<comp>`: Triggers an in-memory re-sync and auto-discovery of remote indexes.

---

## Configuration File (`debpub.json`)

To avoid repeating flags across CI/CD pipeline steps, define a `debpub.json` file and pass `--config`:

```json
{
  "storage": "s3",
  "bucket": "my-debian-repo",
  "prefix": "apt",
  "codename": "stable",
  "component": "main",
  "architectures": ["amd64", "arm64"],
  "preserve_versions": true,
  "origin": "MyCompany",
  "label": "MyCompany Packages",
  "suite": "stable",
  "description": "Production Debian Repository",
  "gpg": {
    "sign": true,
    "key": "support@example.com"
  },
  "lock": {
    "enabled": true,
    "timeout": "2m",
    "ttl": "3m"
  },
  "server": {
    "port": 8080,
    "bind": "0.0.0.0",
    "tls_cert": "",
    "tls_key": ""
  },
  "s3": {
    "endpoint": "",
    "force_path_style": false,
    "legacy_locking": false,
    "profile": "production",
    "region": "eu-central-1"
  }
}
```

Run with configuration:
```bash
debpub publish --config debpub.json ./mypackage_1.0.0_amd64.deb
```

### Option Precedence & Smart Defaults

`debpub` strictly enforces a layered configuration priority with built-in smart defaults and dual format flexibility:
1. **Command-line flags** (highest priority; explicitly overrides any setting).
2. **Config file** (values from `debpub.json` via `--config`).
3. **Environment variables** (e.g., `AWS_REGION`, `AWS_PROFILE`, `USER`).
4. **Built-in defaults** (e.g. `--lock-timeout 2m`, `--lock-ttl 3m`, `--component main`, `--dir .`).

#### Smart Auto-Detection & Fallbacks
- **Suite fallback**: If `--suite` is omitted, it automatically defaults to the distribution `--codename`.
- **GPG Signing**: Supplying `--gpg-key` automatically enables Release manifest signing (`--sign`) without requiring an explicit `-s` flag.
- **SFTP User**: If `--sftp-user` is omitted, it automatically defaults to the invoking system user (`$USER`).
- **Flexible Time Formats**: `--lock-timeout` and `--lock-ttl` accept both unit-based duration strings (e.g. `2m`, `120s`, `3m`, `180s`) and raw integer seconds without units (e.g. `120`, `180`).
- **Auditable Logging**: When defaults or smart fallbacks are applied, `debpub` logs them to standard output for complete transparency in CI/CD pipelines.

---

## Command Reference

### `debpub publish [flags] <file.deb|pattern|directory...>`

| Flag | Shorthand | Description | Default |
| :--- | :--- | :--- | :--- |
| `--config` | | Path to `debpub.json` configuration file | |
| `--storage` | | Storage backend (`s3`, `sftp`, `file`) | `file` |
| `--codename` | `-c` | Debian distribution codename (`stable`, `bookworm`, `jammy`) | *(Required)* |
| `--component` | `-m` | Debian repository component (`main`, `contrib`, `non-free`) | `main` |
| `--dir` | | Local repository root directory (for `file` storage) | `.` |
| `--bucket` | `-b` | S3 bucket name (for `s3` storage) | *(Required for S3)* |
| `--prefix` | | S3 or remote directory path prefix | `""` |
| `--s3-endpoint` | | Custom S3 endpoint URL (MinIO, Ceph, R2) | |
| `--s3-force-path-style` | | Use path-style S3 URLs (required for MinIO) | `false` |
| `--s3-legacy-locking` | | Use check-then-put locking instead of S3 conditional writes | `false` |
| `--s3-profile` | | AWS profile name to use for credentials and configuration | |
| `--s3-region` | | AWS region for S3 bucket (e.g. `us-east-1`, `eu-central-1`) | |
| `--sftp-host` | | SFTP server hostname / IP | *(Required for SFTP)* |
| `--sftp-port` | | SFTP port | `22` |
| `--sftp-user` | | SFTP username | Current OS `$USER` |
| `--sftp-password` | | SFTP password | |
| `--sftp-key` | | SFTP SSH private key file path | |
| `--preserve-versions` | | Keep older package versions in index rather than replacing | `true` |
| `--lock` | | Enable distributed repository locking | `true` |
| `--lock-timeout` | | Max wait time for lock (accepts `2m`, `120s`, or `120`) | `2m` (120s) |
| `--lock-ttl` | | Lock expiration TTL before being marked stale (accepts `3m`, `180s`, or `180`) | `3m` (180s) |
| `--sign` | `-s` | Enable GPG signing of `Release` manifest | `false` (auto if `--gpg-key` set) |
| `--gpg-key` | `-k` | GPG signing key ID, email, or fingerprint | |
| `--gpg-passphrase` | | GPG key passphrase | |
| `--origin` | | Custom repository `Origin` header | |
| `--label` | | Custom repository `Label` header | |
| `--suite` | | Custom repository `Suite` header | Defaults to `--codename` |
| `--description` | | Repository `Description` header | |

### `debpub serve [flags]`

Starts an embedded HTTP/HTTPS server with REST API endpoints and an interactive single-page web UI to browse packages, inspect metadata, filter versions, and sync indexes.

| Flag | Shorthand | Description | Default |
| :--- | :--- | :--- | :--- |
| `--server-port` | | HTTP/HTTPS server listening port | `8080` |
| `--server-bind` | | HTTP/HTTPS server bind IP address or hostname | `0.0.0.0` |
| `--server-tls-cert` | | Path to TLS certificate file for HTTPS | `""` |
| `--server-tls-key` | | Path to TLS private key file for HTTPS | `""` |
| `--config` | | Path to `debpub.json` configuration file | |
| `--storage` | | Storage backend (`s3`, `sftp`, `file`) | `file` |
| `--codename` | `-c` | Distribution codename (auto-discovered if omitted) | `""` |
| `--component` | `-m` | Repository component (auto-discovered from `Release` or `dists/`) | `main` |
| `--dir` | | Local repository root directory (for `file` storage) | `.` |
| `--bucket` | `-b` | S3 bucket name (for `s3` storage) | |
| `--prefix` | | S3 or remote directory path prefix | `""` |
| `--s3-profile` | | AWS profile name for S3 credentials | |
| `--s3-region` | | AWS region for S3 bucket | |
| `--s3-endpoint` | | Custom S3 endpoint URL (MinIO, Ceph, R2) | |
| `--s3-force-path-style` | | Use path-style S3 URLs | `false` |
| `--sftp-host` | | SFTP server hostname / IP | |
| `--sftp-port` | | SFTP port | `22` |
| `--sftp-user` | | SFTP username | Current OS `$USER` |
| `--sftp-key` | | SFTP SSH private key file path | |
| `--sftp-password` | | SFTP password | |

---

## Contributing & Developer Guide

If you are interested in building, testing, or contributing to `debpub`, please see **[DEVELOPMENT.md](DEVELOPMENT.md)** for instructions on local development, running tests with race detection, and our Docker Compose test harness with MinIO and SFTP.

---

## Prior Art & Acknowledgments

`debpub` stands on the shoulders of giants in the Debian packaging ecosystem:

- **[deb-s3](https://github.com/deb-s3/deb-s3)**: Designed by Keith Rarick, morph027, and community contributors (MIT License). `debpub` adopts `deb-s3`'s stateless index concept and distributed lock semantics.
- **[aptly](https://github.com/aptly-dev/aptly)**: Designed by Andrey Smirnov and contributors (MIT License). `debpub` is informed by Aptly's comprehensive Debian manifest generation and compression standards.

---

## Authors & Development

- **Architecture & Maintainer**: Vladimir & Contributors
- **Development**: Architected and developed with pair-programming assistance from Advanced AI Coding Agents (Google Antigravity / Gemini).

---

## License

`debpub` is licensed under the **Apache License, Version 2.0**. See [LICENSE](LICENSE) for details.
