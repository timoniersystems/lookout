# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.9.x   | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in Lookout, please report it responsibly using [GitHub's private vulnerability reporting](https://github.com/timoniersystems/lookout/security/advisories/new).

**Please do not open a public issue for security vulnerabilities.**

### What to include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### What to expect

- **Acknowledgment** within 48 hours
- **Status update** within 7 days with an assessment and remediation plan
- **Fix timeline** depends on severity:
  - Critical/High: patch release within 7 days
  - Medium: included in the next minor release
  - Low: included in the next release

### Scope

The following are in scope:
- Lookout application code (CLI and web UI)
- Docker image configuration
- Helm chart templates
- GitHub Actions workflows

The following are out of scope:
- Third-party dependencies (report upstream, but let us know so we can track)
- Infrastructure you deploy yourself (your Kubernetes cluster, AWS account, etc.)

## Security Measures

This project uses:
- [Gosec](https://github.com/securego/gosec) for Go static security analysis
- [Trivy](https://github.com/aquasecurity/trivy) for container vulnerability scanning
- [Gitleaks](https://github.com/gitleaks/gitleaks) for secret detection
- [Dependabot](https://docs.github.com/en/code-security/dependabot) for dependency updates
- Least-privilege GitHub Actions permissions
- Distroless container runtime (no shell, no package manager)
