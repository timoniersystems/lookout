# syntax=docker/dockerfile:1
# Build arguments for version control
ARG GO_VERSION=1.26.1
ARG TRIVY_BASE_IMAGE=ghcr.io/timoniersystems/lookout/trivy-base:latest

# Trivy stage - copy binary and pre-downloaded DB from the base image
FROM ${TRIVY_BASE_IMAGE} AS trivy

# Build stage - builds for native platform automatically using Debian-based image
FROM golang:${GO_VERSION}-bookworm AS builder

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

# Copy Trivy binary and pre-downloaded DB from the trivy-base image
COPY --from=trivy --chown=65532:65532 /usr/local/bin/trivy /usr/local/bin/trivy
COPY --from=trivy --chown=65532:65532 /root/.cache/trivy /opt/trivy-cache

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
