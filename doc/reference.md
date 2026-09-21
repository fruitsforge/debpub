# `debpub` Command & Flag Reference

Complete reference guide for all `debpub` CLI commands and options.

---

## 1. `debpub publish`

Publishes one or more Debian packages (`.deb`, `.udeb`, `.ddeb`) to the destination repository, uploads pool files, builds compressed indices (`Packages`, `.gz`, `.bz2`, `.xz`), and generates an atomic `Release` / `InRelease` manifest.

### Usage
```bash
debpub publish [flags] <file.deb|pattern|directory...>
```

### Flags Specification

| Flag | Shorthand | Type | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `--config` | | string | | Path to `debpub.json` configuration file |
| `--storage` | | string | `file` | Storage backend (`s3`, `sftp`, `file`) |
| `--codename` | `-c` | string | *(Required)* | Debian distribution codename (`stable`, `bookworm`, `jammy`) |
| `--component` | `-m` | string | `main` | Debian repository component (`main`, `contrib`, `non-free`) |
| `--dir` | | string | `.` | Local repository root directory (for `file` storage) |
| `--bucket` | `-b` | string | *(Required for S3)* | S3 bucket name |
| `--prefix` | | string | `""` | S3 or remote directory path prefix |
| `--s3-endpoint` | | string | | Custom S3 endpoint URL (MinIO, Ceph, R2) |
| `--s3-force-path-style` | | boolean | `false` | Use path-style S3 URLs (required for MinIO) |
| `--s3-legacy-locking` | | boolean | `false` | Use check-then-put locking instead of S3 conditional writes |
| `--s3-profile` | | string | | AWS profile name to use for credentials and configuration |
| `--s3-region` | | string | | AWS region for S3 bucket (e.g. `us-east-1`, `eu-central-1`) |
| `--sftp-host` | | string | *(Required for SFTP)* | SFTP server hostname or IP address |
| `--sftp-port` | | int | `22` | SFTP server port |
| `--sftp-user` | | string | `$USER` | SFTP username (defaults to current system user) |
| `--sftp-password` | | string | | SFTP password |
| `--sftp-key` | | string | | SFTP SSH private key file path |
| `--preserve-versions` | | boolean | `true` | Keep older package versions in index rather than replacing |
| `--lock` | | boolean | `true` | Enable distributed repository locking |
| `--lock-timeout` | | duration | `2m` | Max wait time for lock (accepts `2m`, `120s`, or `120`) |
| `--lock-ttl` | | duration | `3m` | Lock expiration TTL before being marked stale (accepts `3m`, `180s`, or `180`) |
| `--sign` | `-s` | boolean | `false` | Enable GPG signing of `Release` manifest (auto if `--gpg-key` is set) |
| `--gpg-key` | `-k` | string | | GPG signing key ID, email, or fingerprint |
| `--gpg-passphrase` | | string | | GPG key passphrase |
| `--origin` | | string | | Custom repository `Origin` header |
| `--label` | | string | | Custom repository `Label` header |
| `--suite` | | string | `--codename` | Custom repository `Suite` header |
| `--description` | | string | | Custom repository `Description` header |

---

## 2. `debpub serve`

Starts an embedded HTTP/HTTPS server with REST API endpoints and an interactive single-page web UI to browse packages, inspect metadata, filter versions, and sync indexes.

### Usage
```bash
debpub serve [flags]
```

### Flags Specification

| Flag | Shorthand | Type | Default | Description |
| :--- | :--- | :--- | :--- | :--- |
| `--server-port` | | int | `8080` | HTTP/HTTPS server listening port |
| `--server-bind` | | string | `0.0.0.0` | HTTP/HTTPS server bind IP address or hostname |
| `--server-tls-cert` | | string | `""` | Path to TLS certificate file for HTTPS |
| `--server-tls-key` | | string | `""` | Path to TLS private key file for HTTPS |
| `--config` | | string | | Path to `debpub.json` configuration file |
| `--storage` | | string | `file` | Storage backend (`s3`, `sftp`, `file`) |
| `--codename` | `-c` | string | `""` | Distribution codename (auto-discovered if omitted) |
| `--component` | `-m` | string | `main` | Repository component (auto-discovered from `Release` or `dists/`) |
| `--dir` | | string | `.` | Local repository root directory (for `file` storage) |
| `--bucket` | `-b` | string | | S3 bucket name (for `s3` storage) |
| `--prefix` | | string | `""` | S3 or remote directory path prefix |
| `--s3-profile` | | string | | AWS profile name for S3 credentials |
| `--s3-region` | | string | | AWS region for S3 bucket |
| `--s3-endpoint` | | string | | Custom S3 endpoint URL (MinIO, Ceph, R2) |
| `--s3-force-path-style` | | boolean | `false` | Use path-style S3 URLs |
| `--sftp-host` | | string | | SFTP server hostname or IP address |
| `--sftp-port` | | int | `22` | SFTP port |
| `--sftp-user` | | string | `$USER` | SFTP username |
| `--sftp-key` | | string | | SFTP SSH private key file path |
| `--sftp-password` | | string | | SFTP password |
