# Lookout Repository Structure

This document describes the organization of the Lookout repository.

## Directory Structure

```
lookout/
├── docs/                       # All documentation
│   ├── KUBERNETES_DEPLOYMENT.md # Complete K8s guide (Kind, Gateway, ArgoCD, ALB)
│   ├── ARCHITECTURE.md         # System architecture
│   ├── DEVELOPMENT.md          # Development guide
│   ├── DOCKER_COMPOSE.md       # Docker Compose setup
│   ├── TLS_SETUP.md            # TLS certificate setup
│   ├── CI_CD.md                # CI/CD documentation
│   ├── REPOSITORY_STRUCTURE.md # This file
│   └── USAGE.md                # Usage guide
│
├── scripts/                    # Automation scripts
│   ├── deploy.sh               # Deployment script (staging/production)
│   ├── setup-registry.sh       # Docker registry setup for Kind
│   └── generate-certs.sh       # TLS certificate generation
│
├── helm/                       # Helm charts
│   └── lookout/
│       ├── Chart.yaml
│       ├── values.yaml         # Default values
│       ├── values.staging.yaml # Staging overrides
│       ├── values.production.yaml # Production overrides
│       └── templates/          # Kubernetes manifests
│
├── pkg/                        # Go packages
│   ├── cli/                    # CLI interface
│   ├── gui/                    # Web UI
│   ├── common/                 # Shared code
│   └── repository/             # Database layer
│
├── cmd/                        # Application entrypoints
│   ├── cli/                    # CLI binary
│   └── gui/                    # Web server binary
│
├── assets/                     # Web UI assets
│   ├── static/                 # CSS, JS, images
│   └── templates/              # HTML templates
│
└── examples/                   # Example files for testing
    ├── cyclonedx-sbom-example.json
    └── text-file-example.txt
```

## Key Documentation

### Deployment & Operations
- **[KUBERNETES_DEPLOYMENT.md](KUBERNETES_DEPLOYMENT.md)** - **START HERE** - Complete guide covering:
  - Chapter 1: Kind Cluster Setup
  - Chapter 2: Gateway API Setup (with fixed NodePorts)
  - Chapter 3: ArgoCD GitOps Setup
  - Chapter 4: AWS ALB Integration
- **[DOCKER_COMPOSE.md](DOCKER_COMPOSE.md)** - Local development with Docker Compose

### Development
- **[DEVELOPMENT.md](DEVELOPMENT.md)** - Development setup and guidelines
- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System architecture and design
- **[USAGE.md](USAGE.md)** - How to use Lookout (CLI and Web UI)

### Infrastructure
- **[TLS_SETUP.md](TLS_SETUP.md)** - TLS certificate configuration
- **[CI_CD.md](CI_CD.md)** - Continuous integration and deployment

## Deployment Scripts

### `scripts/deploy.sh`

Automates deployment to Kind cluster on EC2.

**Requirements:**
- Set `EC2_HOST` environment variable

**Usage:**
```bash
# Export EC2 host
export EC2_HOST=ubuntu@<your-ec2-ip>

# Deploy to staging
./scripts/deploy.sh staging

# Deploy to production (requires git tag)
git tag -a v1.0.0 -m "Release 1.0.0"
./scripts/deploy.sh production
```

**What it does:**
1. Syncs code to EC2
2. Builds Docker image
3. Tags and pushes to local registry
4. Deploys with Helm
5. Verifies deployment

### `scripts/setup-registry.sh`

Sets up HTTPS Docker registry for Kind cluster.

**Usage:**
```bash
# On EC2 instance
./scripts/setup-registry.sh
```

**What it does:**
1. Creates certificate directory
2. Generates self-signed TLS certificate
3. Starts registry with HTTPS
4. Connects to Kind network
5. Installs cert in Kind nodes
6. Restarts containerd

### `scripts/generate-certs.sh`

Generates self-signed TLS certificates for nginx.

**Usage:**
```bash
./scripts/generate-certs.sh
```

## Configuration Files

### Helm Values

- **values.yaml** - Base configuration
- **values.staging.yaml** - Staging overrides (lower resources, main branch)
- **values.production.yaml** - Production overrides (higher resources, git tags)

### Environment Variables

Configuration via `.env` file:
```bash
GO_VERSION=1.26.0
TRIVY_VERSION=0.69.1
NVD_API_KEY=<your-key>
DGRAPH_HOST=dgraph-alpha
DGRAPH_PORT=9080
```

For deployment:
```bash
EC2_HOST=ubuntu@<your-ec2-ip>  # Required for deploy.sh
```

## Security Notes

- All documentation uses placeholders (`<EC2_INSTANCE_IP>`) instead of hardcoded IPs
- `.env` file is gitignored and contains sensitive data
- TLS certificates are gitignored
- Deploy script requires explicit EC2_HOST configuration

## Getting Started

1. **Local Development:**
   ```bash
   ./scripts/generate-certs.sh
   docker-compose up -d
   ```

2. **Deploy to Staging:**
   ```bash
   export EC2_HOST=ubuntu@<your-ec2-ip>
   ./scripts/deploy.sh staging
   ```

3. **Deploy to Production:**
   ```bash
   git tag -a v1.0.0 -m "Release 1.0.0"
   git push origin v1.0.0
   ./scripts/deploy.sh production
   ```

## Related Documentation

- Main README: `../README.md`
- API Documentation: Run `godoc -http=:6060`
- Helm Chart: `helm/lookout/README.md`

## Contributing

See [DEVELOPMENT.md](DEVELOPMENT.md) for:
- Code style guidelines
- Testing requirements
- Build process
- Development workflow
