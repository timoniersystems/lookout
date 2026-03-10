# Usage Guide

Complete guide to using Lookout for CVE vulnerability analysis and SBOM processing.

## Quick Start

### CLI Usage

```bash
# Single CVE lookup
lookout cve CVE-2021-44228

# Process CVE list from file
lookout cve-file vulnerabilities.txt

# Scan SBOM for vulnerabilities
lookout sbom mybom.json

# Find dependency path for vulnerable package
lookout sbom mybom.json --dep-path "pkg:npm/lodash@4.17.20"
```

### Web UI

```bash
# Generate TLS certificates (first time only)
make certs

# Start all services
make up

# Access UI (HTTPS - recommended)
open https://localhost:7443

# Or via HTTP (redirects to HTTPS)
open http://localhost:7080
```

## Installation

### Docker (Recommended)

```bash
# Clone repository
git clone https://github.com/timoniersystems/lookout.git
cd lookout

# Generate TLS certificates (first time only)
make certs

# Start services
make up

# Access web UI (HTTPS)
open https://localhost:7443
```

### Binary Download

Download from [Releases](https://github.com/timoniersystems/lookout/releases):

```bash
# Linux AMD64
wget https://github.com/timoniersystems/lookout/releases/download/v1.0.0/lookout-linux-amd64
chmod +x lookout-linux-amd64
sudo mv lookout-linux-amd64 /usr/local/bin/lookout

# macOS ARM64 (Apple Silicon)
wget https://github.com/timoniersystems/lookout/releases/download/v1.0.0/lookout-darwin-arm64
chmod +x lookout-darwin-arm64
sudo mv lookout-darwin-arm64 /usr/local/bin/lookout

# Verify installation
lookout version
```

### Build from Source

```bash
git clone https://github.com/timoniersystems/lookout.git
cd lookout
make build
make install
```

## CLI Reference

The CLI uses subcommands. Global flags (`--severity`, `--debug`) apply to all subcommands.

```
lookout [--severity <level>] [--debug] <command> [args] [flags]
```

### Commands

#### `cve` — Single CVE Lookup

```bash
lookout cve CVE-2021-44228
```

Output:
```
Fetching data for CVE: CVE-2021-44228

══════════════════════════════════════════════════════════════════════
  CVE ANALYSIS - CRITICAL
══════════════════════════════════════════════════════════════════════

  🔴  CVE-2021-44228

  Severity:    CRITICAL
  Score:       10.0/10.0
  ...
```

#### `cve-file` — Batch CVE Processing

Create `cves.txt`:
```
CVE-2021-44228
CVE-2022-23305
CVE-2022-23307
```

Run:
```bash
lookout cve-file cves.txt
```

#### `sbom` — SBOM Scanning

**Basic scan:**
```bash
lookout sbom path/to/sbom.json
```

This will:
1. Parse the SBOM
2. Run Trivy scanner (if installed)
3. Fetch CVE data from NVD
4. Display vulnerabilities

**With output file:**
```bash
lookout sbom mybom.json --output vulnerabilities.json
```

**Supported SBOM formats:**
- CycloneDX 1.4+ (JSON)
- SPDX 2.3+ (JSON)

#### `sbom --dep-path` — Dependency Traversal

Find how a vulnerable package is included in your project:

```bash
lookout sbom mybom.json --dep-path "pkg:npm/lodash@4.17.20"
```

Output:
```
══════════════════════════════════════════════════════════════════════
  DEPENDENCY PATH ANALYSIS
══════════════════════════════════════════════════════════════════════

  Searched: pkg:npm/lodash@4.17.20
  Depth:    3 level(s)

  Dependency Tree:

     🏠 pkg:npm/myapp@1.0.0
     │
     └──> 📦 pkg:npm/mocha@8.4.0
          │
          └──> 📦 pkg:npm/async@2.6.3
               │
               └──> ⚠️  pkg:npm/lodash@4.17.20

══════════════════════════════════════════════════════════════════════

  Legend:
    🏠  = Root package (your application)
    📦  = Intermediate dependency
    ⚠️  = Vulnerable component
```

#### `version` — Show Version

```bash
lookout version
```

### Global Flags

| Flag | Description | Default | Example |
|------|-------------|---------|---------|
| `--severity` | Minimum severity to display | `high` | `--severity critical` |
| `--debug` | Enable debug logging | `false` | `--debug` |
| `--help` | Show help | | `lookout --help`, `lookout sbom --help` |

### `sbom` Subcommand Flags

| Flag | Description | Example |
|------|-------------|---------|
| `--output <file>` | Save Trivy results to file | `--output results.json` |
| `--dep-path <purl>` | Trace transitive dependency path | `--dep-path "pkg:npm/lodash@4.17.20"` |

### Severity Levels

```bash
# Show only critical
lookout --severity critical cve CVE-2021-44228

# Show high and critical (default)
lookout --severity high sbom mybom.json

# Show medium and above
lookout --severity medium sbom mybom.json

# Show everything
lookout --severity all sbom mybom.json
```

### Input File Formats

**CVE List (text file):**
```
CVE-2021-44228
CVE-2022-23305
CVE-2022-23307
```

**CycloneDX SBOM (JSON):**
```json
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.4",
  "components": [
    {
      "name": "lodash",
      "version": "4.17.20",
      "purl": "pkg:npm/lodash@4.17.20"
    }
  ]
}
```

## Web UI Guide

### Starting the Server

```bash
# Generate TLS certificates (first time only)
make certs

# Start all services
make up
```

Access at: https://localhost:7443 (or http://localhost:7080 which redirects to HTTPS)

**Note:** Browser will show a security warning for self-signed certificates in development. Click "Advanced" and "Proceed" to continue.

### Features

#### 1. CVE Lookup

**Single CVE:**
1. Navigate to home page
2. Enter CVE ID (e.g., `CVE-2021-44228`)
3. Click "Lookup CVE"
4. View detailed vulnerability information

**Batch Upload:**
1. Click "Choose file..." under Batch CVE File
2. Select text file with CVE IDs or Trivy JSON
3. Click "Process File"
4. View aggregated results

#### 2. SBOM Analysis with Progress Tracking

1. Click "Choose SBOM..." under Scan SBOM section
2. Select CycloneDX or SPDX JSON file
3. Select severity filters (CRITICAL, HIGH, MEDIUM, LOW)
4. Click "Analyze SBOM"
5. **Real-time progress tracking:**
   - Uploading SBOM
   - Parsing Components
   - Building Dependency Graph
   - Scanning for Vulnerabilities
   - Fetching CVE Data
   - Tracing Dependency Paths
6. View vulnerability results with dependency paths

**SBOM Analysis Features:**
- **Async Processing:** Long-running scans in background
- **Progress Updates:** Real-time SSE progress tracking
- **Severity Filtering:** Filter by CRITICAL, HIGH, MEDIUM, LOW
- **Session Storage:** Results cached for 1 hour
- **Dependency Path Tracing:** See full path from vulnerable package to root

#### 3. Dependency Visualization

1. Upload SBOM (as above)
2. Click on any vulnerable component
3. View dependency graph
4. See path to root package

#### 4. Dgraph Explorer

Access Ratel UI at: http://localhost:8000

Query examples:
```graphql
# Find all vulnerable components
{
  vulnerable(func: eq(vulnerable, true)) {
    name
    version
    purl
    cveID
  }
}

# Find component dependencies
{
  component(func: eq(purl, "pkg:npm/lodash@4.17.20")) {
    name
    dependsOn {
      name
      version
    }
  }
}
```

## Workflows

### Workflow 1: Assess Single CVE

**Scenario:** You heard about Log4Shell and want details.

```bash
# Get CVE details
lookout cve CVE-2021-44228

# Check if you're affected
lookout sbom your-app.json | grep -i log4j
```

### Workflow 2: Continuous Vulnerability Monitoring

**Setup:**
1. Generate SBOM regularly
2. Scan with Lookout
3. Track changes over time

```bash
# Generate SBOM (example with Syft)
syft packages dir:. -o cyclonedx-json > sbom.json

# Scan for vulnerabilities
lookout sbom sbom.json --output vuln-$(date +%Y%m%d).json

# Compare with previous
diff vuln-20240101.json vuln-$(date +%Y%m%d).json
```

### Workflow 3: Dependency Path Investigation

**Scenario:** Found a vulnerability, need to know how it got included.

```bash
# 1. Scan SBOM and find vulnerable package
lookout sbom mybom.json

# 2. Trace dependency path
lookout sbom mybom.json --dep-path "pkg:npm/minimist@1.2.5"

# 3. Review the path
# 4. Decide: upgrade parent dependency or exclude vulnerable package
```

### Workflow 4: Pre-Release Security Check

**Before deploying:**

```bash
# 1. Generate latest SBOM
syft packages . -o cyclonedx-json > release-sbom.json

# 2. Scan for high/critical vulnerabilities
lookout sbom release-sbom.json --severity high

# 3. Block release if critical vulnerabilities found
# 4. Document known vulnerabilities if acceptable risk
```

## Integration Examples

### CI/CD Integration

**GitHub Actions:**
```yaml
- name: Security Scan
  run: |
    # Generate SBOM
    syft packages . -o cyclonedx-json > sbom.json

    # Scan with Lookout
    docker run --rm -v $(pwd):/work \
      ghcr.io/timoniersystems/lookout:latest \
      sbom /work/sbom.json --severity high

    # Fail if critical vulnerabilities
    if [ $? -ne 0 ]; then
      echo "Critical vulnerabilities found!"
      exit 1
    fi
```

### API Usage

**Start server:**
```bash
make certs  # first time only
make up
```

**API endpoints (via HTTPS):**

```bash
# Upload and analyze SBOM (async)
curl -k -X POST https://localhost:7443/upload-cyclonedx-bom \
  -F "cyclonedx-bom-file=@sbom.json" \
  -F "severity=CRITICAL" \
  -F "severity=HIGH"

# Process CVE from form input
curl -k -X POST https://localhost:7443/process \
  -F "cveID=CVE-2021-44228"

# Process CVE file
curl -k -X POST https://localhost:7443/upload \
  -F "file=@cve-list.txt"

# Get SBOM analysis results by session ID
curl -k https://localhost:7443/results/{sessionId}

# Health check
curl -k https://localhost:7443/health
```

**Note:** Use `-k` flag to accept self-signed certificates in development. In production, remove `-k` and use valid TLS certificates.

## Best Practices

### 1. Regular Scanning

- Scan SBOMs on every build
- Set up automated alerts
- Track vulnerability trends

### 2. Use NVD API Key

Get API key from https://nvd.nist.gov/developers/request-an-api-key

```bash
export NVD_API_KEY=your_key_here
lookout cve CVE-2021-44228
```

Benefits:
- 50 requests/30s (vs 5 without key)
- Faster batch processing
- Reduced rate limiting

### 3. Severity Filtering

Focus on what matters:
```bash
# Production: Only high/critical
lookout sbom prod.json --severity high

# Development: All severities
lookout sbom dev.json --severity all
```

### 4. SBOM Generation

Use quality SBOM generators:
- [Syft](https://github.com/anchore/syft)
- [CycloneDX Maven Plugin](https://github.com/CycloneDX/cyclonedx-maven-plugin)
- [CycloneDX Node Module](https://github.com/CycloneDX/cyclonedx-node-module)

### 5. Dependency Pinning

After finding vulnerable paths:
1. Pin direct dependencies
2. Use lock files (package-lock.json, go.sum)
3. Regular dependency updates

## Troubleshooting

### "Trivy not found"

Install Trivy:
```bash
brew install trivy  # macOS
# or: https://aquasecurity.github.io/trivy/latest/getting-started/installation/
```

Or skip Trivy scan by passing pre-scanned data:
```bash
lookout cve-file vulnerabilities.txt
```

### Rate Limiting

**Error:** `429 Too Many Requests`

**Solutions:**
1. Use NVD API key
2. Add delays between requests
3. Process in smaller batches

### Dgraph Connection Error

```bash
# Check if Dgraph is running
docker compose ps

# Restart Dgraph
docker compose restart dgraph-alpha

# View logs
docker compose logs dgraph-alpha
```

### Invalid SBOM Format

**Error:** `Failed to parse SBOM`

**Check:**
- Is it valid JSON?
- For CycloneDX: Is `bomFormat` = `"CycloneDX"`? Is `specVersion` 1.4+?
- For SPDX: Is `spdxVersion` = `"SPDX-2.3"` or later? Are there packages with PURLs in `externalRefs`?

**Validate:**
```bash
cat sbom.json | jq .
cat sbom.json | jq '.bomFormat'   # CycloneDX
cat sbom.json | jq '.spdxVersion' # SPDX
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `NVD_API_KEY` | NVD API key | None |
| `DGRAPH_HOST` | Dgraph hostname | `alpha` |
| `DGRAPH_PORT` | Dgraph port | `9080` |
| `SERVER_PORT` | Web server port | `3000` |
| `LOG_LEVEL` | Log level (debug/info/warn/error) | `info` |

## Examples

See [examples/](../examples/) directory for:
- Sample SBOM files
- CVE lists
- Integration scripts
- Test data

## FAQ

**Q: How often is NVD data updated?**
A: Real-time. Lookout fetches directly from NVD API.

**Q: Can I use without Docker?**
A: Yes, for CLI-only. Dependency traversal requires Dgraph (Docker).

**Q: What SBOM formats are supported?**
A: CycloneDX 1.4+ (JSON) and SPDX 2.3+ (JSON). The format is auto-detected on upload.

**Q: Is it free?**
A: Yes, open source under [LICENSE](../LICENSE).

**Q: Can I run offline?**
A: No, requires NVD API access for CVE data.

## Getting Help

- **Documentation**: [docs/](.)
- **Issues**: [GitHub Issues](https://github.com/timoniersystems/lookout/issues)
- **Discussions**: [GitHub Discussions](https://github.com/timoniersystems/lookout/discussions)
