# Lookout

> SBOM (CycloneDX & SPDX) and CVE vulnerability analysis tool with dependency path tracing

[![CI](https://github.com/<username>/lookout/actions/workflows/ci.yml/badge.svg)](https://github.com/<username>/lookout/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/<username>/lookout)](https://goreportcard.com/report/github.com/<username>/lookout)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## What is Lookout?

Lookout helps you understand and fix vulnerabilities in your software dependencies. It answers critical questions:

- 📊 **What vulnerabilities exist in my software?**
- 🔍 **How did this vulnerable package get into my project?**
- 🛠️ **Which direct dependency should I upgrade to fix it?**

### Key Features

- **CVE Analysis** - Fetch detailed vulnerability data from the NVD with rate limiting and retry logic
- **SBOM Scanning** - Scan Software Bill of Materials with Trivy integration
- **Dependency Path Tracing** - Trace vulnerable transitive dependencies back to your root package
- **Multi-Interface** - CLI for automation, Web UI with real-time progress tracking
- **Graph Database** - Dgraph-powered dependency graph visualization
- **Async Processing** - Background SBOM processing with SSE progress updates
- **TLS/HTTPS** - Secure communication via nginx reverse proxy
- **Session Management** - Results storage with auto-expiration
- **Severity Filtering** - Filter vulnerabilities by CRITICAL, HIGH, MEDIUM, LOW

## Quick Start

### CLI Usage

```bash
# Clone and build
git clone https://github.com/<username>/lookout.git
cd lookout
make build && make install

# Fetch CVE data
lookout -cve CVE-2021-44228

# Scan an SBOM
lookout -sbom examples/cyclonedx-sbom-example.json

# Trace dependency path (requires Dgraph)
docker-compose --profile standalone up -d dgraph
export DGRAPH_HOST=localhost
lookout -sbom examples/cyclonedx-sbom-example.json \
  -dep-path 'pkg:composer/asm89/stack-cors@1.3.0'
```

### Web UI

```bash
# Generate TLS certificates
./scripts/generate-certs.sh

# Start all services
docker-compose up -d

# Access UI (HTTPS)
open https://localhost:7443

# Access Dgraph Ratel
open http://localhost:8000
```

## Installation

### Using Docker (Recommended)

```bash
# Generate TLS certificates
./scripts/generate-certs.sh

# Start all services
docker-compose up -d
```

Access points:
- Lookout Web UI: https://localhost:7443 (HTTPS) or http://localhost:7080 (redirects to HTTPS)
- Dgraph Ratel UI: http://localhost:8000
- Dgraph API: http://localhost:8080

See [Docker Compose Guide](docs/DOCKER_COMPOSE.md) for detailed setup and configuration.

### Binary Download

Download from [Releases](https://github.com/<username>/lookout/releases):

```bash
# Linux
wget https://github.com/<username>/lookout/releases/latest/download/lookout-linux-amd64
chmod +x lookout-linux-amd64
sudo mv lookout-linux-amd64 /usr/local/bin/lookout

# macOS (Apple Silicon)
wget https://github.com/<username>/lookout/releases/latest/download/lookout-darwin-arm64
chmod +x lookout-darwin-arm64
sudo mv lookout-darwin-arm64 /usr/local/bin/lookout

# Verify
lookout -version
```

### Build from Source

**Requirements:**
- Go 1.21+
- Docker & Docker Compose (for UI and dependency tracing)
- Trivy (optional, for SBOM scanning)

```bash
git clone https://github.com/<username>/lookout.git
cd lookout

# Build CLI
go build -o lookout ./cmd/cli

# Build UI
go build -o lookout-ui ./cmd/ui

# Or use Makefile
make build
make install
```

## Documentation

- 📖 **[Usage Guide](docs/USAGE.md)** - Complete guide with examples and workflows
- 🐳 **[Docker Compose Guide](docs/DOCKER_COMPOSE.md)** - Running with Docker, services, ports, troubleshooting
- ☸️ **[Kubernetes Deployment](docs/KUBERNETES_SETUP.md)** - Complete K8s guide: Kind cluster, Gateway API, ArgoCD GitOps, AWS ALB, production deployment
- 🔒 **[TLS Setup Guide](docs/TLS_SETUP.md)** - HTTPS configuration and security best practices
- 🏗️ **[Architecture](docs/ARCHITECTURE.md)** - System design and components
- 💻 **[Development Guide](docs/DEVELOPMENT.md)** - Setup and contribution guide
- 🚀 **[CI/CD Guide](docs/CI_CD.md)** - GitHub Actions workflows and releases

## Example: Dependency Path Tracing

When you find a vulnerability in a transitive dependency, Lookout shows you the path:

```bash
lookout -sbom mybom.json -dep-path 'pkg:npm/minimist@1.2.5'
```

Output:
```
════════════════════════════════════════════════════════════
  DEPENDENCY PATH ANALYSIS
════════════════════════════════════════════════════════════

  Searched: pkg:npm/minimist@1.2.5
  Depth:    3 level(s)

  Dependency Tree:

     🔍 pkg:npm/minimist@1.2.5
     │
     └──> 📦 pkg:npm/mkdirp@0.5.1
          │
          └──> 📦 pkg:npm/mocha@8.4.0
               │
               └──> 🏠 pkg:npm/myapp@1.0.0

════════════════════════════════════════════════════════════

  Legend:
    🔍  = Vulnerable component
    📦  = Intermediate dependency
    🏠  = Your application (upgrade this dependency)
```

**Action:** Upgrade `mocha` to get the patched `minimist`.

## Configuration

### NVD API Key (Highly Recommended)

Get 10x faster CVE lookups with an API key:

```bash
# Request key: https://nvd.nist.gov/developers/request-an-api-key

# Set environment variable
export NVD_API_KEY="your-api-key-here"

# Add to shell profile for persistence
echo 'export NVD_API_KEY="your-api-key"' >> ~/.zshrc
```

| Mode | Rate Limit | Speed |
|------|-----------|-------|
| Without API Key | 5 req/30s | 6s delay |
| With API Key | 50 req/30s | 0.6s delay |

### Environment Variables

```bash
# NVD API
export NVD_API_KEY="your-api-key"

# Dgraph connection (for CLI with Docker Dgraph)
export DGRAPH_HOST=localhost  # Use "alpha" when all in Docker
export DGRAPH_PORT=9080

# Web server
export SERVER_PORT=3000
```

See [Usage Guide](docs/USAGE.md#environment-variables) for all options.

## Common Use Cases

### 1. Security Audit

```bash
# Scan your SBOM for vulnerabilities
lookout -sbom path/to/sbom.json -severity high
```

### 2. Investigate Specific CVE

```bash
# Get detailed CVE information
lookout -cve CVE-2021-44228
```

### 3. Batch CVE Processing

```bash
# Process list of CVEs
cat cves.txt
CVE-2021-44228
CVE-2022-23305

lookout -cve-file cves.txt
```

### 4. Fix Transitive Vulnerability

```bash
# 1. Scan and identify vulnerable package
lookout -sbom mybom.json

# 2. Trace dependency path
lookout -sbom mybom.json -dep-path 'pkg:npm/lodash@4.17.20'

# 3. Upgrade the direct dependency shown in path
```

## Project Structure

```
lookout/
├── cmd/
│   ├── cli/              # CLI application entry point
│   └── ui/               # Web UI application entry point
├── pkg/
│   ├── cli/
│   │   └── cli_processor/ # CLI formatting and output
│   ├── common/
│   │   ├── cyclonedx/    # CycloneDX SBOM parsing
│   │   ├── spdx/         # SPDX SBOM parsing
│   │   ├── fileutil/     # File utilities
│   │   ├── handler/      # HTTP handlers
│   │   ├── nvd/          # NVD API client
│   │   ├── processor/    # File processing
│   │   ├── progress/     # Progress tracking
│   │   └── trivy/        # Trivy integration
│   ├── config/           # Configuration management
│   ├── graph/            # Graph operations and queries
│   ├── interfaces/       # Interface definitions
│   ├── logging/          # Structured logging
│   ├── repository/       # Data access layer
│   ├── service/          # Business logic layer
│   ├── ui/               # UI components
│   │   └── echo/         # Echo server setup
│   └── validation/       # Input validation
├── assets/
│   ├── static/           # CSS, JavaScript
│   └── templates/        # HTML templates
├── nginx/                # Nginx reverse proxy config
├── examples/             # Example SBOM files
└── docs/                 # Documentation
```

## Contributing

We welcome contributions! Please see [DEVELOPMENT.md](docs/DEVELOPMENT.md) for:
- Development setup
- Code style guidelines
- Testing requirements
- Submission process

## Supported Formats

- **SBOMs**: CycloneDX 1.4+ (JSON), SPDX 2.3+ (JSON)
- **CVE Lists**: Plain text or Trivy JSON
- **Package URLs**: [PURL Specification](https://github.com/package-url/purl-spec)

## Requirements

**For CLI:**
- Go 1.21+ (build only)
- Trivy (optional, for SBOM scanning)
- Dgraph (optional, for dependency tracing)

**For Web UI:**
- Docker & Docker Compose

## Known Limitations

1. **Rate Limiting**: NVD API has strict rate limits. Use an API key for best performance.
2. **SBOM Format**: Supports CycloneDX 1.4+ and SPDX 2.3+ (JSON only, XML not yet supported).
3. **Large SBOMs**: Processing hundreds of CVEs can be slow. NVD API key highly recommended.

See [Usage Guide](docs/USAGE.md#troubleshooting) for solutions.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- [National Vulnerability Database (NVD)](https://nvd.nist.gov/)
- [Trivy](https://github.com/aquasecurity/trivy) - Vulnerability scanner
- [Dgraph](https://dgraph.io/) - Graph database
- [CycloneDX](https://cyclonedx.org/) - SBOM standard
- [SPDX](https://spdx.dev/) - SBOM standard

## Support

- 📚 [Documentation](docs/)
- 🐛 [Report Issues](https://github.com/<username>/lookout/issues)
- 💬 [Discussions](https://github.com/<username>/lookout/discussions)

---

**Star ⭐ this repository if you find it useful!**
