# Vulnerable Demo Application

A deliberately vulnerable Go microservice designed for testing security scanning workflows. This application contains known vulnerabilities across multiple ecosystems for comprehensive security testing.

## Overview

This demo application serves as a realistic target for vulnerability scanning, SBOM generation, and security policy evaluation. It includes intentional vulnerabilities spanning Go modules, OS packages, and container base images.

## Application Endpoints

- `GET /health` - Health check endpoint returning application status and metadata
- `GET /version` - Version information including dependency versions
- `GET /ping` - Legacy ping endpoint
- `GET /config` - Vulnerable endpoint exposing configuration (YAML parsing without validation)
- `GET /ws` - Vulnerable WebSocket endpoint with unrestricted origin policy

## Known Vulnerabilities

### Critical Severity

#### CVE-2023-45288 - golang.org/x/net HTTP/2 Memory Exhaustion
- **Package**: golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
- **Type**: Denial of Service, Memory Exhaustion
- **CVSS**: 7.5 (High/Critical)
- **Scanner Detection**: Both Trivy and Grype detect this vulnerability
- **Remediation**: Upgrade to golang.org/x/net v0.17.0 or later

#### CVE-2023-39325 - golang.org/x/net HTTP/2 Rapid Reset Attack
- **Package**: golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
- **Type**: Denial of Service
- **CVSS**: 7.5 (High)
- **Scanner Detection**: Trivy and Grype both detect
- **Remediation**: Upgrade to golang.org/x/net v0.17.0 or later

### High Severity

#### CVE-2022-27191 - golang.org/x/crypto SSH Authentication Bypass
- **Package**: golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
- **Type**: Authentication Bypass
- **CVSS**: 7.4 (High)
- **Scanner Detection**: Both scanners detect this in SSH package
- **Remediation**: Upgrade to golang.org/x/crypto v0.0.0-20220314234659-1baeb1ce4c0b

#### CVE-2021-44716 - net/http Request Smuggling
- **Package**: Go standard library (Go 1.19)
- **Type**: HTTP Request Smuggling
- **CVSS**: 7.5 (High)
- **Scanner Detection**: Detected by both Trivy and Grype
- **Remediation**: Upgrade to Go 1.17.6+ or 1.18.1+

### Medium Severity

#### CVE-2022-28948 - gopkg.in/yaml.v3 Stack Overflow
- **Package**: gopkg.in/yaml.v2 v2.4.0 (indirect through gopkg.in/yaml.v3)
- **Type**: Stack Overflow, Denial of Service
- **CVSS**: 6.5 (Medium)
- **Scanner Detection**: Grype detects more reliably than Trivy
- **Remediation**: Upgrade to gopkg.in/yaml.v3 v3.0.1

#### CVE-2022-32149 - golang.org/x/text Path Traversal
- **Package**: golang.org/x/text v0.3.6
- **Type**: Path Traversal
- **CVSS**: 6.5 (Medium)
- **Scanner Detection**: Both scanners detect
- **Remediation**: Upgrade to golang.org/x/text v0.3.8

### OS Package Vulnerabilities

#### Ubuntu 18.04 Runtime Base Image CVEs
- **curl 7.58.0-2ubuntu3.24**: Multiple CVEs including CVE-2021-22947, CVE-2021-22946
- **openssl 1.1.1-1ubuntu2.1~18.04.23**: CVE-2021-3712, CVE-2021-3711
- **libc6 2.27-3ubuntu1.6**: CVE-2021-33560, CVE-2021-35942

#### Alpine Build Stage (golang:1.21-alpine)
- **No significant vulnerabilities**: Uses recent Alpine packages (OpenSSL 3.3.1)
- **Note**: Build-time packages are not included in final runtime image

## Scanner Detection Expectations

### Trivy Detection
```bash
# Expected vulnerability count by severity
CRITICAL: 2-3 vulnerabilities
HIGH: 4-6 vulnerabilities
MEDIUM: 8-12 vulnerabilities
LOW: 15-25 vulnerabilities
```

### Grype Detection
```bash
# Expected vulnerability count by severity
Critical: 2-3 vulnerabilities
High: 4-6 vulnerabilities
Medium: 8-12 vulnerabilities
Low: 15-25 vulnerabilities
```

### Scanner-Specific Variations
- **Trivy**: Better at detecting OS package vulnerabilities in Ubuntu base
- **Grype**: More comprehensive Go module vulnerability detection
- **Common**: Both detect the major golang.org/x/net and golang.org/x/crypto issues
- **False Positives**: Some low-severity alerts may vary between scanners

## SBOM Generation Expected Output

### Syft SBOM Components
- **Go Modules**: 15-20 direct and indirect dependencies
- **OS Packages**: 200+ packages from Ubuntu 20.04 base image
- **Container Layers**: Multi-stage build artifacts and metadata
- **Application Binary**: Main application with build metadata

### Expected SBOM Formats
- **SPDX JSON**: Complete dependency tree with license information
- **CycloneDX JSON**: Vulnerability-focused format with component relationships
- **Syft JSON**: Detailed package catalog with layer information

## Vulnerability Remediation Guide

### Immediate Actions
1. Upgrade Go dependencies to latest stable versions
2. Update base image to Ubuntu 22.04 or Alpine 3.18+
3. Pin specific package versions to avoid regression

### Dependency Updates
```go
// go.mod remediation
require (
    github.com/gin-gonic/gin v1.9.1
    github.com/gorilla/websocket v1.5.0
    golang.org/x/crypto v0.14.0
    golang.org/x/net v0.17.0
    golang.org/x/text v0.14.0
    gopkg.in/yaml.v3 v3.0.1
)
```

### Container Updates
```dockerfile
# Remediated Dockerfile base images
FROM golang:1.21-alpine3.18 AS builder
FROM ubuntu:22.04
```

## Testing and Validation

### Local Testing
```bash
# Build and run application
docker build -t vulnerable-app .
docker run -p 8080:8080 vulnerable-app

# Test health endpoint
curl http://localhost:8080/health

# Scan for vulnerabilities
trivy image vulnerable-app
grype vulnerable-app

# Generate SBOM
syft vulnerable-app -o spdx-json
```

### Expected Health Check Response
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "5m30s",
  "timestamp": "2024-01-15T10:30:00Z",
  "metadata": {
    "go_version": "go1.19",
    "arch": "amd64",
    "os": "linux"
  }
}
```

### Performance Requirements
- **Health Check Response**: < 200ms
- **Application Startup**: < 5 seconds
- **Container Build**: < 2 minutes
- **Memory Usage**: < 50MB runtime

## Integration with Security Pipeline

This application integrates with the existing security workflow:
- **CI Scanning**: Automated vulnerability detection in GitHub Actions
- **SBOM Generation**: Complete dependency tree analysis
- **Policy Evaluation**: OPA policy testing against vulnerability data
- **Remediation Testing**: Validation of security fixes

## Development Notes

### Vulnerability Selection Rationale
- **Ecosystem Diversity**: Covers Go modules, OS packages, container layers
- **Severity Range**: Critical to Low severity for comprehensive testing
- **Scanner Compatibility**: Ensures detection by both Trivy and Grype
- **Real-World Relevance**: Uses actual CVEs found in production environments
- **Remediation Complexity**: Mix of simple updates and architectural changes

### Security Testing Considerations
- **Isolation**: Run only in development/testing environments
- **Network Security**: Vulnerable WebSocket and HTTP endpoints
- **Data Exposure**: Configuration endpoint exposes sensitive YAML data
- **Authentication**: No authentication mechanisms (intentionally vulnerable)
- **Input Validation**: Minimal validation for vulnerability demonstration