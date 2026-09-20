# Debian Packaging & Repository Rules

These rules govern all Debian packaging conventions, control file handling, index generation, and repository layouts implemented in `debpub`.

## 1. Debian Repository Hierarchy & Layout
- **Pool Structure**: Binary packages (`.deb`) must be stored in the pool directory organized by component and package name prefix:
  `pool/<component>/<prefix>/<package>/<filename>.deb` (or `pool/<component>/lib<prefix>/<package>/...` for packages beginning with `lib`).
- **Dists Structure**: Repository metadata resides under `dists/<distribution>/<component>/binary-<arch>/`:
  - `Packages`: Uncompressed RFC 822 / deb822 control paragraphs for all packages in the architecture.
  - `Packages.gz`: Gzip-compressed index.
  - `Packages.bz2`: Bzip2-compressed index.
  - `Packages.xz`: XZ-compressed index.
- **Release Manifests**:
  - `Release`: Located at `dists/<distribution>/Release`, containing architecture/component metadata and cryptographic checksums (SHA256, SHA512, MD5, SHA1) and file sizes for all `Packages*` files.
  - `Release.gpg`: Detached OpenPGP signature.
  - `InRelease`: Inline clear-signed Release file.

## 2. RFC 822 / deb822 Parsing & Writing Standards
- **Line Endings & Whitespace**: Debian control files strictly use Unix LF (`\n`). Continuation lines for multiline fields (such as `Description`) must begin with a single space or tab. Empty lines in descriptions must be formatted as ` .` (space followed by dot).
- **Field Ordering**: Standard Debian field order must be maintained in generated `Packages` indices:
  `Package`, `Version`, `Installed-Size`, `Maintainer`, `Architecture`, `Depends`, `Section`, `Priority`, `Filename`, `Size`, `SHA256`, `SHA1`, `MD5sum`, `Description`.
- **Atomic Operations**: Never write partially constructed indices directly to the remote repository. Always stage files in memory or temporary files, compute checksums, and atomically commit them under distributed lock.

## 3. Version Comparison & Ordering
- Deb version comparison follows Debian policy manual specifications:
  `[epoch:]upstream_version[-debian_revision]`
- Comparison compares digits numerically and non-digits lexicographically, with `~` sorting before anything (including empty string) to handle pre-releases cleanly.

## 4. Stateless Concurrency & Distributed Locking
- S3 locking relies on conditional writes (`If-None-Match: "*"`) to create `.lock` files containing JSON metadata (`id`, `created_at`, `expires_at`, `hostname`).
- Stale locks must be safely detected via expiration timestamps before recovery.
- The repository index (`Packages`) in storage is the sole database — no external DBMS or stateful database tarballs.
