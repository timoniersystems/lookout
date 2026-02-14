# Build arguments for version control
ARG GO_VERSION=1.26.0
ARG ALPINE_VERSION=3.23
ARG TRIVY_VERSION=0.69.1

# Build stage - builds for native platform automatically
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

# Install ca-certificates for HTTPS requests during build
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary for native platform - Docker automatically uses the host architecture
# CGO_ENABLED=0 creates a fully static binary compatible with distroless
# This works on both ARM64 (Mac) and AMD64 (Linux) without any flags
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o lookout-ui ./cmd/ui

# Trivy installation stage
FROM alpine:${ALPINE_VERSION} AS trivy-installer

ARG TRIVY_VERSION=0.69.1

RUN apk add --no-cache curl tar && \
    TRIVY_VER="${TRIVY_VERSION}" && \
    ARCH=$(uname -m) && \
    case "$ARCH" in \
        x86_64) TRIVY_ARCH="64bit" ;; \
        aarch64|arm64) TRIVY_ARCH="ARM64" ;; \
        *) echo "Unsupported architecture: $ARCH" && exit 1 ;; \
    esac && \
    echo "Installing Trivy ${TRIVY_VER} for ${TRIVY_ARCH}" && \
    curl -sfL "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VER}/trivy_${TRIVY_VER}_Linux-${TRIVY_ARCH}.tar.gz" | \
    tar -xz -C /usr/local/bin trivy && \
    chmod +x /usr/local/bin/trivy && \
    trivy --version

# Runtime stage - distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# Copy CA certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy Trivy binary
COPY --from=trivy-installer /usr/local/bin/trivy /usr/local/bin/trivy

# Copy UI binary from builder
COPY --from=builder /app/lookout-ui /app/lookout-ui

WORKDIR /app

# Expose application port
EXPOSE 3000

# distroless doesn't support shell-based healthchecks
# Health checks should be configured in docker-compose.yml or kubernetes

# Run the UI application (web server)
# distroless/static:nonroot already runs as non-root user (uid 65532)
CMD ["/app/lookout-ui"]
