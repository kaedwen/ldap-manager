# LDAP Manager

A secure, self-service password reset system for OpenLDAP with admin interface and optional SSO/2FA integration.

[![Build](https://github.com/kaedwen/ldap-manager/actions/workflows/build.yml/badge.svg)](https://github.com/kaedwen/ldap-manager/actions)
[![License](https://img.shields.io/github/license/kaedwen/ldap-manager)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/kaedwen/ldap-manager)](go.mod)

## Features

- 🔐 **Secure Password Reset**: Time-limited, single-use tokens stored as SHA-256 hashes in LDAP
- 👥 **Admin Interface**: Search users, generate reset links, optional email delivery
- 🔑 **Dual Authentication Modes**:
  - **Internal**: Built-in login with LDAP credentials
  - **Proxy**: SSO via OIDC proxy (Authelia, Authentik, Keycloak, etc.) with 2FA
- 📧 **Email Notifications**: Optional SMTP integration for automatic token delivery
- 🛡️ **Security First**:
  - CSRF protection
  - Rate limiting
  - HMAC-signed sessions
  - Security headers (CSP, HSTS, etc.)
  - Password strength validation
- 🐳 **Container-Ready**: Multi-arch Docker images (amd64, arm64) with health checks
- 📦 **Minimal**: ~10MB container image built from scratch
- 🎯 **Zero External DB**: All data stored in LDAP custom attributes

## Quick Start

### 1. Install LDAP Schema

Add custom attributes to your OpenLDAP server:

```bash
# SSH to your LDAP server
ssh ldap-server

# Add schema (requires OpenLDAP 2.4+)
ldapadd -Y EXTERNAL -H ldapi:/// -f configs/schema.ldif
```

### 2. Configure

Create `config.yaml`:

```yaml
server:
  port: 8080
  health_port: 9090
  session:
    secret: "generate-with-openssl-rand-base64-32"
  auth:
    mode: internal  # or "proxy" for OIDC

ldap:
  url: ldap://ldap.example.com:389
  bind_dn: cn=admin,dc=example,dc=com
  bind_password: "your-ldap-password"
  base_dn: dc=example,dc=com
  admin_group_dn: cn=admins,ou=groups,dc=example,dc=com
```

### 3. Run

**Docker Compose:**
```bash
docker-compose up -d
```

**Docker:**
```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  ghcr.io/kaedwen/ldap-manager:latest
```

**Binary:**
```bash
go install github.com/kaedwen/ldap-manager/cmd/ldap-manager@latest
ldap-manager -config config.yaml
```

### 4. Access

- Admin Interface: `http://localhost:8080/admin`
- Health Check: `http://localhost:9090/health` (internal port)

## Authentication Modes

### Internal Mode (Default)

ldap-manager handles authentication with its own login page:

```yaml
server:
  auth:
    mode: internal
```

- Admin login with LDAP credentials
- Session cookies with HMAC signing
- CSRF protection

### Proxy Mode (OIDC SSO)

Delegate authentication to an OIDC proxy (Authelia, Authentik, Keycloak):

```yaml
server:
  auth:
    mode: proxy
    header_user: Remote-User
    header_groups: Remote-Groups
    require_group: admins
```

- Single Sign-On with 2FA
- No login page (handled by proxy)
- Automatic session creation from headers

**Setup Guides:**
- [**OIDC Proxy Integration**](docs/OIDC_PROXY.md) - Generic guide for all proxies
- [**Authelia Example**](docs/authelia/) - Complete Authelia + Traefik setup

## Usage Workflow

### Admin Workflow

1. Admin logs in (or authenticates via SSO)
2. Searches for user by username or email
3. Generates reset token
4. Reset link displayed and optionally emailed
5. Admin shares link with user

### User Workflow

1. User receives reset link: `https://ldap.example.com/reset?token=abc123...`
2. Clicks link (no login required!)
3. Enters new password
4. Password updated in LDAP
5. Token consumed (single-use)

## Configuration

### Full Configuration Reference

See [`configs/config.yaml.example`](configs/config.yaml.example) for all options.

### Environment Variables

All settings can be overridden with `LDAP_MANAGER_` prefixed env vars:

```bash
LDAP_MANAGER_SERVER_PORT=8443
LDAP_MANAGER_SERVER_AUTH_MODE=proxy
LDAP_MANAGER_LDAP_URL=ldaps://ldap.example.com:636
LDAP_MANAGER_LDAP_BIND_PASSWORD_FILE=/run/secrets/ldap_password
```

### Docker Secrets

Mount password as file for Docker/Kubernetes secrets:

```yaml
services:
  ldap-manager:
    volumes:
      - /run/secrets/ldap_password:/run/secrets/ldap_password:ro
    environment:
      - LDAP_MANAGER_LDAP_BIND_PASSWORD_FILE=/run/secrets/ldap_password
```

The file is watched for changes and automatically reloaded.

## Health Checks

ldap-manager exposes health endpoints on a **separate port** (default: 9090):

- `GET /health` - Basic health check
- `GET /ready` - Readiness probe
- `GET /live` - Liveness probe

**Why separate port?**
- No authentication required
- No middleware overhead
- Safe for internal container checks
- Doesn't interfere with main routing/proxy

**Docker:**
```yaml
healthcheck:
  test: ["CMD-SHELL", "wget http://localhost:9090/health || exit 1"]
```

**Kubernetes:**
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 9090
```

## Deployment

See [docs/authelia/](docs/authelia/) for complete production deployment with Authelia + Traefik.

### Docker Compose

```yaml
version: '3.8'
services:
  ldap-manager:
    image: ghcr.io/kaedwen/ldap-manager:latest
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
    healthcheck:
      test: ["CMD-SHELL", "wget http://localhost:9090/health || exit 1"]
      interval: 30s
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ldap-manager
spec:
  template:
    spec:
      containers:
      - name: ldap-manager
        image: ghcr.io/kaedwen/ldap-manager:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: health
        livenessProbe:
          httpGet:
            path: /health
            port: 9090
        readinessProbe:
          httpGet:
            path: /ready
            port: 9090
        volumeMounts:
        - name: config
          mountPath: /app/config.yaml
          subPath: config.yaml
```

## Security

### Token Security
- 256-bit entropy (32 bytes from crypto/rand)
- SHA-256 hashed in LDAP (plaintext never stored)
- Time-limited (default: 3 days, configurable)
- Single-use (deleted after successful reset)
- HTTPS only in production

### Web Security
- CSRF tokens (crypto/rand, constant-time comparison)
- HMAC-signed session cookies (HttpOnly, Secure, SameSite=Strict)
- Rate limiting (configurable per-IP limits)
- Security headers (CSP, HSTS, X-Frame-Options, etc.)
- Password strength validation (12+ chars, complexity requirements)

### LDAP Security
- Supports both `ldap://` and `ldaps://` (LDAP over TLS)
- Service account with minimal privileges
- Admin verification via group membership
- LDAP filter injection prevention

## Development

```bash
# Build binary
go build -o ldap-manager ./cmd/ldap-manager

# Build Docker image
docker build -t ldap-manager:latest .

# Run tests
go test ./...
```

## Project Structure

```
ldap-manager/
├── cmd/ldap-manager/       # Main entry point
├── internal/
│   ├── config/            # Configuration management
│   ├── domain/            # Domain models
│   ├── handler/           # HTTP handlers
│   ├── middleware/        # HTTP middleware (auth, CSRF, rate limit)
│   ├── repository/        # LDAP repository
│   ├── service/           # Business logic
│   └── server/            # HTTP server
├── pkg/
│   ├── crypto/            # Token generation/hashing
│   └── validator/         # Password validation
├── web/
│   ├── static/            # CSS, JS
│   └── templates/         # HTML templates
├── configs/               # Configuration examples
└── docs/                  # Documentation
    ├── OIDC_PROXY.md     # Generic OIDC proxy guide
    └── authelia/          # Authelia-specific example
```

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

[MIT License](LICENSE)

## Credits

Built with Go 1.26+ using:
- [go-ldap/ldap](https://github.com/go-ldap/ldap) - LDAP client
- [fsnotify/fsnotify](https://github.com/fsnotify/fsnotify) - File watching
- [yaml.v3](https://github.com/go-yaml/yaml) - YAML parser
- Go standard library (net/http, crypto, html/template)
