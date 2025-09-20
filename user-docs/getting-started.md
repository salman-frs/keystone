# Getting Started

This guide helps you get up and running with the Keystone Security Platform after completing the initial setup.

## First Steps

### 1. Verify Installation

Ensure all services are running:

```bash
# Check service status
docker-compose ps

# Verify API connectivity
curl http://localhost:8080/health

# Access dashboard
open http://localhost:3000
```

### 2. Basic Configuration

The platform uses configuration files for different environments:

```bash
# Development configuration
./infrastructure/policies/dev/

# Staging configuration  
./infrastructure/policies/staging/

# Production configuration
./infrastructure/policies/prod/
```

## Core Concepts

### Vulnerability Correlation

Keystone aggregates vulnerability data from multiple scanners:

- **Trivy**: Container and filesystem scanning
- **Grype**: Package vulnerability detection  
- **Snyk**: Dependency vulnerability analysis

### Policy Enforcement

Security policies are defined using Open Policy Agent (OPA):

```rego
# Example policy: infrastructure/policies/dev/basic-security.rego
package security

deny[msg] {
    input.vulnerabilities[_].severity == "CRITICAL"
    msg := "Critical vulnerabilities detected"
}
```

### Cryptographic Attestation

All security artifacts are signed using Sigstore:

- SBOM attestations
- Scan result signatures
- Policy compliance records

## Basic Workflows

### 1. Security Scanning

```bash
# Manual scan (development)
./scripts/scan.sh

# View results in dashboard
open http://localhost:3000/vulnerabilities
```

### 2. Policy Evaluation  

```bash
# Test policy against scan results
./scripts/policy-check.sh ./examples/scan-results.json

# View policy status
open http://localhost:3000/policies
```

### 3. Attestation Generation

```bash
# Generate and sign attestation
./scripts/attest.sh

# Verify attestation
./scripts/verify.sh
```

## Dashboard Navigation

### Security Overview

- **Dashboard Home**: High-level security metrics
- **Vulnerabilities**: Detailed vulnerability analysis
- **Policies**: Policy compliance status
- **Attestations**: Cryptographic verification records

### Real-time Updates

The dashboard provides live updates via Server-Sent Events:

- Scan progress notifications
- Policy evaluation results
- Remediation recommendations

## Example Scenarios

### Scenario 1: Container Security Scan

```bash
# Build demo container
cd examples/vulnerable-app
docker build -t demo-app .

# Scan with Keystone
cd ../..
./scripts/scan.sh demo-app

# Review results in dashboard
```

### Scenario 2: Policy Development

```bash
# Create custom policy
cp infrastructure/policies/templates/basic-security.rego \
   infrastructure/policies/dev/custom-policy.rego

# Edit policy rules
vim infrastructure/policies/dev/custom-policy.rego

# Test policy
./scripts/policy-test.sh
```

### Scenario 3: CI/CD Integration

```bash
# Add to GitHub Actions workflow
cat >> .github/workflows/security.yml << EOF
- name: Keystone Security Scan
  uses: ./
  with:
    scan-target: \${{ github.workspace }}
    policy-path: infrastructure/policies/prod/
EOF
```

## Configuration Reference

### Environment Variables

```bash
# API Configuration
KEYSTONE_API_PORT=8080
KEYSTONE_DATABASE_PATH=./data/keystone.db

# Dashboard Configuration  
VITE_API_URL=http://localhost:8080
VITE_ENVIRONMENT=development

# GitHub Integration
GITHUB_TOKEN=<your-token>
GITHUB_CLIENT_ID=<oauth-client-id>
```

### Policy Configuration

```yaml
# infrastructure/policies/config.yaml
scanners:
  trivy:
    enabled: true
    config: ./trivy.yaml
  grype:
    enabled: true
    config: ./grype.yaml

thresholds:
  critical: 0
  high: 5
  medium: 20
```

## Troubleshooting

### Common Issues

**Scan failures:**
```bash
# Check scanner logs
docker-compose logs api

# Verify scanner configuration
./scripts/verify-scanners.sh
```

**Policy evaluation errors:**
```bash
# Validate policy syntax
opa fmt infrastructure/policies/dev/*.rego

# Test policy evaluation
opa eval -d infrastructure/policies/dev/ "data.security.deny"
```

**Dashboard connection issues:**
```bash
# Check API connectivity
curl -v http://localhost:8080/api/v1/health

# Verify environment configuration
cat apps/dashboard/.env.local
```

## Next Steps

1. **Explore Examples**: Review [examples/](../examples/) directory
2. **API Integration**: Read [API Documentation](api/)
3. **Production Deployment**: Follow [Deployment Guide](deployment/)
4. **Security Model**: Understand [Security Architecture](security/)

## Additional Resources

- [Troubleshooting Guide](troubleshooting.md)
- [Contributing Guidelines](contributing.md)
- [API Reference](api/)
- [Examples and Tutorials](examples/)