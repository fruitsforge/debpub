---
name: debian-packaging
description: Expert AI assistant for Debian package analysis, deb822 control generation, repository layout (dists/pool), compression formats (gz, bz2, xz), and cryptographic release signing.
---

# Debian Packaging & Repository Management Skill

Practical procedures and guidelines for building and managing Debian package archives.

## 1. Debian Archive Layout & Standards
When inspecting, generating, or validating Debian repositories:
- Binary `.deb` files belong in `pool/<component>/<prefix>/<package>/<filename>.deb`.
- Indices reside in `dists/<distribution>/<component>/binary-<arch>/`:
  - `Packages`: raw deb822 text format.
  - `Packages.gz`: gzip compressed.
  - `Packages.bz2`: bzip2 compressed.
  - `Packages.xz`: xz / lzma compressed.
- Release manifests reside in `dists/<distribution>/`:
  - `Release`: Contains metadata header followed by `MD5Sum:`, `SHA1:`, `SHA256:`, and `SHA512:` tables listing all `Packages*` relative paths, byte counts, and hex hashes.
  - `InRelease`: Inline PGP signed version of `Release`.
  - `Release.gpg`: Detached PGP signature of `Release`.

## 2. Inspecting Debian Packages
- Extract control files: `dpkg-deb -e <package.deb> <target_dir>`
- Extract data contents: `dpkg-deb -x <package.deb> <target_dir>`
- Inspect metadata info: `dpkg-deb -I <package.deb>`
- List contents: `dpkg-deb -c <package.deb>`
- Inspect raw ar archive headers: `ar -t <package.deb>` (standard Debian debs contain `debian-binary`, `control.tar.gz`/`control.tar.xz`/`control.tar.zst`, and `data.tar.*`).

## 3. Formatting Guidelines for deb822
- Never emit trailing whitespace on field values.
- Multiline continuation lines must have a leading space.
- Empty lines within multiline description bodies must be rendered as ` .`.
- Separate individual package paragraphs with exactly one empty line (`\n\n`).
