# OIDC Proxy Integration (Authelia, Authentik, Keycloak, etc.)

ldap-manager supports two authentication modes:

## Authentication Modes

### Mode 1: `internal` (Default)
- ldap-manager handles authentication directly
- Admin login page with LDAP credentials
- Session management with HMAC-signed cookies
- CSRF protection
- **Use when:** Running standalone or without SSO

### Mode 2: `proxy`
- Delegates authentication to an OIDC reverse proxy
- Supports: Authelia, Authentik, Keycloak, OAuth2 Proxy, and others
- Reads user/group information from HTTP headers
- Automatic session creation from headers
- No login page required (redirect handled by proxy)
- **Use when:** Running behind an OIDC-capable reverse proxy with SSO/2FA

## Configuration

### ldap-manager Config

**config.yaml:**
```yaml
server:
  auth:
    # Set to "proxy" to enable OIDC proxy mode
    mode: proxy
    
    # Trust headers from proxy (1 for Traefik, 2 for double-proxy)
    trusted_proxy_depth: 1
    
    # Customize header names for your proxy (defaults shown)
    header_user: Remote-User       # Authelia, Authentik default
    header_groups: Remote-Groups   # Authelia default
    header_email: Remote-Email     # Authelia, Authentik default
    header_name: Remote-Name       # Display name
    
    # Required group for admin access (case-insensitive)
    require_group: admins
```

**Environment Variables:**
```bash
LDAP_MANAGER_SERVER_AUTH_MODE=proxy
LDAP_MANAGER_SERVER_AUTH_HEADER_USER=X-Forwarded-User  # For OAuth2 Proxy
LDAP_MANAGER_SERVER_AUTH_REQUIRE_GROUP=ldap-admins
```

### Traefik Configuration

**docker-compose.yml:**
```yaml
services:
  ldap-manager:
    image: ghcr.io/kaedwen/ldap-manager:latest
    environment:
      - LDAP_MANAGER_SERVER_AUTH_MODE=proxy
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.ldap-manager.rule=Host(`ldap.example.com`)"

      # Admin routes - protected with ForwardAuth
      - "traefik.http.routers.ldap-manager-admin.rule=Host(`ldap.example.com`) && PathPrefix(`/admin`)"
      - "traefik.http.routers.ldap-manager-admin.middlewares=authelia@docker"
      - "traefik.http.routers.ldap-manager-admin.priority=100"

      # Public routes - no authentication
      - "traefik.http.routers.ldap-manager-public.rule=Host(`ldap.example.com`) && (PathPrefix(`/reset`) || PathPrefix(`/static`) || Path(`/health`))"
      - "traefik.http.routers.ldap-manager-public.priority=200"
```

## Proxy-Specific Configuration

### Authelia

**Access Control Rules:**
```yaml
access_control:
  rules:
    # Public routes
    - domain: ldap.example.com
      policy: bypass
      resources:
        - "^/reset([?/].*)?$"
        - "^/static/.*$"
        - "^/health$"

    # Admin routes - 2FA required
    - domain: ldap.example.com
      policy: two_factor
      resources:
        - "^/admin.*$"
      subject:
        - "group:admins"
```

**Headers (default):**
- `Remote-User`: username
- `Remote-Groups`: comma-separated groups
- `Remote-Email`: email address
- `Remote-Name`: display name

### Authentik

**Provider Configuration:**
- Set up OAuth2/OpenID Provider
- Configure Forward Auth (Proxy Provider)
- Map user attributes to headers

**ldap-manager config:**
```yaml
server:
  auth:
    mode: proxy
    header_user: X-Authentik-Username  # Authentik default
    header_groups: X-Authentik-Groups
    header_email: X-Authentik-Email
    header_name: X-Authentik-Name
```

### OAuth2 Proxy

**OAuth2 Proxy config:**
```
--set-xauthrequest=true
--pass-user-headers=true
```

**ldap-manager config:**
```yaml
server:
  auth:
    mode: proxy
    header_user: X-Forwarded-User
    header_groups: X-Forwarded-Groups
    header_email: X-Forwarded-Email
    header_name: X-Forwarded-Preferred-Username
```

### Keycloak (via Gatekeeper/Louketo)

**ldap-manager config:**
```yaml
server:
  auth:
    mode: proxy
    header_user: X-Auth-Username
    header_groups: X-Auth-Groups
    header_email: X-Auth-Email
    header_name: X-Auth-Name
```

## Architecture

```
Internet
   │
   ├─> /admin/*   → OIDC Proxy (2FA) → ldap-manager [Protected]
   │                     ↓
   │              Injects Headers:
   │              - Remote-User: johndoe
   │              - Remote-Groups: admins
   │              - Remote-Email: johndoe@example.com
   │
   ├─> /reset     → ldap-manager                     [Public]
   ├─> /static/*  → ldap-manager                     [Public]
   └─> /health    → ldap-manager                     [Public]
```

## How It Works

### Proxy Mode Flow

1. **User accesses `/admin`**
   - Traefik forwards to OIDC proxy (Authelia/Authentik/etc.)
   - User authenticates with username + password + 2FA
   - Proxy validates user is in `admins` group
   - Proxy injects headers: `Remote-User`, `Remote-Groups`, etc.
   
2. **ldap-manager receives request with headers**
   - Validates `Remote-User` header is present
   - Checks `Remote-Groups` contains required group
   - Creates session automatically
   - User accesses admin dashboard
   
3. **User generates reset link**
   - Link: `https://ldap.example.com/reset?token=abc123`
   - Sent to end user via email or copy/paste
   
4. **End user clicks reset link**
   - Accesses `/reset` - public route, no auth
   - Token validated by ldap-manager
   - Sets new password

### Security Benefits

- ✅ **2FA/MFA**: Enforced by OIDC proxy
- ✅ **SSO**: Single authentication across multiple apps
- ✅ **Centralized Access Control**: Manage users in one place
- ✅ **Audit Logging**: Proxy logs all authentication attempts
- ✅ **Password Reset Stays Public**: Users can reset even when locked out

## Troubleshooting

### Admin can't access `/admin`

**Check headers are being forwarded:**
```bash
# Test from inside ldap-manager container
curl -H "Remote-User: johndoe" \
     -H "Remote-Groups: admins" \
     http://localhost:8080/admin/dashboard
```

**Check ldap-manager logs:**
```
proxy auth: no user header found header=Remote-User
```
→ Headers not being forwarded by proxy

**Solution:** Verify ForwardAuth middleware is configured correctly

### User not in required group

**Log output:**
```
proxy auth: user not in required group user=johndoe groups=users,developers required_group=admins
```

**Solutions:**
1. Add user to `admins` group in your OIDC provider
2. Change `require_group` in ldap-manager config
3. Verify group name matches exactly (case-insensitive)

### Reset link requires authentication

**Problem:** `/reset` redirects to login page

**Solution:** Ensure public routes have higher priority and bypass policy:
```yaml
# Traefik
- "traefik.http.routers.ldap-manager-public.priority=200"  # Higher than admin

# Authelia
- domain: ldap.example.com
  policy: bypass
  resources:
    - "^/reset([?/].*)?$"
```

### Headers not visible in ldap-manager

**Check Traefik ForwardAuth:**
```yaml
middlewares:
  authelia:
    forwardAuth:
      address: http://authelia:9091/api/authz/forward-auth
      trustForwardHeader: true
      authResponseHeaders:
        - Remote-User
        - Remote-Groups
        - Remote-Email
        - Remote-Name
```

## Switching Between Modes

### From `internal` to `proxy`

1. Deploy behind OIDC proxy with ForwardAuth
2. Update config: `mode: proxy`
3. Restart ldap-manager
4. Login page will be disabled
5. `/admin` redirects to proxy login

### From `proxy` to `internal`

1. Update config: `mode: internal`
2. Restart ldap-manager
3. Login page will be re-enabled
4. Users authenticate directly with LDAP

**Note:** Sessions are compatible between modes (same cookie format)

## Complete Example: Authelia + Traefik

See `AUTHELIA_INTEGRATION.md` for a complete working example with:
- Full Traefik labels
- Authelia access control rules
- LDAP integration
- Testing procedures
