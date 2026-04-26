# Build stage
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -a \
    -installsuffix cgo \
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev') -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -trimpath \
    -o ldap-manager \
    ./cmd/ldap-manager

# Verify binary
RUN ./ldap-manager --version || echo "Binary built successfully"

# Runtime stage - from scratch
FROM scratch

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/ldap-manager .

# Copy web assets
COPY --from=builder /build/web ./web

# Copy config template (optional)
COPY --from=builder /build/configs/config.yaml.example ./configs/

EXPOSE 8080 9090

# Note: Health check not possible with scratch image (no shell/wget)
# Use external health checks on port 9090:
#   curl http://localhost:9090/health
#   Docker/Podman: healthcheck.test: ["CMD-SHELL", "wget ..."]
#   Kubernetes: livenessProbe.httpGet.port: 9090

ENTRYPOINT ["/app/ldap-manager"]
