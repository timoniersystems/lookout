# Architecture

Lookout is a CycloneDX SBOM vulnerability analysis tool with both CLI and web-based interfaces.

## System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        Lookout                              │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────┐              ┌──────────────┐               │
│  │   CLI    │              │   Web UI     │               │
│  │          │              │  (Echo + Go  │               │
│  │  -cve    │              │   Templates) │               │
│  │  -sbom   │              │              │               │
│  │  -traverse│              └──────┬───────┘               │
│  └────┬─────┘                     │                        │
│       │                           │                        │
│       └───────────┬───────────────┘                        │
│                   │                                        │
│            ┌──────▼─────────┐                              │
│            │ Service Layer   │                              │
│            │ - Validation    │                              │
│            │ - Processing    │                              │
│            └──────┬─────────┘                              │
│                   │                                        │
│         ┌─────────┼──────────┐                             │
│         │         │          │                             │
│    ┌────▼────┐ ┌──▼───┐ ┌───▼────┐                        │
│    │ NVD API │ │Trivy │ │ Dgraph │                        │
│    │         │ │      │ │  Graph │                        │
│    │CVE Data │ │Scanner│ │   DB   │                        │
│    └─────────┘ └──────┘ └────────┘                        │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Components

### CLI (`cmd/cli`)
Command-line interface for vulnerability analysis and SBOM processing.

**Operations:**
- CVE lookup by ID
- CVE batch processing from file
- SBOM vulnerability scanning
- Dependency path traversal

**Key Packages:**
- `pkg/cli/cli_processor` - Argument parsing and processing
- `pkg/cli/cli_processor/cve_formatter.go` - Formatted CVE output (refactored for DRY)

### Web UI (`cmd/ui`)
Web-based interface built with Echo framework and Go templates.

**Features:**
- File upload for SBOM/CVE lists
- Asynchronous SBOM processing with real-time progress tracking
- Interactive vulnerability browsing with severity filtering
- Session-based results storage (auto-expires after 1 hour)
- Dgraph visualization via Ratel
- Real-time dependency graph exploration
- TLS/HTTPS support via nginx reverse proxy

**Key Packages:**
- `pkg/ui/echo` - Echo server setup and routing
- `pkg/common/handler` - HTTP request handlers (async SBOM, progress, results)
- `pkg/common/progress` - Server-Sent Events (SSE) progress tracking

### Core Packages

#### `pkg/config`
- Environment-based configuration management
- Dgraph connection settings
- Server configuration
- NVD API configuration
- Structured configuration loading

#### `pkg/interfaces`
- Interface definitions for testability
- HTTPClient interface
- TrivyRunner interface
- VulnerabilityRepository interface
- Enables mocking in tests

#### `pkg/logging`
- Structured logging with levels (debug, info, warn, error)
- Consistent log format across application
- Context-aware logging

#### `pkg/validation`
- Input validation (CVE IDs, PURLs, file paths)
- Security checks (path traversal prevention)
- Format verification

#### `pkg/service`
- Business logic layer
- VulnerabilityService orchestrates data flow
- Context management and timeouts
- Error handling with context wrapping

#### `pkg/repository`
- Data access layer for Dgraph
- Component and dependency storage
- Dgraph query execution
- Graph traversal operations

#### `pkg/graph`
- Graph database client management
- DQL query execution
- Dependency path traversal algorithms
- Component relationship queries

#### `pkg/cli/cli_processor`
- Command-line argument parsing
- CVE data formatting and display
- Terminal output with color coding
- Refactored formatters (cve_formatter.go)

#### `pkg/common`
- **nvd**: NVD API client with rate limiting and retry logic
- **trivy**: Trivy scanner integration
- **cyclonedx**: CycloneDX SBOM parser with tests
- **processor**: File processing utilities (CVE lists, Trivy JSON)
- **fileutil**: Temporary file handling (refactored for DRY)
- **handler**: HTTP handlers (async processing, progress, results storage)
- **progress**: SSE progress tracking for long-running operations

## Data Flow

### CVE Lookup Flow
```
User Input (CVE-ID)
    ↓
Validation
    ↓
NVD API Client
    ↓
CVE Data Parser
    ↓
Formatter
    ↓
Terminal Output / Web Response
```

### SBOM Processing Flow (Web UI with Async Processing)
```
SBOM File Upload
    ↓
Generate Session ID
    ↓
Start Background Processing (goroutine)
    │
    ├─> Progress Updates (SSE) ─> User's Browser
    │
    ↓
Temporary File Creation
    ↓
Trivy Scanner (optional)
    ↓
CycloneDX Parser
    ↓
Component Extraction
    ↓
Dgraph Insertion (via Repository)
    ↓
NVD API Lookup (with rate limiting)
    ↓
Severity Filtering
    ↓
Dependency Path Tracing
    ↓
Results Storage (in-memory, session-based)
    ↓
Redirect to Results Page
```

### SBOM Processing Flow (CLI)
```
SBOM File Path
    ↓
Trivy Scanner (if installed)
    ↓
CycloneDX Parser
    ↓
Component Extraction
    ↓
Dgraph Insertion (if dependency tracing enabled)
    ↓
NVD API Lookup
    ↓
Vulnerability Mapping
    ↓
Terminal Output (formatted)
```

### Dependency Traversal Flow
```
Component PURL
    ↓
Dgraph Query (find component)
    ↓
Find Root Package
    ↓
BFS Path Search
    ↓
Path Extraction
    ↓
ASCII Tree Visualization
```

## Database Schema (Dgraph)

### Component Node
```graphql
type Component {
  bomRef: string @index(exact)
  reference: string @index(exact)
  name: string @index(exact)
  version: string @index(exact)
  purl: string @index(exact) @upsert
  dependsOn: [uid] @reverse
  vulnerable: bool @index(bool)
  cveID: string @index(exact)
  dgraphURL: string @index(exact)
  root: bool @index(bool)
}
```

### Relationships
- **dependsOn**: Edge to dependencies (reverse: ~dependsOn)
- **root**: Marker for root package
- **vulnerable**: Vulnerability flag
- **purl**: Package URL (unique identifier)

## External Dependencies

### NVD API
- **Purpose**: CVE vulnerability data
- **Rate Limits**:
  - Without API key: 5 requests/30 seconds
  - With API key: 50 requests/30 seconds
- **Retry Logic**: Exponential backoff with jitter

### Trivy
- **Purpose**: Container and SBOM vulnerability scanning
- **Format**: JSON output
- **Integration**: CLI subprocess

### Dgraph
- **Purpose**: Graph database for dependency relationships
- **Version**: v21.03+
- **Deployment**: Docker container (alpha node)

## Security Considerations

### Input Validation
- CVE ID format validation
- PURL format validation
- Path traversal prevention
- File type restrictions

### API Safety
- NVD API rate limiting
- Request retry with backoff
- Timeout management
- Error wrapping with context

### Database Safety
- Connection pooling via client manager
- Context-aware operations
- Graceful error handling
- Schema enforcement

## Configuration

Configuration is managed via the `pkg/config` package, which loads settings from environment variables.

### Environment Variables
```bash
# Server Configuration
SERVER_PORT=3000           # Web server port (internal, proxied by nginx)

# Dgraph Configuration
DGRAPH_HOST=alpha          # Dgraph server hostname
DGRAPH_PORT=9080           # Dgraph gRPC port
DGRAPH_MAX_RETRIES=5       # Maximum retry attempts for Dgraph operations
DGRAPH_RETRY_DELAY_SECONDS=2  # Delay between retries
DGRAPH_MAX_TRAVERSAL_DEPTH=10 # Maximum depth for dependency traversal

# NVD API Configuration
NVD_API_KEY=<optional>     # NVD API key for higher rate limits (highly recommended)

# Logging
LOG_LEVEL=info             # Log level: debug, info, warn, error
```

### Build-time Variables
```bash
VERSION=1.0.0              # Application version
GO_VERSION=1.26.0          # Go version for Docker builds
TRIVY_VERSION=0.69.1       # Trivy version for Docker
# Note: Runtime uses distroless/static-debian12 for minimal attack surface
```

### Configuration Loading
The application uses structured configuration loading via `pkg/config/config.go`:
- Environment variables override defaults
- Validation on startup
- Type-safe configuration struct
- Documented defaults

## Scalability Considerations

### Current Limitations
- Single Dgraph instance (no HA)
- Synchronous NVD API calls
- In-memory component processing

### Future Enhancements
- Dgraph clustering for HA
- Redis caching for NVD responses
- Background job processing
- Concurrent API requests with worker pools

## Deployment

### Docker Compose
```yaml
services:
  nginx:         # Reverse proxy with TLS termination (ports 7080, 7443)
  lookout:       # Web application (internal port 3000)
  dgraph-alpha:  # Dgraph Alpha (data server)
  dgraph-zero:   # Dgraph Zero (cluster coordinator)
  dgraph-ratel:  # Dgraph UI (port 8000)
```

**Access Points:**
- **Web UI (HTTPS):** https://localhost:7443
- **Web UI (HTTP):** http://localhost:7080 (redirects to HTTPS)
- **Dgraph Ratel UI:** http://localhost:8000
- **Dgraph HTTP API:** http://localhost:8080
- **Dgraph gRPC API:** localhost:9080

**Security Features:**
- TLS 1.2/1.3 only via nginx
- HTTP to HTTPS redirect
- Security headers (HSTS, CSP, X-Frame-Options)
- Rate limiting on upload endpoints
- Gzip compression
- Self-signed certificates for development (via scripts/generate-certs.sh)

### Standalone CLI
Binary can run independently without Dgraph for:
- CVE lookups
- File processing
- SBOM scanning (with Trivy)

SBOM dependency traversal requires Dgraph.

## Testing Strategy

- **Unit Tests**: Package-level logic
- **Integration Tests**: Service layer with mocks
- **E2E Tests**: Docker Compose setup
- **Coverage Target**: 80% (current: 25.4%)

See [DEVELOPMENT.md](DEVELOPMENT.md) for testing guide.
