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
