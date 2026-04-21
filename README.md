# secure-compose

> Docker Compose with age-encrypted secrets for teams and Docker secrets

**secure-compose** is a CLI tool that encrypts your `.env` files and Docker secrets using [age](https://github.com/FiloSottile/age) (a simple, modern encryption tool) so you can safely commit secrets to git and share them with your team. It also auto-discovers and decrypts secrets defined in your `docker-compose.yaml`.

## Why?

When you use Docker Compose in development, you typically store secrets in a `.env` file or use Docker secrets. These:
- Often contain passwords, API keys, and tokens in **plaintext**
- Get committed to git by mistake
- Live unencrypted on servers

**secure-compose** solves this by:
- Encrypting `.env` → `.env.age` with a passphrase
- Supporting Docker secrets (`--secret-file`) for compose-based workflows
- Auto-discovering secrets from `docker-compose.yaml` and decrypting them before `up`
- Allowing safe commit of `.env.age` to git
- Decrypting transparently when running `docker compose`
- No key files to manage — just a shared passphrase

## Features

- 🔒 **age encryption** — Modern, well-vetted encryption (X25519 / Scrypt)
- 👥 **Team-friendly** — Same passphrase for the whole team
- 🚫 **No key files** — Passphrase only, easier to rotate
- ✅ **Git-safe** — `.env.age` is safe to commit
- 🐳 **Docker secrets support** — Encrypt/decrypt secret files for compose
- 🔍 **Auto-discovery** — Parses `docker-compose.yaml` for secrets automatically
- 🖥️ **Editor integration** — Edit encrypted files directly
- 🔄 **Docker compose wrapper** — Works with existing workflows
- 📤 **Stdout mode** — Decrypt to stdout for piping workflows

## Installation

### From go install (fastest)

```bash
go install github.com/rafitox/secure-compose@latest
```

### From binary

```bash
# Linux
curl -fsSL https://github.com/rafitox/secure-compose/releases/download/v0.2.0/secure-compose-linux-amd64 -o secure-compose
chmod +x secure-compose
sudo mv secure-compose /usr/local/bin/

# macOS
curl -fsSL https://github.com/rafitox/secure-compose/releases/download/v0.2.0/secure-compose-darwin-arm64 -o secure-compose
chmod +x secure-compose
sudo mv secure-compose /usr/local/bin/
```

### From source

```bash
git clone https://github.com/rafitox/secure-compose.git
cd secure-compose
go build -o secure-compose .
sudo mv secure-compose /usr/local/bin/
```

### Prerequisites

- [age](https://github.com/FiloSottile/age#installation) — Encryption tool
- Docker with Compose plugin (or docker-compose v1)

```bash
# macOS
brew install age

# Ubuntu/Debian
sudo apt install age

# Docker (Compose v2 comes bundled)
# For v1: sudo apt install docker-compose
```

## Quick Start

### 1. Create your .env file

```bash
# .env (DO NOT COMMIT THIS!)
DATABASE_PASSWORD=super-secret-word
API_KEY=***
STRIPE_SECRET=***
```

### 2. Encrypt

```bash
secure-compose encrypt
# → Encrypting .env → .env.age
# → Enter passphrase: ********
# → Confirm passphrase: ********
# → ✓ Encrypted successfully
# → You can safely commit .env.age to git
```

### 3. Commit .env.age (safe!)

```bash
echo ".env" >> .gitignore
git add .env.age
git commit -m "Add encrypted secrets"
```

### 4. Team member decrypts

```bash
git pull
secure-compose decrypt
# → Enter passphrase: ********
# → ✓ Decrypted successfully
```

### 5. Run docker compose

```bash
secure-compose up -d
# → Decrypting secrets...
# → Running: docker compose up -d

# Or use any docker compose command
secure-compose logs -f
secure-compose exec postgres psql -U postgres
secure-compose down
```

## Commands

```
secure-compose encrypt                 Encrypt .env to .env.age
secure-compose encrypt --secret-file <file>  Encrypt a specific secret file
secure-compose decrypt                 Decrypt .env.age to .env
secure-compose decrypt -o             Decrypt to stdout (for piping)
secure-compose decrypt --secret-file <file>  Decrypt a specific secret file
secure-compose edit                   Edit encrypted file
secure-compose up [args]              docker compose up (auto-decrypts secrets)
secure-compose down [args]            docker compose down
secure-compose exec <svc> <cmd>       docker compose exec
secure-compose restart [args]         docker compose restart
secure-compose logs [args]            docker compose logs
secure-compose build [args]            docker compose build
secure-compose -h, --help             Show help
secure-compose --version              Show version
```

### Options

```
-o, --stdout              Write decrypted content to stdout (for piping)
-s, --secret-file <path>  Encrypt/decrypt a specific secret file
-f, --compose-file <path> Override compose file path (default: auto-detect)
```

## Docker Secrets Workflow

For projects using Docker's built-in secrets feature:

### 1. Define secrets in docker-compose.yaml

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

# Create secret file
echo "super-secret-password" > secrets/db_password.txt

# Encrypt it
secure-compose encrypt --secret-file secrets/db_password.txt
# → Encrypting secrets/db_password.txt → secrets/db_password.txt.age
# → Remove plaintext file after encryption
# → ✓ Secret encrypted successfully

# Add to gitignore
echo "secrets/" >> .gitignore
```

### 3. Team member decrypts and runs

```bash
git pull
secure-compose up -d
# → Found 1 secret(s) in compose file
# → Decrypting secret: db_password
# → Decrypting secrets...
# → Running: docker compose up -d
```

## Stdout Mode (Piping)

Decrypt directly to stdout for use in scripts and piping:

```bash
# Pipe to another process
secure-compose decrypt --stdout | jq -r '.DATABASE_PASSWORD'

# Redirect to a file
secure-compose decrypt --stdout > backup.env

# Use with secret files
secure-compose decrypt --secret-file secrets/db_password.txt --stdout

# CI/CD piping
SECURE_COMPOSE_PASSPHRASE="$PASSPHRASE" secure-compose decrypt --stdout
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SECURE_COMPOSE_ENV_FILE` | Path to .env file | `.env` |
| `SECURE_COMPOSE_ENCRYPTED_FILE` | Path to .env.age | `.env.age` |
| `SECURE_COMPOSE_SECRET_FILE` | Specific secret file for encrypt/decrypt | (none) |
| `SECURE_COMPOSE_SECRETS_DIR` | Directory for secret files | (none) |
| `SECURE_COMPOSE_PASSPHRASE` | Passphrase (for CI/automation) | (none) |
| `SECURE_COMPOSE_NO_TEARDOWN` | Skip .env cleanup | `0` |
| `EDITOR` | Editor for `secure-compose edit` | `vim` |

## Security

### How it works

1. **Encryption**: Uses `age --encrypt --passphrase --armor`
   - Uses [Scrypt](https://en.wikipedia.org/wiki/Scrypt) KDF for key derivation
   - Encrypts with ChaCha20-Poly1305 (passphrase mode)
   - Outputs ASCII-armored format (readable, git-friendly)

2. **Passphrase sharing**
   - Share via your team's secret manager (1Password, Vault, etc.)
   - Rotate regularly and when team members leave
   - Consider using `SECURE_COMPOSE_PASSPHRASE` in CI/CD

3. **Cleanup**
   - `.env` and secret files are persisted after `up` (for container access)
   - Remember to remove them manually when done: `rm .env secrets/*.txt`

### Docker Secrets Security

When using Docker secrets:
- Secret files are mounted at `/run/secrets/<name>` inside the container
- This is more secure than environment variables (which can leak via logs)
- Use `_FILE` environment variables (e.g., `POSTGRES_PASSWORD_FILE`) to read from secrets
- Consider mounting secrets directory as tmpfs to prevent disk writes

### What it doesn't do

- ❌ **Not a replacement for a real secrets manager** (Vault, AWS Secrets Manager) in production
- ❌ **No per-user keys** — everyone shares the same passphrase
- ❌ **No audit logging** — for production, use your cloud's secrets service

## Workflow Examples

### Development with .env files

```bash
# Morning: pull and decrypt
git pull
secure-compose decrypt

# Work with docker compose
secure-compose up -d
secure-compose logs -f api

# Evening: cleanup
secure-compose down
rm -f .env
```

### Development with Docker secrets

```bash
# Morning: pull
git pull

# Start (auto-decrypts secrets from compose file)
secure-compose up -d
# → Found 1 secret(s) in compose file
# → Decrypting secret: db_password
# → Running: docker compose up -d

# Work normally
secure-compose logs -f
secure-compose exec postgres psql -U postgres

# Evening: cleanup
secure-compose down
rm -f secrets/*.txt
```

### CI/CD (GitHub Actions example)

```yaml
- name: Deploy
  env:
    SECURE_COMPOSE_PASSPHRASE: ${{ secrets.PASSPHRASE }}
  run: |
    secure-compose up -d
```

### Editing secrets

```bash
secure-compose edit
# → Decrypts .env.age to temp file
# → Opens $EDITOR
# → Re-encrypts after save
```

### Piping workflows

```bash
# Inspect a specific value
secure-compose decrypt --stdout | grep DATABASE

# Backup decrypted secrets
secure-compose decrypt --stdout > secrets.env

# Use with jq for JSON conversion
secure-compose decrypt --stdout | jq -r 'to_entries | .[] | "\(.key)=\(.value)"'
```

## Comparison

| Feature | secure-compose | git-crypt | SOPS |
|---------|---------------|-----------|------|
| Ease of use | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| No git filters | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐ |
| Team-friendly | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| Age encryption | ✅ | ❌ (GPG) | ✅ |
| Docker secrets support | ✅ | ❌ | ✅ |
| CI/CD friendly | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| Auto-discovery | ✅ | ❌ | ❌ |

## License

MIT
