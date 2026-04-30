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
    -ldflags="-w -s -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev') -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -trimpath \
    -o ldap-manager \
    ./cmd/ldap-manager

# Build healthcheck binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o healthcheck \
    ./cmd/healthcheck

# Verify binary
RUN ./ldap-manager --version || echo "Binary built successfully"

# Runtime stage - from scratch
FROM scratch

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/ldap-manager .
COPY --from=builder /build/healthcheck .

# Copy web assets
COPY --from=builder /build/web ./web

# Copy config template (optional)
COPY --from=builder /build/configs/config.yaml.example ./configs/

EXPOSE 8080 9090

# Health check using our static binary
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/healthcheck"]

ENTRYPOINT ["/app/ldap-manager"]
