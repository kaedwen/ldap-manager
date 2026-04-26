# LDAP Manager with Authelia Integration

This setup protects the admin interface with Authelia 2FA while keeping the password reset functionality public.

## Architecture

```
Internet
   │
   ├─> /admin/*        → Authelia (2FA) → ldap-manager  [Protected]
   ├─> /reset          → ldap-manager                   [Public]
   ├─> /static/*       → ldap-manager                   [Public]
   └─> /health         → ldap-manager                   [Public]
```

## Authentication Modes

ldap-manager supports two authentication modes, configurable via `server.auth.mode`:

### Mode 1: `internal` (Default)
- ldap-manager handles authentication directly
- Admin login page with LDAP credentials
- Session management with HMAC-signed cookies
- CSRF protection
- **Use when:** Running standalone or without SSO

### Mode 2: `authelia`
- Delegates authentication to Authelia
- Reads `Remote-User` and `Remote-Groups` headers
- Automatic session creation from headers
- No login page (redirects to Authelia)
- **Use when:** Running behind Authelia with Traefik

## Configuration

### 1. ldap-manager Config

**config.yaml:**
```yaml
server:
  auth:
    # Set to "authelia" to enable SSO mode
    mode: authelia
    # Trust headers from proxy (1 for Traefik, 2 for double-proxy)
    trusted_proxy_depth: 1
```

**Environment Variable:**
```bash
LDAP_MANAGER_SERVER_AUTH_MODE=authelia
```

### 2. Traefik Labels (in docker-compose.yml)

The setup uses **router priority** to distinguish between protected and public routes:
- Priority 200: Public routes (reset, static, health)
- Priority 100: Admin routes (with Authelia)
- Higher priority = evaluated first

### 3. Authelia Rules (authelia-rules.yml)

The setup uses **router priority** to distinguish between protected and public routes:
- Priority 200: Public routes (reset, static, health)
- Priority 100: Admin routes (with Authelia)
- Higher priority = evaluated first

### 2. Authelia Rules (authelia-rules.yml)

Copy the content of `authelia-rules.yml` into your Authelia configuration under `access_control.rules`.

**Rules:**
- `/reset`, `/static`, `/health` → `policy: bypass` (public)
- `/admin/*` → `policy: two_factor` + `group:admins` (protected)

### 3. LDAP Integration

Authelia should be configured to use the same LDAP server:

```yaml
authentication_backend:
  ldap:
    address: ldap://ldap.example.com:389
    base_dn: dc=example,dc=com
    username_attribute: uid
    users_filter: (&(objectClass=inetOrgPerson)(uid={input}))
    groups_filter: (&(objectClass=groupOfNames)(member={dn}))
    group_name_attribute: cn
    mail_attribute: mail
    display_name_attribute: cn
    additional_users_dn: ou=users
    additional_groups_dn: ou=groups
    user: cn=admin,dc=example,dc=com
    password: <LDAP_ADMIN_PASSWORD>
```

## Deployment

### Using Docker Compose

```bash
# Deploy
docker-compose up -d

# Check logs
docker-compose logs -f ldap-manager

# Stop
docker-compose down
```

### Using Podman Systemd

```bash
# Generate systemd unit
podman generate systemd --new --name ldap-manager > /etc/containers/systemd/ldap-manager.container

# Start service
systemctl daemon-reload
systemctl enable --now ldap-manager.service
```

## Usage Flow

### Admin Workflow
1. Admin navigates to `https://ldap.example.com/admin`
2. Redirected to Authelia login
3. Enters username + password (LDAP auth)
4. Completes 2FA (TOTP, WebAuthn, etc.)
5. Access granted to admin dashboard
6. Searches for user and generates reset link
7. Shares link with user (email or copy/paste)

### User Workflow
1. User receives reset link: `https://ldap.example.com/reset?token=abc123...`
2. Clicks link → **Direct access** (no Authelia login)
3. Token validated by ldap-manager
4. Enters new password
5. Password updated in LDAP

## Security Considerations

- **2FA Required**: Admins must complete two-factor authentication
- **Group-based Access**: Only members of `cn=admins,ou=groups,dc=example,dc=com`
- **Token Security**: Reset tokens are SHA-256 hashed and time-limited (3 days default)
- **Public Reset**: Users don't need authentication (they can't log in with forgotten password!)
- **HTTPS**: Traefik enforces TLS via Let's Encrypt

## Troubleshooting

### Admin can't access /admin
- Check Authelia logs: `docker logs authelia`
- Verify user is in `admins` group in LDAP
- Check Traefik routing: `docker logs traefik | grep ldap-manager`

### Reset link not accessible
- Verify public router priority is higher (200 > 100)
- Check Traefik dashboard for router order
- Test health endpoint: `curl https://ldap.example.com/health`

### Authelia keeps asking for auth on /reset
- Check `access_control.rules` - ensure `/reset` has `policy: bypass`
- Verify Authelia config reload: `docker restart authelia`
- Clear browser cookies/cache

## Integration with Existing Authelia

If you already have Authelia running, just add the rules to your existing `configuration.yml`:

```yaml
access_control:
  rules:
    # ... your existing rules ...

    # Add these for ldap-manager
    - domain: ldap.example.com
      policy: bypass
      resources:
        - "^/reset([?/].*)?$"
        - "^/static/.*$"
        - "^/health$"

    - domain: ldap.example.com
      policy: two_factor
      resources:
        - "^/admin.*$"
      subject:
        - "group:admins"
```

Then update your Traefik labels to point to your existing Authelia instance.
