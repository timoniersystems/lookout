# Development Guide

This guide covers setting up your development environment, coding standards, and contributing to Lookout.

## Prerequisites

### Required
- **Go 1.21+** - [Install Go](https://golang.org/doc/install)
- **Docker** - [Install Docker](https://docs.docker.com/get-docker/)
- **Docker Compose** - Usually bundled with Docker Desktop
- **Git** - [Install Git](https://git-scm.com/downloads)

### Optional but Recommended
- **golangci-lint** - Code linting
  ```bash
  brew install golangci-lint  # macOS
  # or: https://golangci-lint.run/usage/install/
  ```
- **Trivy** - SBOM scanning (for testing)
  ```bash
  brew install trivy  # macOS
  # or: https://aquasecurity.github.io/trivy/latest/getting-started/installation/
  ```
- **act** - Test GitHub Actions locally
  ```bash
  brew install act  # macOS
  ```

## Getting Started

### 1. Clone Repository

```bash
git clone https://github.com/<username>/lookout.git
cd lookout
```

### 2. Install Dependencies

```bash
# Download Go dependencies
go mod download

# Verify dependencies
go mod verify
```

### 3. Environment Setup

Create `.env` file in project root:

```bash
cat > .env << 'EOF'
# Dgraph Configuration
DGRAPH_HOST=alpha
DGRAPH_PORT=9080

# Web Server
SERVER_PORT=3000

# NVD API (optional - higher rate limits)
# Get key from: https://nvd.nist.gov/developers/request-an-api-key
# NVD_API_KEY=your_api_key_here

# Logging
LOG_LEVEL=info
EOF
```

### 4. Start Development Services

```bash
# Start Dgraph database
docker-compose up -d

# Verify services are running
docker-compose ps

# Check Dgraph health
curl http://localhost:9080/health
```

### 5. Build and Run

**Using Makefile (Recommended):**
```bash
# Build both CLI and UI
make build

# Build CLI only
make build-cli

# Build UI only
make build-gui

# Install binaries to ~/.local/bin
make install
```

**CLI:**
```bash
# Build
make build-cli
# or: go build -o lookout ./cmd/cli

# Run
./lookout -help

# Or run directly
go run ./cmd/cli -help
```

**Web UI:**
```bash
# Build
make build-gui
# or: go build -o lookout-ui ./cmd/ui

# Run
./lookout-ui

# Or run directly
go run ./cmd/ui
```

Access web UI at: http://localhost:3000

## Project Structure

```
lookout/
├── cmd/
│   ├── cli/              # CLI entry point
│   └── ui/               # Web UI entry point
├── pkg/
│   ├── cli/
│   │   └── cli_processor/ # CLI argument parsing and formatting
│   ├── common/
│   │   ├── cyclonedx/    # SBOM parsing
│   │   ├── fileutil/     # File utilities
│   │   ├── handler/      # HTTP handlers
│   │   ├── nvd/          # NVD API client
│   │   ├── processor/    # File processing
│   │   └── trivy/        # Trivy integration
│   ├── config/           # Configuration
│   ├── ui/
│   │   ├── dgraph/       # Dgraph operations
│   │   └── echo/         # Echo server setup
│   ├── interfaces/       # Interface definitions
│   ├── logging/          # Logging utilities
│   ├── repository/       # Data access layer
│   ├── service/          # Business logic
│   └── validation/       # Input validation
├── templates/            # HTML templates
├── examples/             # Example SBOM files
├── docs/                 # Documentation
└── .github/
    └── workflows/        # CI/CD workflows
```

## Development Workflow

### 1. Create Feature Branch

```bash
git checkout -b feature/my-new-feature
```

### 2. Make Changes

Follow these principles:
- Write tests for new functionality
- Update documentation as needed
- Follow Go best practices
- Keep functions small and focused

### 3. Test Your Changes

```bash
# Run unit tests only (fast, no external dependencies)
make test
# or: go test -short ./...

# Run all tests including integration tests (requires Dgraph)
make test-all
# or: go test -tags=integration ./...

# Run only integration tests
make test-integration

# Run tests with coverage
make test-coverage
# or: go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with race detection
go test -race ./...

# Test specific package
go test -v ./pkg/validation

# Run tests continuously
go test -v ./... -count=1
```

**Note on Integration Tests:**
Integration tests are marked with the `//go:build integration` build tag and require external dependencies (like Dgraph). They are located alongside unit tests in the same packages:
- `pkg/gui/dgraph/integration_test.go` - Dgraph database integration tests
- `pkg/common/cyclonedx/integration_test.go` - SBOM parsing and traversal integration tests

To skip integration tests during development, use `go test -short ./...` or `make test`.

### 4. Lint Your Code

```bash
# Run all linters
golangci-lint run

# Run specific linters
golangci-lint run --enable-only=errcheck,gosec,staticcheck

# Auto-fix issues
golangci-lint run --fix
```

### 5. Commit Changes

```bash
git add .
git commit -m "feat: add new feature description"
```

Follow [conventional commits](https://www.conventionalcommits.org/):
- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation
- `test:` - Tests
- `refactor:` - Code refactoring
- `ci:` - CI/CD changes
- `chore:` - Maintenance

### 6. Push and Create PR

```bash
git push origin feature/my-new-feature
```

Then create a Pull Request on GitHub.

## Testing Guide

### Writing Tests

**Unit Test Example:**
```go
package mypackage

import "testing"

func TestMyFunction(t *testing.T) {
    t.Run("valid input", func(t *testing.T) {
        result := MyFunction("input")
        expected := "output"

        if result != expected {
            t.Errorf("Expected %s, got %s", expected, result)
        }
    })

    t.Run("invalid input", func(t *testing.T) {
        result := MyFunction("")

        if result != "" {
            t.Error("Expected empty string for empty input")
        }
    })
}
```

**Table-Driven Tests:**
```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid", "CVE-2021-44228", false},
        {"invalid", "INVALID", true},
        {"empty", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := Validate(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Test Coverage Goals

- **Overall Target**: 80%
- **Current**: 25.4%
- **Package Requirements**:
  - Core business logic: >80%
  - Utilities: >90%
  - Handlers/Infrastructure: >60%

### Running Tests in Docker

```bash
# Build test image
docker build -f Dockerfile.test -t lookout:test .

# Run tests in container
docker run --rm lookout:test go test ./...
```

## Code Style

### Go Best Practices

1. **Error Handling**
   ```go
   // Good
   if err != nil {
       return fmt.Errorf("operation failed: %w", err)
   }

   // Bad
   if err != nil {
       log.Fatal(err)  // Don't use in libraries
   }
   ```

2. **Context Usage**
   ```go
   // Good
   func DoSomething(ctx context.Context, param string) error {
       // Use context for timeouts, cancellation
   }

   // Bad
   func DoSomething(param string) error {
       ctx := context.Background()  // Don't create context here
   }
   ```

3. **Naming Conventions**
   - Packages: lowercase, single word
   - Exported: PascalCase
   - Unexported: camelCase
   - Interfaces: -er suffix (Reader, Writer)

4. **Documentation**
   ```go
   // ValidateCVEID validates that a CVE ID follows the correct format.
   // It returns an error if the ID is empty or malformed.
   func ValidateCVEID(cveID string) error {
       // implementation
   }
   ```

### Linting Rules

See `.golangci.yml` for complete configuration.

**Key linters enabled:**
- `errcheck` - Check unchecked errors
- `gosec` - Security issues
- `staticcheck` - Static analysis
- `govet` - Suspicious constructs
- `revive` - Code quality
- `misspell` - Spelling errors

## Debugging

### CLI Debugging

```bash
# Enable debug logging
export LOG_LEVEL=debug
./lookout -cve CVE-2021-44228

# Use Delve debugger
go install github.com/go-delve/delve/cmd/dlv@latest
dlv debug ./cmd/lookout -- -cve CVE-2021-44228
```

### Web UI Debugging

```bash
# Enable debug mode
export LOG_LEVEL=debug
./lookout-ui

# Debug with Delve
dlv debug ./cmd/ui
```

### Dgraph Debugging

```bash
# Access Ratel UI
open http://localhost:8000

# Query Dgraph directly
curl -X POST localhost:8080/query -H "Content-Type: application/dql" -d '{
  components(func: has(purl)) {
    uid
    name
    version
    purl
  }
}'

# Check Dgraph logs
docker-compose logs alpha

# Check Dgraph stats
curl http://localhost:8080/health
```

## Common Tasks

### Adding a New Package

```bash
# Create package directory
mkdir -p pkg/mynewpackage

# Create files
touch pkg/mynewpackage/mynewpackage.go
touch pkg/mynewpackage/mynewpackage_test.go

# Add package documentation
cat > pkg/mynewpackage/doc.go << 'EOF'
// Package mynewpackage provides functionality for...
package mynewpackage
EOF
```

### Adding a New CLI Flag

1. Edit `pkg/cli/cli_processor/cli_processor.go`
2. Add flag to `ParseCLIArgs()`
3. Update help text in `printHelp()`
4. Add tests in `cli_processor_test.go`

### Adding a New HTTP Endpoint

1. Add handler in `pkg/common/handler/handlers.go`
2. Register route in `pkg/gui/echo/launch_web_server.go`
3. Create template in `templates/`
4. Add tests

### Updating Dependencies

```bash
# Update specific dependency
go get -u github.com/labstack/echo/v4

# Update all dependencies
go get -u ./...

# Tidy and verify
go mod tidy
go mod verify

# Test after update
go test ./...
```

## Performance Profiling

### CPU Profiling

```bash
# Generate profile
go test -cpuprofile=cpu.prof -bench=. ./pkg/mypackage

# Analyze with pprof
go tool pprof cpu.prof

# Web UI
go tool pprof -http=:8080 cpu.prof
```

### Memory Profiling

```bash
# Generate profile
go test -memprofile=mem.prof -bench=. ./pkg/mypackage

# Analyze
go tool pprof mem.prof
```

### Benchmarking

```go
func BenchmarkMyFunction(b *testing.B) {
    for i := 0; i < b.N; i++ {
        MyFunction("input")
    }
}
```

Run benchmarks:
```bash
go test -bench=. ./pkg/mypackage
```

## Troubleshooting

### "Module not found" Error

```bash
go mod download
go mod tidy
```

### Dgraph Connection Refused

```bash
# Check if Dgraph is running
docker-compose ps

# Restart Dgraph
docker-compose restart alpha

# Check logs
docker-compose logs alpha
```

### Tests Failing Intermittently

```bash
# Run with race detector
go test -race ./...

# Run multiple times
go test -count=10 ./pkg/mypackage
```

## Resources

- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Dgraph Documentation](https://dgraph.io/docs/)
- [Echo Framework Guide](https://echo.labstack.com/guide/)
- [Project Architecture](ARCHITECTURE.md)
- [CI/CD Guide](CI_CD.md)

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/<username>/lookout/issues)
- **Discussions**: [GitHub Discussions](https://github.com/<username>/lookout/discussions)
- **Dgraph**: [Dgraph Discuss](https://discuss.dgraph.io/)
