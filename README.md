# secure-compose

> Docker Compose with age-encrypted secrets — zero-disk architecture

**secure-compose** encrypts your `.env` files and Docker secrets using [age](https://github.com/FiloSottile/age) so you can safely commit them to git and share with your team. Secrets are **never written to disk** after decryption — env vars stay in RAM and file secrets live in a tmpfs (RAM disk).

## Why?

Traditional secret management leaves decrypted secrets on disk:

- `.env` files in plaintext
- Decrypted files left behind after `docker compose up`
- Secrets survive in `/tmp` even after containers stop

**secure-compose** uses a zero-disk architecture:

| Secret Type | How It's Handled |
|-------------|------------------|
| **Env vars** (`.env.age`) | Decrypted to `map[string]string` in RAM → injected directly into container process via `Cmd.Env` |
| **File secrets** (Docker secrets) | Decrypted to tmpfs (RAM disk) → mounted into container → wiped on exit |

## Features

- 🔒 **age encryption** — X25519 / ChaCha20-Poly1305, scrypt KDF
- 🚫 **Zero-disk decrypted secrets** — env vars never touch disk
- ⚡ **tmpfs for file secrets** — RAM disk, auto-cleanup on SIGINT/SIGTERM
- 👥 **Team-friendly** — shared passphrase, no key files
- ✅ **Git-safe** — `.env.age` is safe to commit
- 🔄 **Passphrase rotation** — `secure-compose rotate` re-encrypts all `.age` files
- 🔍 **Auto-discovery** — parses `docker-compose.yaml` for secrets
- 🖥️ **Editor integration** — edit encrypted files directly
- 📤 **Stdout mode** — decrypt to stdout for piping

## Installation

### From go install (fastest)

```bash
go install github.com/rafitox/secure-compose@latest
```

### From binary

```bash
# Linux
curl -fsSL https://github.com/rafitox/secure-compose/releases/download/v0.4.0/secure-compose-linux-amd64 -o secure-compose
chmod +x secure-compose
sudo mv secure-compose /usr/local/bin/

# macOS
curl -fsSL https://github.com/rafitox/secure-compose/releases/download/v0.4.0/secure-compose-darwin-arm64 -o secure-compose
chmod +x secure-compose
sudo mv secure-compose /usr/local/bin/
```

### From source

```bash
git clone https://github.com/rafitox/secure-compose.git
cd secure-compose
go build -ldflags "-X github.com/rafitox/secure-compose/internal/cli.Version=v0.4.0" -o secure-compose .
sudo mv secure-compose /usr/local/bin/
```

### Prerequisites

- [age](https://github.com/FiloSottile/age#installation) — encryption tool
- Docker with Compose plugin (or docker-compose v1)

```bash
# macOS
brew install age

# Ubuntu/Debian
sudo apt install age
```

## Quick Start

### 1. Create your `.env` file

```bash
# .env (DO NOT COMMIT THIS!)
DATABASE_PASSWORD=super-secret
API_KEY=sk-live-xxxxx
STRIPE_SECRET=sk_test_xxxxx
```

### 2. Encrypt

```bash
secure-compose encrypt
# → Encrypting .env → .env.age
# → ✓ Encrypted successfully
# → You can safely commit .env.age to git
```

### 3. Commit `.env.age` (safe!)

```bash
echo ".env" >> .gitignore
git add .env.age
git commit -m "Add encrypted secrets"
```

### 4. Pull and run

```bash
git pull
secure-compose up -d
# → Decrypted 3 env var(s) from .env.age (in memory only)
# → Running: docker compose up -d
```

No `decrypt` step needed — secrets are decrypted **in memory only** and injected directly into the docker compose process.

## Commands

```
secure-compose encrypt                 Encrypt .env to .env.age
secure-compose encrypt --secret-file <file>  Encrypt a specific secret file
secure-compose decrypt                 Decrypt .env.age to .env (for compatibility)
secure-compose decrypt --stdout        Decrypt to stdout (for piping)
secure-compose decrypt --secret-file <file>  Decrypt a specific secret file
secure-compose edit                   Edit encrypted .env file
secure-compose rotate                 Re-encrypt all .age files with new passphrase
secure-compose run <svc> [cmd...]     Run command with secrets injected (no disk write)
secure-compose up [args]              docker compose up (auto-decrypts, in memory)
secure-compose down [args]            docker compose down
secure-compose exec <svc> <cmd>       docker compose exec
secure-compose restart [args]         docker compose restart
secure-compose logs [args]            docker compose logs
secure-compose build [args]           docker compose build
secure-compose -h, --help            Show help
secure-compose --version              Show version
```

### Options

```
-o, --stdout              Write decrypted content to stdout (for piping)
--secret-file <path>      Encrypt/decrypt a specific secret file
-f, --compose-file <path> Override compose file path (default: auto-detect)
--env-file <path>         Path to .env.age for run command
```

## Zero-Disk Architecture

### Env Vars (`.env.age`)

When you run `secure-compose up` or `secure-compose run`:

```
1. Decrypt .env.age → raw bytes in RAM
2. Parse KEY=VALUE into map[string]string
3. Inject into exec.Cmd.Env (os.Environ + decrypted vars)
4. SecureZero the raw bytes (overwrite memory)
5. exec.Command("docker", "compose", "up", "-d").Run()
6. Container reads env vars — never written to disk
```

### File Secrets (Docker Secrets)

When you have secrets defined in `docker-compose.yaml`:

```
1. Parse compose file, find secret file refs
2. Mount tmpfs (RAM disk) at /run/user/<uid>/secure-compose/<session>/
3. Decrypt each secret.<name>.age → tmpfs mount
4. Run docker compose (secrets bind-mounted from tmpfs)
5. On SIGINT/SIGTERM: unmount tmpfs, wipe files, clear RAM
```

### Memory Hardening

- **`SecureZero([]byte)`** — overwrites sensitive byte slices after use
- **`ConstantTimeCompare(a, b)`** — timing-attack resistant passphrase comparison
- Passphrase cleared from memory immediately after use

## Docker Secrets Workflow

### 1. Define secrets in `docker-compose.yaml`

```yaml
services:
  postgres:
    image: postgres
    environment:
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    secrets:
      - db_password

secrets:
  db_password:
    file: ./secrets/db_password.txt
```

### 2. Create and encrypt the secret file

```bash
# Create secrets directory
mkdir -p secrets
chmod 700 secrets

# Create and encrypt
echo "super-secret-password" > secrets/db_password.txt
secure-compose encrypt --secret-file secrets/db_password.txt
# → Remove plaintext file after encryption
# → ✓ Secret encrypted successfully
```

### 3. Run

```bash
secure-compose up -d
# → Found 1 file secret(s) in compose
# → Secret 'db_password' mounted at /run/user/1000/secure-compose/session-12345/db_password (RAM disk)
# → Running: docker compose up -d
```

On Ctrl+C or `secure-compose down`, the tmpfs mount is unmounted and all secret files are wiped from RAM.

## Passphrase Rotation

When a team member leaves or you suspect the passphrase is compromised:

```bash
secure-compose rotate
# → Rotate passphrase for all .age files
#
# ⚠  This will re-encrypt all .age files with a new passphrase.
#    All team members must be notified of the new passphrase.
#
# → Enter current passphrase: ********
# → Enter new passphrase: ********
# → Confirm new passphrase: ********
#
# → Found 4 .age file(s) to re-encrypt
# → Rotated: .env.age
# → Rotated: secrets/db_password.txt.age
# → Rotated: secrets/api_key.age
# → ✓ Rotated 4 file(s)
#
# → Share the new passphrase with your team via 1Password/Vault
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SECURE_COMPOSE_ENV_FILE` | Path to `.env` file | `.env` |
| `SECURE_COMPOSE_ENCRYPTED_FILE` | Path to `.env.age` | `.env.age` |
| `SECURE_COMPOSE_SECRET_FILE` | Specific secret file for encrypt/decrypt | (none) |
| `SECURE_COMPOSE_PASSPHRASE` | Passphrase (for CI/automation) | (none) |
| `SECURE_COMPOSE_NO_TEARDOWN` | Skip tmpfs cleanup | `0` |
| `SECURE_COMPOSE_VERSION` | Override version string | (none) |
| `EDITOR` | Editor for `secure-compose edit` | `vim` |

## Security

### What changed in v0.4.0

Previous versions decrypted secrets to disk (`.env` file persisted after `up`). v0.4.0 uses a zero-disk architecture:

- **Env vars**: decrypted to `map[string]string` in RAM, injected via `Cmd.Env`, never written to disk
- **File secrets**: decrypted to tmpfs (RAM disk), bind-mounted to container, wiped on exit

### How encryption works

1. **`age --encrypt --passphrase --armor`**
   - Scrypt KDF for key derivation
   - ChaCha20-Poly1305 for authenticated encryption
   - ASCII-armored output (readable, git-friendly)

2. **Passphrase sharing**
   - Share via your team's secret manager (1Password, Vault, etc.)
   - Rotate regularly with `secure-compose rotate`
   - Use `SECURE_COMPOSE_PASSPHRASE` in CI/CD

### What it doesn't do

- ❌ **Not a replacement for a real secrets manager** (Vault, AWS Secrets Manager) in production
- ❌ **No per-user keys** — everyone shares the same passphrase
- ❌ **No audit logging** — for production, use your cloud's secrets service

## Comparison

| Feature | secure-compose | git-crypt | SOPS |
|---------|---------------|-----------|------|
| Zero-disk decrypted secrets | ✅ | ❌ | ❌ |
| Age encryption | ✅ | ❌ (GPG) | ✅ |
| Team-friendly | ✅ | ✅ | ✅ |
| Docker secrets support | ✅ | ❌ | ✅ |
| Auto-discovery | ✅ | ❌ | ❌ |
| Passphrase rotation | ✅ | ❌ | ✅ |
| CI/CD friendly | ✅ | ✅ | ✅ |

## License

MIT
