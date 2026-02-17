# CI/CD Pipeline

Lookout uses GitHub Actions for continuous integration and deployment.

## Workflows Overview

### 🔄 CI Workflow (`ci.yml`)
**Triggers:** Every push to `main`/`develop` and all pull requests

**Jobs:**
1. **Test** - Run test suite across Go versions
2. **Lint** - Code quality checks with golangci-lint
3. **Build** - Compile CLI and UI binaries
4. **Security** - Gosec security scanning

**Matrix Testing:**
- Go 1.21
- Go 1.22

**Features:**
- Race detection (`-race` flag)
- Dependency caching
- Coverage upload to Codecov
- SARIF security reports

### 📊 Coverage Workflow (`coverage.yml`)
**Triggers:** Push to `main` and pull requests

**Features:**
- Detailed coverage reports
- PR comments with coverage summary
- Coverage badge generation
- Threshold enforcement (25%)

**Coverage Badge:**
Shows current test coverage percentage with color coding:
- 🟢 Green: >80%
- 🟡 Yellow: 60-80%
- 🔴 Red: <60%

### 🐳 Docker Workflow (`docker.yml`)
**Triggers:** Push to `main` and version tags (not pull requests)

**Jobs:**
1. **Build** - Multi-platform Docker image
2. **Docker Compose** - Integration testing

**Features:**
- GitHub Container Registry (GHCR) publishing
- Trivy vulnerability scanning
- Build caching for faster builds
- Automatic semantic versioning

**Image Tags:**
- `main` - Latest main branch
- `v1.0.0` - Semantic version
- `<commit-sha>` - Commit-specific
- `latest` - Latest release

**Deployment Triggers:**
- Push to `main` → Builds image with `:main` tag → ArgoCD deploys to staging (digest strategy)
- Push semver tag (`v*`) → Builds image with `:v1.0.0` tag → ArgoCD deploys to production (semver strategy)

### 🚀 Release Workflow (`release.yml`)
**Triggers:** Version tags (`v*`)

**Automated Release Process:**
1. Run full test suite
2. Build multi-platform binaries
3. Generate changelog
4. Create GitHub release
5. Publish Docker images

**Supported Platforms:**
- **Linux**: AMD64, ARM64
- **macOS**: AMD64 (Intel), ARM64 (Apple Silicon)
- **Windows**: AMD64

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

### 🔒 CodeQL Workflow (`codeql.yml`)
**Triggers:** Push to `main`, pull requests, weekly schedule

**Security Analysis:**
- Static code analysis
- Security vulnerability detection
- Code quality checks
- SARIF report generation

**Schedule:** Every Monday at 6 AM UTC

## Dependency Management

### Dependabot Configuration
**Schedule:** Weekly updates on Mondays

**Monitors:**
- Go modules (grouped by framework)
- GitHub Actions versions
- Docker base images

**Grouping:**
- Dgraph dependencies
- Echo framework dependencies

## Setting Up CI/CD

### 1. Enable GitHub Features

Go to repository **Settings**:

**Actions:**
- ✅ Enable GitHub Actions
- ✅ Allow all actions and reusable workflows

**Security:**
- ✅ Enable Dependabot alerts
- ✅ Enable Dependabot security updates
- ✅ Enable Secret scanning
- ✅ Enable Code scanning (CodeQL)

**Packages:**
- ✅ Enable GitHub Packages

### 2. Configure Secrets (Optional)

For enhanced features, add these secrets:

```bash
# Codecov integration (optional)
gh secret set CODECOV_TOKEN --body "your_codecov_token"

# Coverage badge (optional)
gh secret set GIST_SECRET --body "ghp_your_github_token"
gh secret set GIST_ID --body "your_gist_id"
```

**Creating a coverage badge:**
1. Create a new gist: https://gist.github.com/
2. Copy the gist ID from the URL
3. Generate a GitHub token with `gist` scope
4. Add secrets to repository

### 3. Branch Protection Rules

**Recommended settings for `main` branch:**

Settings → Branches → Add rule:
- Branch name pattern: `main`
- ✅ Require pull request reviews before merging
- ✅ Require status checks to pass before merging
  - ✅ `Test (1.21)`
  - ✅ `Test (1.22)`
  - ✅ `Lint`
  - ✅ `Build`
- ✅ Require branches to be up to date before merging
- ✅ Require conversation resolution before merging

## Creating a Release

### 1. Prepare Release

```bash
# Ensure you're on main and up to date
git checkout main
git pull origin main

# Ensure tests pass
make test
# or: go test -short ./...

# Ensure linting passes
golangci-lint run
```

### 2. Create and Push Tag

```bash
# Create annotated tag
git tag -a v1.0.0 -m "Release v1.0.0"

# Push tag to trigger release workflow
git push origin v1.0.0
```

### 3. Monitor Release

1. Go to **Actions** tab
2. Watch `Release` workflow
3. Release appears in **Releases** section when complete

### 4. Post-Release

The release workflow automatically:
- ✅ Builds binaries for all platforms
- ✅ Generates changelog
- ✅ Creates GitHub release with assets
- ✅ Publishes Docker image with `:latest` tag

### 5. Production Deployment

After pushing a semver tag, ArgoCD Image Updater (configured with semver strategy) automatically detects the new tag and deploys it to the production namespace. No manual deployment steps are needed.

**Verify production deployment:**
```bash
# Check ArgoCD production app status
kubectl get application lookout-production -n argocd

# Check production pods
kubectl get pods -n production

# Test production endpoint
curl -u production:PASSWORD https://lookout-prod.timonier.io/health
```

## Local Testing

### Test Workflows Locally with Act

```bash
# Install act (macOS)
brew install act

# Or download from: https://github.com/nektos/act

# List workflows
act -l

# Run CI workflow
act -j test

# Run specific job
act -j lint

# Run with specific event
act push
act pull_request
```

### Test Docker Build

```bash
# Build image
docker build -t lookout:local .

# Test with docker-compose
docker-compose up

# Check Dgraph health
curl http://localhost:9080/health
```

### Run Tests Locally

```bash
# Unit tests only (fast, no external dependencies)
make test
# or: go test -short ./...

# All tests including integration tests (requires Dgraph)
make test-all
# or: go test -tags=integration ./...

# Integration tests only
make test-integration

# With coverage
make test-coverage
# or: go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# With race detection
go test -race ./...

# Specific package
go test -v ./pkg/validation
```

**Integration Tests:**
Integration tests are marked with `//go:build integration` build tags and require external dependencies like Dgraph. They are located alongside unit tests in packages like `pkg/gui/dgraph/` and `pkg/common/cyclonedx/`.

### Run Linters Locally

```bash
# Install golangci-lint
brew install golangci-lint

# Run linting
golangci-lint run

# Run specific linters
golangci-lint run --enable-only=errcheck,gosec

# Fix auto-fixable issues
golangci-lint run --fix
```

## Monitoring CI/CD

### GitHub Actions Dashboard
`https://github.com/<username>/lookout/actions`

**Sections:**
- **All workflows** - Overview of all runs
- **CI** - Test, lint, build results
- **Coverage** - Coverage trends
- **Docker** - Image build status
- **Release** - Release workflow status

### Packages
`https://github.com/<username>/lookout/pkgs/container/lookout`

**View:**
- Published Docker images
- Image tags and sizes
- Pull statistics

### Security
`https://github.com/<username>/lookout/security`

**Tabs:**
- **Code scanning** - CodeQL alerts
- **Dependabot** - Dependency updates
- **Secret scanning** - Leaked secrets

## Troubleshooting

### Tests Failing in CI but Passing Locally

**Causes:**
- Different Go version
- Missing dependencies
- Race conditions
- Environment differences

**Solutions:**
```bash
# Test with same Go version as CI
go version  # Should be 1.21 or 1.22

# Run with race detection
go test -race ./...

# Clear cache
go clean -cache -testcache
```

### Docker Build Failing

**Common issues:**
- Outdated dependencies: `go mod tidy`
- Missing .dockerignore: Add large directories
- Network issues: Check proxy settings

### Coverage Badge Not Updating

**Checklist:**
- ✅ Workflow runs on `main` branch
- ✅ `GIST_SECRET` and `GIST_ID` are set
- ✅ Gist token has `gist` scope
- ✅ Gist is public

### Release Not Creating

**Checklist:**
- ✅ Tag starts with `v` (e.g., `v1.0.0`)
- ✅ Tag is pushed to remote
- ✅ Workflow has `contents: write` permission
- ✅ No test failures

## Best Practices

### Commit Messages
Follow conventional commits:
```
feat: add new feature
fix: bug fix
docs: documentation changes
test: add/update tests
ci: CI/CD changes
refactor: code refactoring
chore: maintenance tasks
```

### Pull Requests
- Create from feature branch
- Wait for all checks to pass
- Address review comments
- Squash merge when ready

### Versioning
Follow semantic versioning (semver):
- `v1.0.0` - Major release
- `v1.1.0` - Minor release (new features)
- `v1.1.1` - Patch release (bug fixes)
- `v2.0.0-beta.1` - Pre-release

## Metrics and Badges

### Add to README

```markdown
![CI](https://github.com/<username>/lookout/actions/workflows/ci.yml/badge.svg)
![Coverage](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/<username>/<gist-id>/raw/lookout-coverage.json)
![CodeQL](https://github.com/<username>/lookout/actions/workflows/codeql.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/<username>/lookout)](https://goreportcard.com/report/github.com/<username>/lookout)
[![Docker](https://ghcr-badge.egpl.dev/<username>/lookout/latest_tag?label=Docker)](https://github.com/<username>/lookout/pkgs/container/lookout)
```

## Resources

- [GitHub Actions Documentation](https://docs.github.com/actions)
- [golangci-lint Documentation](https://golangci-lint.run/)
- [Semantic Versioning](https://semver.org/)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [Act - Local Testing](https://github.com/nektos/act)
