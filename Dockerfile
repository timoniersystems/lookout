# Build arguments for version control
ARG GO_VERSION=1.26.0
ARG TRIVY_VERSION=0.69.1

# Build stage - builds for native platform automatically using Debian-based image
FROM golang:${GO_VERSION}-bookworm AS builder

ARG TRIVY_VERSION

# Install Trivy in the builder stage (curl is already in golang image)
RUN TRIVY_VER="${TRIVY_VERSION}" && \
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

# Download Trivy vulnerability database at build time so it's baked into the image
# This avoids runtime DB downloads in the distroless container
RUN trivy filesystem --download-db-only --cache-dir /trivy-cache

WORKDIR /app

# Tell Go this module is private (don't try to fetch from GitHub)
ENV GOPRIVATE=github.com/timoniersystems/lookout

# Copy everything
COPY . .

# Build static binary (dependencies downloaded automatically during build)
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o lookout-ui ./cmd/ui

# Runtime stage - distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# Copy CA certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy Trivy binary from builder with correct ownership
COPY --from=builder --chown=65532:65532 /usr/local/bin/trivy /usr/local/bin/trivy

# Copy pre-downloaded Trivy vulnerability database to a path NOT overlaid by emptyDir mounts
COPY --from=builder --chown=65532:65532 /trivy-cache /opt/trivy-cache

# Tell Trivy where to find its cache and skip DB updates at runtime
ENV TRIVY_CACHE_DIR=/opt/trivy-cache
ENV TRIVY_SKIP_DB_UPDATE=true

# Set working directory first
WORKDIR /app

# Copy UI binary from builder with correct ownership for nonroot user
COPY --from=builder --chown=65532:65532 /app/lookout-ui /app/lookout-ui

# Expose application port
EXPOSE 3000

# distroless doesn't support shell-based healthchecks
# Health checks should be configured in docker-compose.yml or kubernetes

# Run the UI application (web server)
# distroless/static:nonroot already runs as non-root user (uid 65532)
CMD ["/app/lookout-ui"]
