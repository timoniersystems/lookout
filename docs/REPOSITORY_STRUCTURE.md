# Lookout Repository Structure

This document describes the organization of the Lookout repository.

## Directory Structure

```
lookout/
├── docs/                       # All documentation
│   ├── KUBERNETES_SETUP.md     # Complete K8s guide (Kind, Gateway, ArgoCD, ALB)
│   ├── ARCHITECTURE.md         # System architecture
│   ├── DOCKER_COMPOSE.md       # Docker Compose setup
│   ├── TLS_SETUP.md            # TLS certificate setup
│   ├── CI_CD.md                # CI/CD documentation
│   ├── REPOSITORY_STRUCTURE.md # This file
│   └── USAGE.md                # Usage guide
│
├── scripts/                    # Automation scripts
│   ├── create-ghcr-secret.sh   # Create GHCR pull secret in a namespace
│   ├── deploy.sh               # Legacy deployment script
│   ├── generate-certs.sh       # TLS certificate generation for nginx
│   ├── generate-favicon.py     # Favicon generator
│   ├── setup-alb.sh            # AWS ALB setup (staging + production)
│   ├── setup-argocd.sh         # ArgoCD installation
│   ├── setup-argocd-github-repo.sh    # ArgoCD GitHub repo connection
│   ├── setup-argocd-image-updater.sh  # ArgoCD Image Updater installation
│   ├── setup-basic-auth.sh     # Basic auth setup (configurable per env)
│   ├── setup-external-secrets.sh      # External Secrets Operator setup
│   ├── setup-fixed-nodeports.sh       # Gateway fixed NodePort config
│   ├── setup-gateway.sh        # Envoy Gateway setup
│   ├── setup-health-httproute.sh      # ALB health check route
│   ├── setup-kind-nodeport-forwarding.sh  # Kind NodePort forwarding
│   ├── setup-registry.sh       # Docker registry setup for Kind
│   └── sync-to-ec2.sh          # Rsync code to EC2 instance
│
├── helm/                       # Helm charts
│   └── lookout/
│       ├── Chart.yaml
│       ├── README.md            # Helm chart documentation
│       ├── values.yaml          # Default values
│       ├── values.staging.yaml  # Staging: main branch image, shared gateway, basic auth
│       ├── values.production.yaml # Production: semver tags, cross-namespace gateway, basic auth
│       └── templates/
│           ├── _helpers.tpl
│           ├── gateway.yaml
│           ├── httproute.yaml
│           ├── securitypolicy.yaml
│           ├── externalsecret-basic-auth.yaml
│           └── NOTES.txt
│
├── k8s/                        # Kubernetes manifests
│   └── argocd/
│       ├── staging-application.yaml    # ArgoCD staging app (digest image updater)
│       └── production-application.yaml # ArgoCD production app (semver image updater)
│
├── cmd/                        # Application entrypoints
│   ├── cli/                    # CLI binary
│   └── ui/                     # Web server binary
│
├── pkg/                        # Go packages
│   ├── cli/
│   │   └── cli_processor/      # CVE formatting and output
│   ├── common/
│   │   ├── cyclonedx/          # CycloneDX SBOM parsing
│   │   ├── spdx/               # SPDX SBOM parsing
│   │   ├── fileutil/           # File utilities
│   │   ├── handler/            # HTTP handlers (upload, results, progress)
│   │   ├── nvd/                # NVD API client
│   │   ├── processor/          # File processing
│   │   ├── progress/           # Progress tracking (SSE)
│   │   └── trivy/              # Trivy integration
│   ├── config/                 # Configuration management
│   ├── graph/                  # Dgraph database operations
│   ├── interfaces/             # Interface definitions
│   ├── logging/                # Structured logging
│   ├── repository/             # Data access layer
│   ├── service/                # Business logic layer
│   ├── ui/
│   │   └── echo/               # Echo server setup
│   └── validation/             # Input validation
│
├── assets/                     # Web UI assets
│   ├── static/                 # CSS, JS, images
│   └── templates/              # HTML templates
│
├── nginx/                      # Nginx reverse proxy config (Docker Compose)
│
├── examples/                   # Example files for testing
│   ├── cyclonedx-sbom-example.json
│   ├── spdx-npm-example.spdx.json
│   ├── spdx-debian-example.spdx.json
│   ├── spdx-maven-example.spdx.json
│   ├── spdx-appbomination-example.spdx.json
│   ├── trivy-results-example.json
│   ├── text-file-example.txt
│   └── ... (additional CycloneDX/SPDX samples)
│
├── .github/
│   ├── workflows/
│   │   ├── ci.yml              # Test, lint, build, security scan
│   │   ├── coverage.yml        # Coverage reports and badge
│   │   ├── docker.yml          # Docker build, Trivy scan, push to GHCR
│   │   └── release.yml         # Binary releases and Docker publish
│   └── dependabot.yml          # Dependency update automation
│
├── Dockerfile                  # Multi-stage build (Go + Trivy DB)
├── docker-compose.yml          # Local dev stack (Dgraph, nginx, app)
├── Makefile                    # Build, test, install targets
├── .golangci.yml               # Linter configuration
└── go.mod / go.sum             # Go module dependencies
```

## Key Documentation

### Deployment & Operations
- **[KUBERNETES_SETUP.md](KUBERNETES_SETUP.md)** - Complete guide: Kind cluster, Gateway API, ArgoCD GitOps, AWS ALB, production deployment
- **[DOCKER_COMPOSE.md](DOCKER_COMPOSE.md)** - Local development with Docker Compose

### Development
- **[CONTRIBUTING.md](../CONTRIBUTING.md)** - Development setup and contribution guidelines
- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System architecture and design
- **[USAGE.md](USAGE.md)** - How to use Lookout (CLI and Web UI)

### Infrastructure
- **[TLS_SETUP.md](TLS_SETUP.md)** - TLS certificate configuration (Docker Compose)
- **[CI_CD.md](CI_CD.md)** - GitHub Actions workflows and releases

## Getting Started

1. **Local Development (Docker Compose):**
   ```bash
   make certs
   make up
   # Access at https://localhost:7443
   ```

2. **Kubernetes Staging:**
   Managed by ArgoCD. Push to `main` triggers automatic deployment via image digest strategy.

3. **Kubernetes Production:**
   ```bash
   # Tag a release to trigger production deployment
   git tag -a v1.0.0 -m "Release 1.0.0"
   git push origin v1.0.0
   ```
   ArgoCD Image Updater (semver strategy) detects the new tag and deploys automatically.

## Related Documentation

- Main README: `../README.md`
- Helm Chart: `../helm/lookout/README.md`
- API Documentation: Run `godoc -http=:6060`

## Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for code style, testing, and development workflow.
