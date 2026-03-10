# Contributing to Lookout

Thank you for your interest in contributing to Lookout! This document covers the contribution process, policies, and development setup.

## How to Contribute

1. **Open an issue** first to discuss the change you'd like to make
2. **Fork the repository** and create a feature branch from `main`
3. **Make your changes** following the coding standards below
4. **Write tests** for new functionality
5. **Run the test suite** and linter before submitting:
   ```bash
   make test
   golangci-lint run
   ```
6. **Submit a pull request** using the PR template

## Pull Request Guidelines

- Keep PRs focused on a single change
- Write a clear description of what the PR does and why
- Reference related issues (e.g., "Fixes #42")
- Ensure CI passes (tests, lint, security scan)
- Be responsive to review feedback

### What makes a good PR

- **Small and focused** - one logical change per PR
- **Well-tested** - new code has tests, existing tests still pass
- **Clear intent** - the reviewer can understand *why* the change was made
- **Clean history** - squash fixup commits before requesting review

### What will get a PR closed

- Drive-by PRs with no prior discussion on an issue
- PRs that don't pass CI
- Bulk changes with no clear rationale
- PRs where the author can't explain the changes when asked

## AI-Assisted Contributions Policy

We welcome contributions that use AI tools (GitHub Copilot, Claude, ChatGPT, etc.) as part of the development process. However, we require transparency and accountability.

### Disclosure Requirement

If AI tools were used in a meaningful way to generate, debug, or design your contribution, you must disclose this in your pull request. The PR template includes a required section for this.

**What requires disclosure:**
- Code generated or substantially written by AI
- Architecture or design decisions guided by AI
- Debugging assistance that led to the fix
- Documentation drafted by AI

**What does not require disclosure:**
- IDE autocomplete or simple code completion
- Syntax suggestions or variable name recommendations
- Using AI to understand existing code (reading, not writing)

### Quality Standards

AI-assisted contributions are held to the same quality standards as any other contribution:

- **You must understand the code you submit.** If you can't explain what it does and why during review, the PR will be closed.
- **You must have tested the code.** "The AI generated it" is not a substitute for verification.
- **You are responsible for correctness.** The contributor, not the AI tool, is accountable for bugs, security issues, and maintenance.

### What we will reject

- **AI slop**: bulk PRs that are clearly unreviewed AI output (formatting issues, hallucinated APIs, generic boilerplate that doesn't fit the project)
- **Unexplained changes**: if you can't articulate why a change is needed or how it works, it won't be merged regardless of how it was produced
- **Quantity over quality**: multiple low-effort AI-generated PRs are worse than one thoughtful contribution

## Reporting Issues

- Use [GitHub Issues](https://github.com/timoniersystems/lookout/issues) for bugs and feature requests
- For security vulnerabilities, see [SECURITY.md](SECURITY.md)
- Include reproduction steps for bugs
- Check existing issues before opening a new one

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).

---

## Development Setup

### Prerequisites

**Required:**
- **Go 1.26+** - [Install Go](https://golang.org/doc/install)
- **Docker** - [Install Docker](https://docs.docker.com/get-docker/)
- **Docker Compose** - Usually bundled with Docker Desktop
- **Git** - [Install Git](https://git-scm.com/downloads)

**Optional but Recommended:**
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

### Getting Started

#### 1. Clone Repository

```bash
git clone https://github.com/timoniersystems/lookout.git
cd lookout
```

#### 2. Install Dependencies

```bash
go mod download
go mod verify
```

#### 3. Environment Setup

Create a `.env` file in the project root:

```bash
cat > .env << 'EOF'
DGRAPH_HOST=alpha
DGRAPH_PORT=9080
SERVER_PORT=3000
# NVD_API_KEY=your_api_key_here  # optional, higher rate limits
LOG_LEVEL=info
EOF
```

#### 4. Start Development Services

```bash
make up             # Start full stack (Dgraph + app + nginx)
make up-standalone  # Start Dgraph only

# Verify
docker compose ps
curl http://localhost:9080/health
```

#### 5. Build and Run

```bash
make build       # Build both CLI and UI
make build-cli   # CLI only
make build-ui    # UI only
make install     # Install binaries to ~/.local/bin
```

**CLI:**
```bash
make build-cli
./lookout --help
# or run directly:
go run ./cmd/cli --help
```

**Web UI:**
```bash
make build-ui
./lookout-ui
# or run directly:
go run ./cmd/ui
```

Access the web UI at http://localhost:3000 (or https://localhost:7443 via nginx with Docker Compose).

### Project Structure

```
lookout/
├── cmd/
│   ├── cli/              # CLI entry point (Cobra commands)
│   └── ui/               # Web UI entry point
├── pkg/
│   ├── cli/
│   │   └── cli_processor/ # CVE formatting and output
│   ├── common/
│   │   ├── cyclonedx/    # CycloneDX SBOM parsing
│   │   ├── spdx/         # SPDX SBOM parsing
│   │   ├── fileutil/     # File utilities
│   │   ├── handler/      # HTTP handlers
│   │   ├── nvd/          # NVD API client
│   │   ├── processor/    # File processing
│   │   ├── progress/     # Progress tracking (SSE)
│   │   └── trivy/        # Trivy integration
│   ├── config/           # Configuration management
│   ├── graph/            # Graph database operations
│   ├── interfaces/       # Interface definitions
│   ├── logging/          # Structured logging
│   ├── repository/       # Data access layer
│   ├── service/          # Business logic layer
│   ├── ui/
│   │   └── echo/         # Echo server setup
│   └── validation/       # Input validation
├── assets/
│   ├── static/           # CSS, JavaScript
│   └── templates/        # HTML templates
├── nginx/                # Nginx reverse proxy config
├── examples/             # Example SBOM files
├── docs/                 # Documentation
└── .github/
    └── workflows/        # CI/CD workflows
```

### Development Workflow

#### 1. Create Feature Branch

```bash
git checkout -b feature/my-new-feature
```

#### 2. Make Changes

- Write tests for new functionality
- Update documentation as needed
- Follow Go best practices
- Keep functions small and focused

#### 3. Test Your Changes

```bash
make test             # Unit tests only (fast, no external deps)
make test-all         # All tests including integration (requires Dgraph)
make test-integration # Integration tests only
make test-coverage    # Generate HTML coverage report

# Additional options
go test -race ./...              # Race detection
go test -v ./pkg/validation      # Specific package
go test -count=10 ./pkg/...      # Run multiple times
```

**Note on Integration Tests:**
Integration tests use the `//go:build integration` tag and require Dgraph running. They live alongside unit tests (e.g. `pkg/common/cyclonedx/integration_test.go`). Use `make test` or `go test -short ./...` to skip them during development.

#### 4. Lint Your Code

```bash
golangci-lint run                                           # All linters
golangci-lint run --enable-only=errcheck,gosec,staticcheck
golangci-lint run --fix                                     # Auto-fix
```

#### 5. Commit Changes

Follow [conventional commits](https://www.conventionalcommits.org/):
- `feat:` - New features
- `fix:` - Bug fixes
- `docs:` - Documentation
- `test:` - Tests
- `refactor:` - Code refactoring
- `ci:` - CI/CD changes
- `chore:` - Maintenance

#### 6. Push and Create PR

```bash
git push origin feature/my-new-feature
```

Then create a Pull Request on GitHub.

### Testing Guide

**Unit Test Example:**
```go
func TestMyFunction(t *testing.T) {
    t.Run("valid input", func(t *testing.T) {
        result := MyFunction("input")
        if result != "output" {
            t.Errorf("expected 'output', got %q", result)
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

CI enforces a minimum coverage threshold of 25% (warning only, non-blocking).

### Code Style

**Error Handling:**
```go
// Good
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// Bad
if err != nil {
    log.Fatal(err)  // don't use in libraries
}
```

**Context Usage:**
```go
// Good
func DoSomething(ctx context.Context, param string) error { ... }

// Bad — don't create context inside the function
func DoSomething(param string) error {
    ctx := context.Background()
    ...
}
```

**Naming Conventions:**
- Packages: lowercase, single word
- Exported: PascalCase
- Unexported: camelCase
- Interfaces: `-er` suffix (Reader, Writer)

**Key linters enabled** (see `.golangci.yml` for full config):
`errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gosec`, `gocritic`, `misspell`

### Debugging

**CLI:**
```bash
./lookout --debug cve CVE-2021-44228

# Delve debugger
go install github.com/go-delve/delve/cmd/dlv@latest
dlv debug ./cmd/cli -- cve CVE-2021-44228
```

**Web UI:**
```bash
export LOG_LEVEL=debug
./lookout-ui

dlv debug ./cmd/ui
```

**Dgraph:**
```bash
open http://localhost:8000  # Ratel UI

curl -X POST localhost:8080/query -H "Content-Type: application/dql" -d '{
  components(func: has(purl)) { uid name version purl }
}'

docker compose logs dgraph-alpha
curl http://localhost:8080/health
```

### Common Tasks

**Adding a New CLI Subcommand:**
1. Create `cmd/cli/<command>.go` in `package main`
2. Define a `*cobra.Command` and register it via `rootCmd.AddCommand(...)` in `init()`
3. Add tests in `cmd/cli/cli_test.go`

**Adding a New HTTP Endpoint:**
1. Add handler in `pkg/common/handler/handlers.go`
2. Register route in `pkg/ui/echo/launch_web_server.go`
3. Create template in `assets/templates/`
4. Add tests

**Adding a New Package:**
```bash
mkdir -p pkg/mynewpackage
touch pkg/mynewpackage/mynewpackage.go
touch pkg/mynewpackage/mynewpackage_test.go
```

**Updating Dependencies:**
```bash
go get -u github.com/labstack/echo/v4  # specific
go get -u ./...                        # all
go mod tidy && go mod verify
go test ./...
```

### Performance Profiling

```bash
# CPU
go test -cpuprofile=cpu.prof -bench=. ./pkg/mypackage
go tool pprof -http=:8080 cpu.prof

# Memory
go test -memprofile=mem.prof -bench=. ./pkg/mypackage
go tool pprof mem.prof

# Benchmarks
go test -bench=. ./pkg/mypackage
```

### Troubleshooting

**"Module not found":**
```bash
go mod download && go mod tidy
```

**Dgraph connection refused:**
```bash
docker compose ps
docker compose restart dgraph-alpha
docker compose logs dgraph-alpha
```

**Tests failing intermittently:**
```bash
go test -race ./...
go test -count=10 ./pkg/mypackage
```

### Resources

- [Effective Go](https://golang.org/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Dgraph Documentation](https://dgraph.io/docs/)
- [Echo Framework Guide](https://echo.labstack.com/guide/)
- [Project Architecture](docs/ARCHITECTURE.md)
- [CI/CD Guide](docs/CI_CD.md)
- **Issues**: [GitHub Issues](https://github.com/timoniersystems/lookout/issues)
- **Discussions**: [GitHub Discussions](https://github.com/timoniersystems/lookout/discussions)
