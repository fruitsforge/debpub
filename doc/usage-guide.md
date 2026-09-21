# `debpub` Usage Guide & Recipes

This guide provides practical examples, common usage patterns, and advanced recipes for publishing and managing Debian repositories with `debpub`.

---

## Table of Contents

- [1. Publishing to AWS S3 & S3-Compatible Storage](#1-publishing-to-aws-s3--s3-compatible-storage)
  - [Standard AWS S3 Bucket](#standard-aws-s3-bucket)
  - [MinIO, Ceph, Wasabi, or Cloudflare R2](#minio-ceph-wasabi-or-cloudflare-r2)
  - [AWS Profile and Explicit Region](#aws-profile-and-explicit-region)
- [2. Publishing to Local Filesystem or HTTP Roots](#2-publishing-to-local-filesystem-or-http-roots)
- [3. Publishing to Remote Server via SFTP](#3-publishing-to-remote-server-via-sftp)
- [4. Dual Manifest GPG Signing (`InRelease` + `Release.gpg`)](#4-dual-manifest-gpg-signing-inrelease--releasegpg)
- [5. High-Throughput Batch Publishing](#5-high-throughput-batch-publishing)
  - [Multiple Explicit Packages](#multiple-explicit-packages)
  - [Wildcard / Glob Patterns](#wildcard--glob-patterns)
  - [Recursive Directory Scanning](#recursive-directory-scanning)
  - [Why Batch Publishing is Superior in CI/CD](#why-batch-publishing-is-superior-in-cicd)
- [6. Running the Repository Web Browser (`debpub serve`)](#6-running-the-repository-web-browser-debpub-serve)
  - [Local Directory](#local-directory)
  - [AWS S3 with Auto-Discovery](#aws-s3-with-auto-discovery)
  - [Remote SFTP Repository](#remote-sftp-repository)
  - [Custom HTTPS / TLS Termination](#custom-https--tls-termination)
  - [REST API Endpoints](#rest-api-endpoints)

---

## 1. Publishing to AWS S3 & S3-Compatible Storage

`debpub` natively interfaces with AWS S3 and any S3-compliant object store using the AWS SDK for Go v2.

### Standard AWS S3 Bucket

Publish a package to an S3 bucket with distributed locking enabled:

```bash
debpub publish \
  --storage s3 \
  --bucket my-debian-repo \
  --prefix debian \
  --codename stable \
  --component main \
  ./mypackage_1.0.0_amd64.deb
```

### MinIO, Ceph, Wasabi, or Cloudflare R2

Use custom endpoints and force path-style URLs:

```bash
debpub publish \
  --storage s3 \
  --bucket my-repo \
  --s3-endpoint http://minio.local:9000 \
  --s3-force-path-style \
  --codename bookworm \
  ./mypackage_1.0.0_amd64.deb
```

### AWS Profile and Explicit Region

Specify custom AWS profiles and regions directly via flags:

```bash
debpub publish \
  --storage s3 \
  --bucket my-debian-repo \
  --s3-profile production \
  --s3-region eu-central-1 \
  --codename bookworm \
  ./mypackage_1.0.0_amd64.deb
```

---

## 2. Publishing to Local Filesystem or HTTP Roots

Ideal for local testing, bind mounts, or static web servers (such as Nginx or Apache):

```bash
debpub publish \
  --storage file \
  --dir /var/www/repos/apt \
  --codename stable \
  --component main \
  ./mypackage_1.0.0_amd64.deb
```

---

## 3. Publishing to Remote Server via SFTP

For publishing directly to remote Linux servers, appliances, or mirrors over SSH using key-based authentication:

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

---

## 4. Dual Manifest GPG Signing (`InRelease` + `Release.gpg`)

`debpub` generates modern dual-manifest signatures compliant with Debian/Ubuntu standards:

- **`InRelease`**: An inline-signed cleartext manifest (`BEGIN PGP SIGNED MESSAGE`).
- **`Release.gpg`**: A detached binary OpenPGP signature (`BEGIN PGP SIGNATURE`).

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

> [!TIP]
> Supplying `--gpg-key` automatically activates `--sign` without needing to specify `-s`.

---

## 5. High-Throughput Batch Publishing

`debpub` natively supports publishing multiple packages within a **single atomic transaction**. You can supply multiple file arguments, wildcard glob expressions, and directories.

### Multiple Explicit Packages
```bash
debpub publish \
  --storage s3 \
  --bucket my-debian-repo \
  --codename stable \
  ./pkg1_1.0.0_amd64.deb ./pkg2_2.1.0_amd64.deb ./pkg3_0.5.0_all.deb
```

### Wildcard / Glob Patterns
```bash
# Quoted patterns are expanded by debpub directly (ideal for CI/CD runners & Windows):
debpub publish \
  --storage s3 \
  --bucket my-debian-repo \
  --codename stable \
  "./dist/*.deb"
```

### Recursive Directory Scanning
```bash
# Recursively traverses directory tree and discovers all .deb, .udeb, and .ddeb packages:
debpub publish \
  --storage s3 \
  --bucket my-debian-repo \
  --codename stable \
  ./build/output/
```

### Why Batch Publishing is Superior in CI/CD

> [!IMPORTANT]
> When uploading multiple packages in CI/CD, batching is significantly faster and safer:
> 1. **Single Lock Acquisition**: Acquires the distributed lock once for the entire batch.
> 2. **Single Index Rebuild**: Computes hashes and compresses index files (`Packages.gz`, `Packages.bz2`, `Packages.xz`) once.
> 3. **Reduced API Operations**: Saves up to 90% of S3 PUT calls and avoids concurrent race conditions between pipeline steps.

---

## 6. Running the Repository Web Browser (`debpub serve`)

`debpub` includes an embedded zero-dependency HTTP/HTTPS server and single-page web UI to browse repositories, inspect package details (control fields, architectures, dependencies, SHA-256 hashes), and dynamically sync index files.

### Local Directory
```bash
debpub serve --storage file --dir /var/www/repos/apt --server-port 8080
```

### AWS S3 with Auto-Discovery
```bash
debpub serve \
  --storage s3 \
  --bucket repo-deb.dev.example.com \
  --s3-profile dev \
  --server-port 8090
```

### Remote SFTP Repository
```bash
debpub serve \
  --storage sftp \
  --sftp-host repo.example.com \
  --sftp-user debian \
  --sftp-key ~/.ssh/id_ed25519 \
  --prefix /var/www/debian \
  --server-port 8080
```

### Custom HTTPS / TLS Termination
```bash
debpub serve \
  --storage s3 \
  --bucket my-debian-repo \
  --server-port 8443 \
  --server-tls-cert /path/to/cert.pem \
  --server-tls-key /path/to/key.pem
```

Open **`http://localhost:8080/`** (or your configured port) in your browser.

### REST API Endpoints

The embedded daemon exposes REST API endpoints for automation and tooling:

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/api/info?codename=<name>&component=<comp>` | `GET` | Returns repository metadata, active targets, and total package/version counts. |
| `/api/packages?codename=<name>&component=<comp>&arch=<arch>&q=<search>` | `GET` | Lists package cards with all available versions. |
| `/api/packages/{name}?codename=<name>&component=<comp>&version=<ver>` | `GET` | Returns complete Debian control paragraph for a package. |
| `/api/fetch?codename=<name>&component=<comp>` | `POST` | Triggers an in-memory re-sync and auto-discovery of remote indexes. |
