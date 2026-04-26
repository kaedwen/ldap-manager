# LDAP Manager Deployment

Podman Quadlet deployment files for production server `srv-01`.

## Files

- `ldap-manager.container` - Podman Quadlet systemd unit file
- `config.yaml` - Production configuration
- `deploy.sh` - Automated deployment script
- `authelia-rules.yml` - Authelia access control rules

## Quick Deploy

```bash
cd deployment
./deploy.sh
```

## Manual Deployment

### 1. Create Config Directory

```bash
ssh srv-01
sudo mkdir -p /var/mnt/storage/config-001/ldap-manager
sudo chown core:core /var/mnt/storage/config-001/ldap-manager
```

### 2. Copy Configuration

```bash
scp config.yaml srv-01:/var/mnt/storage/config-001/ldap-manager/config.yaml
```

### 3. Create Secrets

```bash
ssh srv-01

# Generate session secret
openssl rand -base64 32 | podman secret create ldap_manager_session_secret -

# Verify LDAP admin password secret exists (shared with Authelia)
podman secret ls | grep ldap_admin_password
```

### 4. Deploy Quadlet

```bash
scp ldap-manager.container srv-01:/tmp/
ssh srv-01 'sudo mv /tmp/ldap-manager.container /etc/containers/systemd/ && \
            sudo chown root:root /etc/containers/systemd/ldap-manager.container && \
            sudo chmod 644 /etc/containers/systemd/ldap-manager.container'
```

### 5. Start Service

```bash
ssh srv-01
sudo systemctl daemon-reload
sudo systemctl enable ldap-manager.service
sudo systemctl start ldap-manager.service
```

### 6. Verify

```bash
ssh srv-01

# Check service status
sudo systemctl status ldap-manager.service

# Check health
podman exec ldap-manager wget -qO- http://localhost:9090/health

# View logs
sudo journalctl -u ldap-manager.service -f
```

## Authelia Integration

Update `/var/mnt/storage/config-001/authelia/configuration.yml`:

```yaml
access_control:
  rules:
    # LDAP Manager - Public password reset
    - domain: passwd.heinrich.blue
      resources:
        - '^/reset.*$'
        - '^/static/.*$'
      policy: bypass
    
    # LDAP Manager - Admin interface (requires auth + 2FA)
    - domain: passwd.heinrich.blue
      resources:
        - '^/admin.*$'
      policy: two_factor
      subject:
        - 'group:admins'
```

Restart Authelia:
```bash
ssh srv-01 'sudo systemctl restart authelia.service'
```

## Testing

### Public Access (No Auth)
```bash
curl https://passwd.heinrich.blue/reset
```

### Admin Access (Requires Auth)
```bash
# Should redirect to auth.heinrich.blue
curl -I https://passwd.heinrich.blue/admin/dashboard
```

### Health Check
```bash
ssh srv-01 'podman exec ldap-manager wget -qO- http://localhost:9090/health'
```

## Troubleshooting

### View Logs
```bash
ssh srv-01 'sudo journalctl -u ldap-manager.service -f'
```

### Restart Service
```bash
ssh srv-01 'sudo systemctl restart ldap-manager.service'
```

### Check Networks
```bash
ssh srv-01 'podman network inspect base.network | grep ldap-manager'
ssh srv-01 'podman network inspect openldap.network | grep ldap-manager'
```

### Exec Into Container
```bash
ssh srv-01 'podman exec -it ldap-manager /bin/sh'
```

Note: Container uses scratch image, so no shell available. Use `wget` for debugging:
```bash
ssh srv-01 'podman exec ldap-manager wget -qO- http://localhost:9090/health'
```

## Rollback

```bash
ssh srv-01
sudo systemctl stop ldap-manager.service
sudo systemctl disable ldap-manager.service
sudo rm /etc/containers/systemd/ldap-manager.container
sudo systemctl daemon-reload
podman rm -f ldap-manager
```

## Configuration

### Environment Variables

All settings can be overridden via environment variables:

```ini
Environment=LDAP_MANAGER_SERVER_PORT=8080
Environment=LDAP_MANAGER_SERVER_AUTH_MODE=proxy
Environment=LDAP_MANAGER_LDAP_URL=ldap://openldap:389
```

### Secrets

Secrets are injected as environment variables:
- `ldap_manager_session_secret` → `LDAP_MANAGER_SERVER_SESSION_SECRET`
- `ldap_admin_password` → `LDAP_MANAGER_LDAP_BIND_PASSWORD`

### Networks

The container joins two networks:
- `base.network` - For Traefik routing
- `openldap.network` - For LDAP access

### Health Check

Internal health endpoint on port 9090:
- `/health` - Basic health check
- `/ready` - Readiness probe
- `/live` - Liveness probe

## Architecture

```
Internet
  ↓
Traefik (TLS termination, port 443)
  ↓
  ├─→ Authelia (auth.heinrich.blue)
  │     ↓
  │   Protected Routes (/admin/*)
  │     ↓
  └─→ ldap-manager (passwd.heinrich.blue)
        ↓
      Public Routes (/reset)
```

## Domain

- **Public**: `https://passwd.heinrich.blue/reset` - No authentication
- **Admin**: `https://passwd.heinrich.blue/admin/*` - Requires Authelia SSO + 2FA

## Security

- TLS handled by Traefik with Let's Encrypt
- Session secrets via Podman secrets
- LDAP password via Podman secrets
- Authelia SSO with 2FA for admin access
- Public reset endpoint requires valid token
- Rate limiting per IP
- CSRF protection on forms
- Security headers (CSP, HSTS, etc.)
