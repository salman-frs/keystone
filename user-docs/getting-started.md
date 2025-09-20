# Getting Started

This guide covers prerequisite installation and basic workflows for the Keystone Security Platform after completing the initial setup from the [Setup Guide](setup.md).

## First Steps

### 1. Verify Prerequisites Installation

Ensure all prerequisite tools are properly installed:

```bash
# Verify Go installation (required: 1.21+)
go version

# Verify GitHub CLI authentication
gh auth status

# Verify Docker via Colima
docker --version
colima status

# Verify security tools
trivy --version
grype version
syft version
cosign version
```

### 2. GitHub Authentication Setup

Configure GitHub CLI for repository access and API integration:

```bash
# Login to GitHub (if not already done)
gh auth login

# Select authentication method: Browser or Token
# Choose: HTTPS protocol
# Authorize GitHub CLI for repository access

# Verify access to Keystone repository
gh repo view salman-frs/keystone

# Test API access
gh api user
```

### 3. Install Kind (Kubernetes in Docker)

Optional: Install kind for container orchestration testing:

```bash
# Install kind via Homebrew
brew install kind

# Verify installation
kind version

# Create test cluster (optional)
kind create cluster --name keystone-test

# Check cluster status
kubectl cluster-info --context kind-keystone-test

# Clean up test cluster
kind delete cluster --name keystone-test
```

### 4. Node.js and Go Development Environment

Ensure development tools are properly configured:

```bash
# Verify Go installation and workspace
go version
go env GOPATH
go env GOROOT

# Install Node.js (for future dashboard development)
brew install node@18

# Verify Node.js installation
node --version
npm --version

# Optional: Install development tools
npm install -g typescript
npm install -g @types/node
```

## Core Security Components (Implemented)

### Multi-Scanner Vulnerability Detection

Keystone integrates multiple vulnerability scanners for comprehensive coverage:

- **Trivy**: Container, filesystem, and dependency scanning with CVE database
- **Grype**: Package vulnerability detection with anchore database
- **Correlation Framework**: Capability to compare and merge findings from multiple scanners

### SBOM Generation and Management

Software Bill of Materials (SBOM) creation using industry-standard formats:

- **Syft Integration**: Generates SPDX and CycloneDX format SBOMs
- **Artifact Storage**: GitHub Container Registry for secure SBOM storage
- **Validation Workflows**: Automated SBOM content verification

### GitHub Actions Security Pipeline

Automated security workflows integrated into CI/CD:

- **Trigger Events**: Push, pull request, and release events
- **Scanner Execution**: Parallel vulnerability scanning with multiple tools
- **Artifact Management**: Automated upload to GitHub Container Registry
- **Workflow Validation**: Testing scripts for local development

### Future Components (Planned)

- **Policy Enforcement**: Open Policy Agent (OPA) integration for security policies
- **Cryptographic Attestation**: Sigstore keyless signing for supply chain security
- **Real-time Dashboard**: React-based security visualization interface

## Basic Workflows (Current Implementation)

### 1. Manual Vulnerability Scanning

```bash
# Scan vulnerable demo application with Trivy
trivy fs examples/vulnerable-app/ --format json --output scanner-test-output/trivy-manual.json

# Scan with Grype
grype examples/vulnerable-app/ -o json > scanner-test-output/grype-manual.json

# View scan results
cat scanner-test-output/trivy-manual.json | jq '.Results[].Vulnerabilities | length'
cat scanner-test-output/grype-manual.json | jq '.matches | length'
```

### 2. SBOM Generation Workflow

```bash
# Generate SPDX format SBOM
syft examples/vulnerable-app/ -o spdx-json=sbom-test-output/manual-spdx.json

# Generate CycloneDX format SBOM
syft examples/vulnerable-app/ -o cyclonedx-json=sbom-test-output/manual-cyclonedx.json

# Validate SBOM content
cat sbom-test-output/manual-spdx.json | jq '.packages[].name'
cat sbom-test-output/manual-cyclonedx.json | jq '.components[].name'
```

### 3. Automated Workflow Testing

```bash
# Run comprehensive workflow validation
./scripts/setup/test-workflow.sh

# Test individual components
./scripts/setup/validate-scanners.sh
./scripts/setup/test-sbom-workflow.sh
./scripts/setup/test-vulnerable-app.sh
```

### 4. GitHub Actions Integration

```bash
# View workflow configuration
cat .github/workflows/security-pipeline.yaml

# Test workflow locally (if act is installed)
brew install act
act push

# Monitor workflow runs
gh run list
gh run view [run-id]
```

## Security Tool Configuration

### Trivy Configuration

Trivy scans containers, filesystems, and dependencies:

```bash
# Update Trivy vulnerability database
trivy image --download-db-only

# Scan with specific severity levels
trivy fs examples/vulnerable-app/ --severity HIGH,CRITICAL

# Generate detailed JSON report
trivy fs examples/vulnerable-app/ --format json --output detailed-trivy.json

# View database info
trivy --cache-dir ~/.cache/trivy version
```

### Grype Configuration

Grype focuses on package vulnerability detection:

```bash
# Update Grype vulnerability database
grype db update

# Scan with output formatting
grype examples/vulnerable-app/ -o table
grype examples/vulnerable-app/ -o json

# Check database status
grype db status
```

### Syft Configuration

Syft generates software bill of materials:

```bash
# List available output formats
syft --help | grep -A 10 "output formats"

# Generate with specific cataloger
syft examples/vulnerable-app/ --catalogers go-module

# Include file metadata
syft examples/vulnerable-app/ -o spdx-json --file-metadata
```

## Practical Security Scenarios

### Scenario 1: Vulnerable Application Analysis

```bash
# Build and scan vulnerable demo application
cd examples/vulnerable-app
go build -o vulnerable-app .
cd ../..

# Multi-scanner analysis
trivy fs examples/vulnerable-app/ --format json > analysis-trivy.json
grype examples/vulnerable-app/ -o json > analysis-grype.json

# Compare vulnerability findings
cat analysis-trivy.json | jq '.Results[].Vulnerabilities[].VulnerabilityID' | sort | uniq
cat analysis-grype.json | jq '.matches[].vulnerability.id' | sort | uniq
```

### Scenario 2: SBOM Generation and Validation

```bash
# Generate comprehensive SBOM
syft examples/vulnerable-app/ -o cyclonedx-json=comprehensive-sbom.json

# Validate SBOM structure
cat comprehensive-sbom.json | jq 'keys'
cat comprehensive-sbom.json | jq '.components | length'
cat comprehensive-sbom.json | jq '.components[].name'

# Check for Go dependencies
cat comprehensive-sbom.json | jq '.components[] | select(.type=="library") | .name'
```

### Scenario 3: CI/CD Pipeline Integration

```bash
# Examine GitHub Actions workflow
cat .github/workflows/security-pipeline.yaml

# Test workflow components locally
./scripts/setup/validate-scanners.sh
./scripts/setup/test-sbom-workflow.sh

# View workflow run history (if repository has actions)
gh run list --workflow=security-pipeline.yaml
```

### Scenario 4: Development Workflow Integration

```bash
# Pre-commit security check
./scripts/setup/test-vulnerable-app.sh

# Continuous monitoring setup
watch -n 30 './scripts/setup/validate-scanners.sh'

# Integration with development tools
go mod tidy examples/vulnerable-app/
trivy fs examples/vulnerable-app/ --exit-code 1 --severity HIGH,CRITICAL
```

## Environment Configuration

### GitHub Integration Variables

```bash
# Required for GitHub Actions integration
export GITHUB_TOKEN="your-personal-access-token"

# Optional: Repository configuration
export GITHUB_REPOSITORY="salman-frs/keystone"
export GITHUB_REF="refs/heads/main"

# Verify environment
echo "Token: ${GITHUB_TOKEN:0:10}..."
gh auth status
```

### Scanner Configuration Files

Create custom configuration files for enhanced scanning:

```bash
# Trivy configuration (optional)
cat > .trivyignore << EOF
# Ignore specific CVEs (example)
CVE-2023-12345
EOF

# Grype configuration (optional)
cat > .grype.yaml << EOF
# Grype configuration
output: json
fail-on-severity: high
EOF
```

### Directory Structure Configuration

```bash
# Ensure output directories exist
mkdir -p scanner-test-output
mkdir -p sbom-test-output

# Set proper permissions
chmod 755 scanner-test-output sbom-test-output
chmod +x scripts/setup/*.sh
```

## Troubleshooting

### Scanner Issues

**Trivy database update failures:**
```bash
# Clear Trivy cache and update
rm -rf ~/.cache/trivy
trivy image --download-db-only

# Check database status
trivy --version
```

**Grype database issues:**
```bash
# Update Grype database
grype db update

# Check database status
grype db status

# Reset database if corrupted
rm -rf ~/.grype
grype db update
```

**Syft SBOM generation failures:**
```bash
# Verify target directory exists
ls -la examples/vulnerable-app/

# Test with verbose output
syft examples/vulnerable-app/ -v

# Check supported formats
syft --help | grep -A 5 "output formats"
```

### GitHub Actions Integration Issues

**Workflow permission errors:**
```bash
# Check repository permissions
gh api repos/salman-frs/keystone --jq '.permissions'

# Verify GitHub token scopes
gh auth token | gh api graphql -f query='query { viewer { login } }'
```

**Local workflow testing issues:**
```bash
# Install act for local testing
brew install act

# Test workflow with act
act -l  # List available actions
act push -n  # Dry run
```

## Next Steps

1. **Run Demo Workflow**: Execute [Demo Guide](demo.md) for complete vulnerability detection demonstration
2. **Explore Architecture**: Review [Architecture Overview](architecture.md) with system diagrams
3. **Study Examples**: Examine vulnerable application in `examples/vulnerable-app/`
4. **Configure CI/CD**: Understand GitHub Actions workflow in `.github/workflows/security-pipeline.yaml`
5. **Review Validation Scripts**: Study testing scripts in `scripts/setup/`

## Verification Checklist

Confirm successful setup completion:

- [ ] Go 1.21+ installed and verified
- [ ] GitHub CLI authenticated and tested
- [ ] Colima running with Docker functionality
- [ ] Trivy, Grype, Syft, and Cosign installed
- [ ] Repository cloned and directory structure verified
- [ ] Vulnerable application builds successfully
- [ ] Scanner validation scripts execute without errors
- [ ] SBOM generation produces valid output
- [ ] GitHub Actions workflow structure reviewed

## Additional Resources

- **[Demo Guide](demo.md)**: Step-by-step vulnerability detection workflow
- **[Architecture Overview](architecture.md)**: System design and component relationships
- **[Troubleshooting Guide](troubleshooting.md)**: Common issues and solutions
- **[API Documentation](api/)**: Future API specifications
- **[Security Model](security/)**: Threat model and compliance framework