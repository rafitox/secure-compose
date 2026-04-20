# secure-compose

> Docker Compose with age-encrypted secrets for teams

**secure-compose** is a CLI tool that encrypts your `.env` files using [age](https://github.com/FiloSottile/age) (a simple, modern encryption tool) so you can safely commit secrets to git and share them with your team.

## Why?

When you use Docker Compose in development, you typically store secrets in a `.env` file. This file:
- Often contains passwords, API keys, and tokens in **plaintext**
- Gets committed to git by mistake
- Lives unencrypted on servers

**secure-compose** solves this by:
- Encrypting `.env` → `.env.age` with a passphrase
- Allowing safe commit of `.env.age` to git
- Decrypting transparently when running `docker compose`
- No key files to manage — just a shared passphrase

## Features

- 🔒 **age encryption** — Modern, well-vetted encryption (X25519 / Scrypt)
- 👥 **Team-friendly** — Same passphrase for the whole team
- 🚫 **No key files** — Passphrase only, easier to rotate
- ✅ **Git-safe** — `.env.age` is safe to commit
- 🖥️ **Editor integration** — Edit encrypted files directly
- 🔄 **Docker compose wrapper** — Works with existing workflows

## Installation

### From binary (recommended)

```bash
# Linux
curl -fsSL https://github.com/rafitox/secure-compose/releases/download/v0.1.0/secure-compose-linux-amd64 -o secure-compose
chmod +x secure-compose
sudo mv secure-compose /usr/local/bin/

# macOS
curl -fsSL https://github.com/rafitox/secure-compose/releases/download/v0.1.0/secure-compose-darwin-arm64 -o secure-compose
chmod +x secure-compose
sudo mv secure-compose /usr/local/bin/
```

### From source

```bash
git clone https://github.com/rafitox/secure-compose.git
cd secure-compose
make install
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
DATABASE_PASSWORD=super-secret-password
API_KEY=sk-live-xxxxxxxxxxxxx
STRIPE_SECRET=sk_test_xxxx
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
secure-compose encrypt               Encrypt .env to .env.age
secure-compose decrypt               Decrypt .env.age to .env
secure-compose edit                 Edit encrypted file
secure-compose up [args]            docker compose up
secure-compose down [args]          docker compose down
secure-compose exec <svc> <cmd>     docker compose exec
secure-compose restart [args]       docker compose restart
secure-compose logs [args]          docker compose logs
secure-compose build [args]          docker compose build
secure-compose -h, --help           Show help
secure-compose --version            Show version
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `SECURE_COMPOSE_ENV_FILE` | Path to .env file | `.env` |
| `SECURE_COMPOSE_ENCRYPTED_FILE` | Path to .env.age | `.env.age` |
| `SECURE_COMPOSE_PASSPHRASE` | Passphrase (for CI/automation) | (none) |
| `SECURE_COMPOSE_NO_TEARDOWN` | Skip .env cleanup | `0` |
| `EDITOR` | Editor for `secure-compose edit` | `vim` |

## Security

### How it works

1. **Encryption**: `age --encrypt --passphrase --armor`
   - Uses [Scrypt](https://en.wikipedia.org/wiki/Scrypt) KDF for key derivation
   - Encrypts with X25519 (if using key files) or Argon2id (passphrase)
   - Outputs ASCII-armored format (readable, git-friendly)

2. **Passphrase sharing**
   - Share via your team's secret manager (1Password, Vault, etc.)
   - Rotate regularly and when team members leave
   - Consider using `SECURE_COMPOSE_PASSPHRASE` in CI/CD

3. **Cleanup**
   - `.env` file is automatically deleted after `up`, `exec`, etc.
   - Only `.env.age` persists on disk

### What it doesn't do

- ❌ **Not a replacement for a real secrets manager** (Vault, AWS Secrets Manager) in production
- ❌ **No per-user keys** — everyone shares the same passphrase
- ❌ **No audit logging** — for production, use your cloud's secrets service

## Workflow Examples

### Development

```bash
# Morning: pull and decrypt
git pull
secure-compose decrypt

# Work with docker compose
secure-compose up -d
secure-compose logs -f api

# Evening: cleanup (automatic, or manual)
secure-compose down
rm -f .env
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

## Comparison

| Feature | secure-compose | git-crypt | SOPS |
|---------|---------------|-----------|------|
| Ease of use | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |
| No git filters | ⭐⭐⭐⭐⭐ | ⭐⭐ | ⭐⭐⭐⭐ |
| Team-friendly | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ |
| Age encryption | ✅ | ❌ (GPG) | ✅ |
| CI/CD friendly | ⭐⭐⭐⭐⭐ | ⭐⭐⭐ | ⭐⭐⭐⭐ |

## License

MIT
