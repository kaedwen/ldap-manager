# Health Check Binary

A minimal static binary for Docker health checks that works with `FROM scratch` images.

## Features

- **Static binary** - No dependencies, works in scratch containers
- **Small size** - ~1.5 MB compressed
- **Fast** - 2-second timeout, minimal overhead
- **Configurable** - Set custom health URL via environment variable

## Usage

### Default behavior

```bash
/app/healthcheck
```

Checks `http://localhost:9090/health` by default.

### Custom health URL

```bash
HEALTH_URL=http://localhost:8080/custom /app/healthcheck
```

### Exit codes

- `0` - Health check passed (HTTP 200)
- `1` - Health check failed (connection error, non-200 status, timeout)

## Docker Integration

The health check binary is automatically included in the Docker image:

```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/healthcheck"]
```

## Why a separate binary?

- **No shell required** - Works with `FROM scratch` base images
- **No external tools** - No need for wget, curl, or other utilities
- **Smaller attack surface** - Minimal dependencies
- **Better performance** - Native Go binary vs. shell scripts

## Alternative approaches

If you prefer using external health check methods:

### Docker Compose
```yaml
healthcheck:
  test: ["CMD", "/app/healthcheck"]
```

### Kubernetes
```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 9090
  initialDelaySeconds: 5
  periodSeconds: 30
```

### Docker CLI
```bash
docker run --health-cmd="/app/healthcheck" --health-interval=30s myimage
```
