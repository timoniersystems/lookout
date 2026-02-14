# Build arguments for version control
ARG GO_VERSION=1.26.0
ARG ALPINE_VERSION=3.23
ARG TRIVY_VERSION=0.58.2

# Build stage - builds for native platform automatically
FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder

# Install build dependencies
RUN apk add --no-cache \
    gcc \
    musl-dev \
    alpine-sdk \
    ca-certificates

WORKDIR /app

# Copy dependency files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build for native platform - Docker automatically uses the host architecture
# This works on both ARM64 (Mac) and AMD64 (Linux) without any flags
RUN CGO_ENABLED=1 go build -o lookout-ui ./cmd/ui

# Runtime stage
ARG ALPINE_VERSION=3.21
FROM alpine:${ALPINE_VERSION}

# Re-declare TRIVY_VERSION for this stage
ARG TRIVY_VERSION=0.58.2

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    curl \
    tar \
    tzdata

# Install Trivy (with architecture detection)
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

# Create non-root user
RUN addgroup -g 1000 lookout && \
    adduser -D -u 1000 -G lookout lookout

# Create necessary directories
RUN mkdir -p /app/outputs && \
    chown -R lookout:lookout /app

WORKDIR /app

# Copy UI binary from builder and ensure it's executable
COPY --from=builder --chown=lookout:lookout --chmod=755 /app/lookout-ui /app/lookout-ui

# Switch to non-root user
USER lookout

# Expose application port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:3000/health || exit 1

# Run the UI application (web server)
CMD ["/app/lookout-ui"]
