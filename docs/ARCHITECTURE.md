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

### CLI (`cmd/lookout`)
Command-line interface for vulnerability analysis and SBOM processing.

**Operations:**
- CVE lookup by ID
- CVE batch processing from file
- SBOM vulnerability scanning
- Dependency path traversal

### Web UI (`cmd/ui`)
Web-based interface built with Echo framework and Go templates.

**Features:**
- File upload for SBOM/CVE lists
- Interactive vulnerability browsing
- Dgraph visualization via Ratel
- Real-time dependency graph exploration

### Core Packages

#### `pkg/cli/cli_processor`
- Command-line argument parsing
- CVE data formatting and display
- Terminal output with color coding

#### `pkg/service`
- Business logic layer
- Orchestrates data flow between components
- Context management and timeouts

#### `pkg/repository`
- Data access layer
- Dgraph database operations
- Component and dependency storage

#### `pkg/validation`
- Input validation (CVE IDs, PURLs, file paths)
- Security checks (path traversal prevention)

#### `pkg/common`
- **nvd**: NVD API client with rate limiting
- **trivy**: Trivy scanner integration
- **cyclonedx**: CycloneDX SBOM parser
- **processor**: File processing utilities
- **fileutil**: Temporary file handling

#### `pkg/gui/dgraph`
- Dgraph database operations
- Component insertion and querying
- Dependency graph traversal
- Vulnerability tracking

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

### SBOM Processing Flow
```
SBOM File Upload
    ↓
Trivy Scanner (optional)
    ↓
CycloneDX Parser
    ↓
Component Extraction
    ↓
Dgraph Insertion
    ↓
NVD API Lookup
    ↓
Vulnerability Mapping
    ↓
Results Display
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

### Environment Variables
```bash
DGRAPH_HOST=alpha          # Dgraph server host
DGRAPH_PORT=9080           # Dgraph gRPC port
SERVER_PORT=3000           # Web server port
NVD_API_KEY=<optional>     # NVD API key for higher rate limits
```

### Build-time Variables
```bash
VERSION=1.0.0              # Application version
```

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
  lookout:    # Web application
  alpha:      # Dgraph Alpha (data)
  ratel:      # Dgraph UI
```

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
