# CI/CD Pipeline

Lookout uses GitHub Actions for continuous integration and deployment.

## Workflows Overview

### CI Workflow (`ci.yml`)

**Triggers:** Push to `main`/`develop` and pull requests to `main`/`develop`
**Path filters:** Only runs when Go source files, `go.mod`, `go.sum`, or the workflow file change.

**Jobs:**

1. **Test** - Run unit tests with race detection and coverage
   - Go 1.26
   - `go test -v -race -timeout 5m ./...`
   - Coverage uploaded to Codecov
2. **Lint** - Code quality checks with golangci-lint v2.9.0 (5-minute timeout)
3. **Build** - Compile CLI and UI binaries, upload as artifacts (7-day retention)
4. **Security Scan** - Gosec scanner with SARIF output (`-no-fail` mode)

### Coverage Workflow (`coverage.yml`)

**Triggers:** Push to `main` and pull requests to `main`
**Path filters:** Same as CI workflow.

**Features:**
- Generates detailed coverage report with `go tool cover`
- Posts coverage summary as PR comment
- Generates a coverage badge via GitHub Gist (requires `GIST_SECRET` and `GIST_ID` secrets)
- Warns if coverage drops below 25% threshold (non-blocking)
- Uploads coverage report as artifact (30-day retention)

### Docker Workflow (`docker.yml`)

**Triggers:** Push to `main` and version tags (`v*`)
**Path filters:** Go source files, `Dockerfile`, `scripts/**`, or the workflow file.

**Jobs:**

1. **Build** - Build Docker image (linux/amd64) with Buildx
   - Source hash-based cache key for invalidation on code changes
   - Build args: `GO_VERSION=1.26.0`, `TRIVY_VERSION=0.69.1`
   - Image exported as artifact for downstream jobs

2. **Trivy Scan** - Vulnerability scanning in 3 parallel matrix configurations:
   | Format | Severity | Purpose |
   |--------|----------|---------|
   | SARIF | CRITICAL, HIGH, MEDIUM | Upload to GitHub Security tab |
   | JSON | CRITICAL, HIGH, MEDIUM, LOW | Artifact for offline review (30-day retention) |
   | Table | CRITICAL, HIGH | Human-readable output (non-blocking) |

3. **Push** - Push image to GHCR (skipped on pull requests)
   - Requires build and scan to pass first
   - Attests build provenance via `actions/attest-build-provenance@v2` (OIDC-signed, pushed to registry)

4. **Docker Compose** - Integration test of the full stack
   - Generates TLS certs, starts all services
   - Tests Dgraph health, Lookout UI health, nginx HTTPS, HTTP-to-HTTPS redirect

**Image Tags** (via `docker/metadata-action`):
- `main` - Latest main branch build
- `v1.0.0`, `v1.0`, `v1` - Semver from tags
- `sha-abc1234` - Commit-specific
- `latest` - Default branch only

**Deployment Triggers:**
- Push to `main` → Image tagged `:main` → ArgoCD deploys to staging (digest strategy)
- Push semver tag (`v*`) → Image tagged `:v1.0.0` → ArgoCD deploys to production (semver strategy)

### Release Workflow (`release.yml`)

**Triggers:** Version tags (`v*`)

**Jobs:**

1. **Create Release**
   - Runs full test suite
   - Cross-compiles CLI for 5 platforms + UI for Linux
   - Attests build provenance for all 6 binaries via `actions/attest-build-provenance@v2`
   - Generates changelog from git log since previous tag
   - Creates GitHub release with binaries and checksums
   - Pre-release auto-detected for `-rc`, `-beta`, `-alpha` tags

2. **Publish Docker Image** (runs after release)
   - Builds and pushes to GHCR with version tag + `latest`
   - Attests Docker image provenance (OIDC-signed, pushed to registry)

**Supported Platforms:**
| OS | Architecture |
|----|-------------|
| Linux | AMD64, ARM64 |
| macOS | AMD64 (Intel), ARM64 (Apple Silicon) |
| Windows | AMD64 |

**Release Artifacts:**
```
lookout-linux-amd64
lookout-linux-arm64
lookout-darwin-amd64
lookout-darwin-arm64
lookout-windows-amd64.exe
lookout-ui-linux-amd64
checksums.txt
```

## Build Provenance Attestations

Both the Release and Docker workflows generate [SLSA build provenance](https://slsa.dev/) attestations using `actions/attest-build-provenance@v2`. These are OIDC-signed statements that cryptographically link each artifact to the GitHub Actions workflow that produced it.

**What gets attested:**
- **Release binaries** - All 6 platform binaries (CLI + UI) via `subject-path`
- **Docker images** (release) - Image digest attested and pushed to GHCR registry
- **Docker images** (main branch) - Image digest captured from `docker push` output, attested and pushed to registry

**Required permissions:** `id-token: write` (OIDC token) and `attestations: write`

**Verification:**

```bash
# Verify a downloaded binary
gh attestation verify lookout-linux-amd64 --repo timoniersystems/lookout

# Verify a Docker image
gh attestation verify oci://ghcr.io/timoniersystems/lookout:latest --repo timoniersystems/lookout
```

Attestations are visible on the GitHub release page under the "Attestations" section.

## Dependency Management

### Dependabot Configuration

**Schedule:** Weekly on Mondays at 6 AM UTC

**Ecosystems monitored:**
- **Go modules** - Up to 10 open PRs, grouped by framework (Dgraph, Echo)
- **GitHub Actions** - Up to 5 open PRs
- **Docker** - Up to 5 open PRs

**Commit message prefixes:**
- Go: `chore(deps)`
- Actions: `chore(ci)`
- Docker: `chore(docker)`

## Creating a Release

### 1. Prepare

```bash
git checkout main
git pull origin main

# Run tests
go test -v ./...
```

### 2. Tag and Push

```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

This triggers two workflows:
- **Release** - Builds binaries, creates GitHub release
- **Docker Build** - Builds and pushes Docker image, ArgoCD deploys to production

### 3. Verify Production Deployment

ArgoCD Image Updater (semver strategy) automatically detects the new tag and deploys to production.

```bash
# Check ArgoCD production app status
kubectl get application lookout-production -n argocd

# Check production pods
kubectl get pods -n production

# Test production endpoint
curl -u production:PASSWORD https://lookout-prod.timonier.io/health
```

## Setting Up CI/CD

### 1. Required Secrets

None - the workflows use `GITHUB_TOKEN` which is automatically provided.

### 2. Optional Secrets

```bash
# Codecov integration
gh secret set CODECOV_TOKEN --body "your_codecov_token"

# Coverage badge via Gist
gh secret set GIST_SECRET --body "ghp_your_github_token"  # needs gist scope
gh secret set GIST_ID --body "your_gist_id"
```

### 3. Branch Protection (Recommended)

Settings > Branches > Add rule for `main`:
- Require status checks: `Test`, `Build`
- Require branches to be up to date before merging

## Local Testing

### Run Tests

```bash
# Unit tests (fast)
go test -short ./...

# With race detection
go test -race ./...

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Integration tests (requires Dgraph)
go test -tags=integration ./...
```

### Test Docker Build

```bash
docker build -t lookout:local .

# Full stack
./scripts/generate-certs.sh
docker compose up -d
curl -k https://localhost:7443/health
```

### Test Workflows Locally with Act

```bash
brew install act
act -l          # List workflows
act -j test     # Run test job
act push        # Simulate push event
```
