# `debpub` Configuration Specification (`debpub.json`)

To avoid repeating flags across CI/CD pipeline steps, `debpub` can be configured declaratively via a `debpub.json` file passed with `--config`.

---

## Annotated Configuration Example

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
    "key": "support@example.com",
    "passphrase": ""
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
  },
  "sftp": {
    "host": "repo.example.com",
    "port": 22,
    "user": "deployer",
    "key": "~/.ssh/id_ed25519"
  }
}
```

Running with a configuration file:
```bash
debpub publish --config debpub.json ./mypackage_1.0.0_amd64.deb
```

---

## Configuration Layering & Option Precedence

`debpub` strictly enforces a layered configuration priority:

```text
1. Command-line flags        (Highest priority: explicitly overrides any setting)
        ↓
2. Config file (debpub.json) (Loaded when --config <file> is provided)
        ↓
3. Environment variables     (e.g., AWS_REGION, AWS_PROFILE, USER)
        ↓
4. Built-in defaults         (e.g., --lock-timeout 2m, --component main, --dir .)
```

---

## Smart Auto-Detection & Fallback Rules

- **Suite Fallback**: If `--suite` is omitted, it automatically defaults to the distribution `--codename`.
- **GPG Auto-Sign**: Supplying `--gpg-key` automatically enables Release manifest signing (`--sign`) without requiring an explicit `-s` flag.
- **SFTP User**: If `--sftp-user` is omitted, it automatically defaults to the invoking system user (`$USER`).
- **Flexible Time Formats**: `--lock-timeout` and `--lock-ttl` accept both duration strings (e.g. `2m`, `120s`, `3m`, `180s`) and raw integer seconds without units (e.g. `120`, `180`).
- **Auditable Logging**: When defaults or smart fallbacks are applied, `debpub` logs them to standard output for complete transparency in CI/CD pipelines.
