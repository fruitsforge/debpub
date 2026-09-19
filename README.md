# debpub

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/debpub/debpub)](https://goreportcard.com/report/github.com/debpub/debpub)

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

- 🚀 **Stateless Index Architecture**: The Debian repository's `Packages` manifest in S3 or on disk **is** the database. No LevelDB, SQLite, or database tarballs to backup or restore.
- 🔒 **Distributed Locking (`--lock`)**: Safe concurrent writes from multiple independent CI/CD runners using atomic `.lock` metadata with configurable TTL, automatic stale lock recovery, and S3 conditional writes (`If-None-Match: "*"`).
- 📦 **Modern Compression**: Out-of-the-box generation of `Packages`, `Packages.gz`, **`Packages.bz2`**, and `Packages.xz`.
- ⚡ **Zero Runtime Dependencies**: A single self-contained static binary (< 20 MB). Runs in `scratch`, `alpine`, or minimal CI containers without Ruby, Python, or the `awscli` binary.
- 🌐 **Multi-Storage Protocols**: First-class support for **AWS S3** (and S3-compatible endpoints like MinIO, Ceph, Cloudflare R2), **SFTP** (SSH file transfer for Linux servers and mirrors), and **Local Filesystem** (`file://`).

---

## Installation & Distribution

### Standalone Binary
Download the precompiled static binary for your architecture from [GitHub Releases](https://github.com/debpub/debpub/releases):

```bash
# Example for Linux amd64
curl -sSL -o /usr/local/bin/debpub https://github.com/debpub/debpub/releases/latest/download/debpub-linux-amd64
chmod +x /usr/local/bin/debpub
```

### Docker
Run `debpub` directly inside any container pipeline:

```bash
docker run --rm -v $(pwd):/workspace ghcr.io/debpub/debpub:latest publish --config debpub.json ./package.deb
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

`debpub` generates and publishes both:
- `InRelease`: Modern inline clearsigned manifest (preferred by APT `1.6+`).
- `Release` + `Release.gpg`: Traditional manifest with detached signature for backwards compatibility.

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
    "timeout": "5m",
    "ttl": "10m"
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

### Option Precedence Hierarchy
`debpub` strictly enforces layered configuration priority:
1. **Command-line flags** (highest priority; explicitly overrides any setting).
2. **Config file** (values from `debpub.json` via `--config`).
3. **Environment variables** (e.g., standard AWS credential variables `AWS_PROFILE`, `AWS_ACCESS_KEY_ID`, `AWS_REGION`).
4. **Built-in defaults** (e.g. `--lock-timeout 5m`, `--component main`).

For example, you can use `debpub.json` for base bucket and codename settings, and override `--component testing` dynamically on the command line:

```bash
debpub publish --config debpub.json --component testing ./mypackage_1.0.0_amd64.deb
```

---

## Command Reference

### `debpub publish [flags] <package.deb ...>`

| Flag | Shorthand | Description | Default |
| :--- | :--- | :--- | :--- |
| `--config` | | Path to `debpub.json` configuration file | |
| `--storage` | | Storage backend (`s3`, `sftp`, `file`) | `file` |
| `--codename` | `-c` | Debian distribution codename (`stable`, `bookworm`, `jammy`) | |
| `--component` | `-m` | Debian repository component (`main`, `contrib`, `non-free`) | `main` |
| `--dir` | | Local repository root directory (for `file` storage) | `.` |
| `--bucket` | `-b` | S3 bucket name (for `s3` storage) | |
| `--prefix` | | S3 or remote directory path prefix | |
| `--s3-endpoint` | | Custom S3 endpoint URL (MinIO, Ceph, R2) | |
| `--s3-force-path-style` | | Use path-style S3 URLs (required for MinIO) | `false` |
| `--s3-legacy-locking` | | Use check-then-put locking instead of S3 conditional writes | `false` |
| `--s3-profile` | | AWS profile name to use for credentials and configuration | |
| `--s3-region` | | AWS region for S3 bucket (e.g. `us-east-1`, `eu-central-1`) | |
| `--sftp-host` | | SFTP server hostname / IP | |
| `--sftp-port` | | SFTP port | `22` |
| `--sftp-user` | | SFTP username | |
| `--sftp-password` | | SFTP password | |
| `--sftp-key` | | SFTP SSH private key file path | |
| `--preserve-versions` | | Keep older package versions in index rather than replacing | `false` |
| `--lock` | | Enable distributed repository locking | `true` |
| `--lock-timeout` | | Maximum time to wait for repository lock | `5m0s` |
| `--lock-ttl` | | Lock expiration TTL before being marked stale | `10m0s` |
| `--sign` | `-s` | Enable GPG signing of `Release` manifest | `false` |
| `--gpg-key` | `-k` | GPG signing key ID, email, or fingerprint | |
| `--gpg-passphrase` | | GPG key passphrase | |
| `--origin` | | Custom repository `Origin` header | |
| `--label` | | Custom repository `Label` header | |
| `--suite` | | Custom repository `Suite` header | |
| `--description` | | Repository `Description` header | |

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
- **Development**: Built with pair-programming assistance from Advanced AI Coding Agents.

---

## License

`debpub` is licensed under the **Apache License, Version 2.0**. See [LICENSE](LICENSE) for details.
