# LDAP Manager

A self-service password reset system for OpenLDAP with admin interface.

## Features

- 🔐 Admin authentication via LDAP with group-based authorization
- 🔑 Secure password reset token generation
- 📧 Optional email notifications
- 🛡️ Built-in security: CSRF protection, rate limiting, HMAC-signed sessions
- 💾 No external database required (tokens stored in LDAP)
- 🔄 Auto-reload of credentials from file (Docker/Kubernetes secrets support)

## Quick Start

### Prerequisites

- Go 1.22 or later
- OpenLDAP server
- SMTP server (optional, for email notifications)

### Installation

1. **Install LDAP Schema**

```bash
# Copy the schema to your LDAP server
sudo ldapadd -Y EXTERNAL -H ldapi:/// -f configs/schema.ldif
```

2. **Configure the Application**

```bash
# Copy example config
cp configs/config.yaml.example configs/config.yaml

# Edit configuration
nano configs/config.yaml
```

Required settings:
- `server.session.secret`: Generate with `openssl rand -base64 32`
- `ldap.url`: Your LDAP server URL (ldap:// or ldaps://)
- `ldap.bind_dn`: Service account DN
- `ldap.bind_password` or `ldap.bind_password_file`: Service account password
- `ldap.admin_group_dn`: DN of admin group

3. **Build and Run**

```bash
# Build
make build

# Run
./ldap-manager -config configs/config.yaml
```

Or run directly:

```bash
go run cmd/ldap-manager/main.go -config configs/config.yaml
```

### Using Environment Variables

Override any config value using environment variables with the `LDAP_MANAGER_` prefix:

```bash
export LDAP_MANAGER_SERVER_SESSION_SECRET="your-secret-key"
export LDAP_MANAGER_LDAP_BIND_PASSWORD="your-ldap-password"
export LDAP_MANAGER_LDAP_URL="ldap://ldap.example.com:389"

./ldap-manager
```

### Using Docker

```bash
# Build image
docker build -t ldap-manager .

# Run with Docker Compose
docker-compose up
```

## Configuration

See [configs/config.yaml.example](configs/config.yaml.example) for all configuration options.

### Password File Support

For Docker/Kubernetes secrets:

```yaml
ldap:
  bind_password_file: /run/secrets/ldap_password
```

The application automatically watches this file and reloads the password when it changes.

## Usage

### Admin Workflow

1. Navigate to `https://your-server:8443/admin/login`
2. Log in with your LDAP credentials
3. Search for a user
4. Generate a password reset token
5. Share the link with the user (or email is sent automatically)

### User Workflow

1. Receive reset link from admin
2. Click the link
3. Enter new password (must meet strength requirements)
4. Submit to complete reset

## Security Features

- **CSRF Protection**: Custom implementation with per-session tokens
- **Session Management**: HMAC-SHA256 signed cookies
- **Rate Limiting**: Per-IP limits on login and reset attempts
- **Token Hashing**: SHA-256 hashed tokens in LDAP
- **Single-Use Tokens**: Tokens deleted after successful use
- **TLS Support**: HTTPS with configurable certificates
- **Security Headers**: CSP, HSTS, X-Frame-Options, etc.

## Development

```bash
# Run tests
make test

# Format code
make fmt

# Lint
make lint

# Run locally
make run
```

## Architecture

- **Language**: Go 1.22+ (stdlib-focused, minimal dependencies)
- **LDAP Client**: go-ldap/ldap/v3
- **Config**: gopkg.in/yaml.v3
- **Rate Limiting**: golang.org/x/time/rate
- **File Watching**: fsnotify

No frameworks - uses Go standard library for HTTP, templating, crypto, and sessions.

## License

MIT License

## Support

For issues and questions, please use the GitHub issue tracker.
