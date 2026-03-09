# GitHub Actions Workflows

This directory contains CI/CD workflows for the Lookout project.

## Workflows

### 🔄 CI (`ci.yml`)
**Triggers:** Push to `main`/`develop`, Pull Requests

**Jobs:**
- **Test**: Runs tests on Go 1.26 with race detection
- **Lint**: Runs golangci-lint for code quality
- **Build**: Builds CLI and GUI binaries, uploads as artifacts
- **Security**: Runs Gosec security scanner, uploads SARIF results
- **Secrets**: Gitleaks scan for leaked secrets in git history

**Features:**
- Go 1.26
- Coverage reporting to Codecov
- Dependency caching for faster builds
- SARIF security reports

---

### 📊 Coverage (`coverage.yml`)
**Triggers:** Push to `main`, Pull Requests

**Jobs:**
- Generates detailed coverage reports
- Comments on PRs with coverage summary
- Creates coverage badge (main branch only)
- Checks coverage threshold (currently 25%)

**Requirements:**
- `GIST_SECRET`: GitHub token with gist permissions (for badge)
- `GIST_ID`: Gist ID for storing badge data

---

### 🐳 Docker (`docker.yml`)
**Triggers:** Push to `main`, Tags (`v*`), Pull Requests

**Jobs:**
- **build**: Builds and pushes Docker images to GitHub Container Registry
- **docker-compose**: Tests docker-compose setup

**Features:**
- Multi-platform builds with caching
- Automatic image tagging (branch, PR, semver, SHA)
- Trivy vulnerability scanning
- Health checks for services

**Published Images:**
- `ghcr.io/<username>/lookout:main` (latest main)
- `ghcr.io/<username>/lookout:v1.0.0` (version tags)
- `ghcr.io/<username>/lookout:<sha>` (commit SHA)

---

### 🚀 Release (`release.yml`)
**Triggers:** Tags matching `v*`

**Jobs:**
- **release**: Creates GitHub releases with binaries
- **publish-docker**: Publishes Docker images

**Features:**
- Multi-platform binary builds (Linux, macOS, Windows for AMD64/ARM64)
- Automatic changelog generation
- SHA256 checksums for all binaries
- Docker image publishing with `latest` tag

**Supported Platforms:**
- Linux: AMD64, ARM64
- macOS: AMD64 (Intel), ARM64 (Apple Silicon)
- Windows: AMD64

**Usage:**
```bash
git tag v1.0.0
git push origin v1.0.0
```

---

### 🤖 Dependabot (`../dependabot.yml`)
**Schedule:** Weekly (Mondays 6 AM UTC)

**Monitors:**
- Go modules (grouped by framework: Dgraph, Echo)
- GitHub Actions
- Docker base images

**Configuration:**
- Max 10 PRs for Go modules
- Max 5 PRs for GitHub Actions and Docker
- Auto-labels and conventional commit messages

---

## Setup Requirements

### Required Secrets
None required for basic functionality. Optional:

- `CODECOV_TOKEN`: For Codecov integration
- `GIST_SECRET`: For coverage badge
- `GIST_ID`: Gist ID for badge storage

### Repository Settings

1. **Enable GitHub Actions**
   - Settings → Actions → General → Allow all actions

2. **Enable Packages**
   - Settings → General → Features → Enable Packages

3. **Branch Protection** (Recommended)
   ```
   Settings → Branches → Add rule for "main"
   - Require pull request reviews
   - Require status checks to pass (ci/test, ci/lint, ci/build)
   - Require branches to be up to date
   ```

4. **Security**
   ```
   Settings → Security → Code security and analysis
   - Enable Dependabot alerts
   - Enable Dependabot security updates
   - Enable Secret scanning
   ```

---

## Badges

Add these to your README.md:

```markdown
![CI](https://github.com/<username>/lookout/actions/workflows/ci.yml/badge.svg)
![Coverage](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/<username>/<gist-id>/raw/lookout-coverage.json)
![Docker](https://github.com/<username>/lookout/actions/workflows/docker.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/<username>/lookout)](https://goreportcard.com/report/github.com/<username>/lookout)
```

---

## Local Testing

### Test CI Locally with Act
```bash
# Install act
brew install act  # macOS
# or download from https://github.com/nektos/act

# Run CI workflow
act -j test

# Run specific job
act -j lint
```

### Test Docker Build
```bash
docker build -t lookout:test .
docker-compose up
```

---

## Workflow Status

Check workflow status at:
`https://github.com/<username>/lookout/actions`

---

## Troubleshooting

### Coverage badge not updating
- Verify `GIST_SECRET` has gist permissions
- Check `GIST_ID` is correct
- Badge updates only on `main` branch pushes

### Docker push failing
- Ensure GitHub Packages is enabled
- Check repository has `write:packages` permission

### Release not creating
- Verify tag format: `v1.0.0` (must start with `v`)
- Check `contents: write` permission in workflow

### Tests failing in CI but passing locally
- Check Go version matches (1.26)
- Verify all dependencies are in `go.mod`
- Look for race conditions (CI runs with `-race`)
