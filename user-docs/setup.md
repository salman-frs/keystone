# Setup Guide

This guide provides complete macOS setup instructions for the Keystone Security Platform local development environment.

## Prerequisites

### Required Software

- **Go 1.21+**: Backend services and vulnerable demo application
- **Node.js 18+**: Frontend development (when dashboard is implemented)
- **Docker with Colima**: Container runtime without Docker Desktop dependency
- **GitHub CLI**: Repository access and authentication
- **Security Tools**: Trivy, Grype, Syft, and Cosign for vulnerability scanning and SBOM generation

### macOS Setup with Colima

Colima provides Docker compatibility without Docker Desktop licensing requirements:

```bash
# Install Colima and Docker CLI
brew install colima docker

# Start Colima with specific configuration
colima start --cpu 4 --memory 8 --disk 60

# Verify Docker connectivity
docker --version
docker ps

# Test container functionality
docker run hello-world
```

### Security Tools Installation

Install required security scanning tools:

```bash
# Install Trivy vulnerability scanner
brew install trivy

# Install Grype vulnerability scanner
brew install anchore/grype/grype

# Install Syft SBOM generator
brew install anchore/syft/syft

# Install Cosign for cryptographic signing
brew install cosign

# Verify installations
trivy --version
grype version
syft version
cosign version
```

### GitHub CLI Setup

Configure GitHub CLI for repository access:

```bash
# Install GitHub CLI
brew install gh

# Authenticate with GitHub
gh auth login

# Verify authentication
gh auth status

# Test repository access
gh repo view salman-frs/keystone
```

### Verification Commands

```bash
# Check Go version (required: 1.21+)
go version

# Check Docker connectivity via Colima
docker ps
docker system info

# Check GitHub CLI authentication
gh auth status

# Verify security tools
trivy --version
grype version
syft version
cosign version
```

## Local Development Environment

### 1. Clone and Setup

```bash
git clone https://github.com/salman-frs/keystone.git
cd keystone

# Verify directory structure
ls -la
```

### 2. Environment Configuration

Keystone uses environment variables for configuration:

```bash
# Create environment configuration (if .env.example exists)
if [ -f .env.example ]; then
    cp .env.example .env.local
fi

# Set GitHub token for API access
export GITHUB_TOKEN="your-github-token"

# Verify environment
echo $GITHUB_TOKEN | cut -c1-10
```

### 3. Verify Project Structure

```bash
# Confirm directory structure matches expectations
ls -la
ls .github/workflows/
ls examples/
ls scripts/setup/
```

### 4. Test Security Workflow

Run the implemented security scanning workflow:

```bash
# Test vulnerability scanners
./scripts/setup/validate-scanners.sh

# Test SBOM generation
./scripts/setup/test-sbom-workflow.sh

# Test vulnerable application
./scripts/setup/test-vulnerable-app.sh

# Run complete workflow test
./scripts/setup/test-workflow.sh
```

### 5. Verify Installation Success

Confirm all components are working:

```bash
# Check vulnerable app builds
cd examples/vulnerable-app
go build -o vulnerable-app .
cd ../..

# Verify scanner outputs exist
ls -la scanner-test-output/
ls -la sbom-test-output/

# Test GitHub Actions workflow locally (if act is installed)
# brew install act
# act push
```

## Development Workflow

### Security Pipeline Development

```bash
# Modify security workflow
vim .github/workflows/security-pipeline.yaml

# Test workflow changes locally
cd examples/vulnerable-app
go mod tidy
go build .
cd ../..

# Run security scans
trivy fs examples/vulnerable-app/ --format json --output scanner-test-output/trivy-format-test.json
grype examples/vulnerable-app/ -o json > scanner-test-output/grype-format-test.json
```

### SBOM Generation Workflow

```bash
# Generate SBOM for vulnerable app
syft examples/vulnerable-app/ -o spdx-json=sbom-test-output/sbom-spdx.json
syft examples/vulnerable-app/ -o cyclonedx-json=sbom-test-output/sbom-cyclonedx.json

# Validate SBOM content
cat sbom-test-output/sbom-spdx.json | jq '.packages | length'
cat sbom-test-output/sbom-cyclonedx.json | jq '.components | length'
```

### Future Development (Planned)

```bash
# Backend API development (when implemented)
cd apps/api
go mod tidy
go test ./...
go build ./cmd/...

# Frontend dashboard development (when implemented)
cd apps/dashboard
npm install
npm run dev
npm test
```

## Data Storage

Keystone stores scan results and SBOM data locally:

```bash
# Scan output directory
ls -la scanner-test-output/

# SBOM output directory  
ls -la sbom-test-output/

# GitHub Actions artifacts (when workflows run)
# Stored in GitHub Container Registry: ghcr.io/salman-frs/keystone
```

## Troubleshooting

### Common macOS Setup Issues

**Colima startup failures:**
```bash
# Reset Colima completely
colima delete
colima start --cpu 4 --memory 8 --disk 60

# Check Colima status
colima status
colima list
```

**Docker connectivity issues:**
```bash
# Verify Docker socket
docker context ls
docker system info

# Restart Colima if needed
colima restart
```

**Security tool installation issues:**
```bash
# Update Homebrew and retry
brew update
brew upgrade

# Verify tool installations
which trivy grype syft cosign
```

**GitHub CLI authentication issues:**
```bash
# Re-authenticate with GitHub
gh auth logout
gh auth login

# Test repository access
gh repo view salman-frs/keystone
```

**Permission issues:**
```bash
# Fix script permissions
chmod +x scripts/setup/*.sh

# Fix output directory permissions
mkdir -p scanner-test-output sbom-test-output
chmod -R 755 scanner-test-output sbom-test-output
```

## Next Steps

Once setup is complete:

1. **Follow Getting Started**: Read [Getting Started Guide](getting-started.md)
2. **Run Demo Workflow**: Execute [Demo Guide](demo.md) for vulnerability detection demonstration
3. **Explore Architecture**: Review [Architecture Overview](architecture.md) with system diagrams
4. **Test Integration**: Examine GitHub Actions workflow in `.github/workflows/security-pipeline.yaml`
5. **Review Examples**: Study vulnerable application in `examples/vulnerable-app/`

## Development Tools

### Recommended VSCode Extensions

- **Go extension**: Go language support
- **YAML extension**: GitHub Actions workflow editing
- **Docker extension**: Container management
- **GitHub Actions extension**: Workflow syntax highlighting
- **JSON extension**: SBOM and scan result viewing

### Useful Development Commands

```bash
# Format Go code
go fmt ./examples/vulnerable-app/

# Validate GitHub Actions workflow
gh workflow list
gh workflow view security-pipeline

# Manual security scanning
./scripts/setup/validate-scanners.sh

# SBOM validation
./scripts/setup/test-sbom-workflow.sh

# Complete integration test
./scripts/setup/test-workflow.sh
```

### Performance Optimization

```bash
# Optimize Colima for security scanning
colima stop
colima start --cpu 6 --memory 12 --disk 100

# Cache scanner databases for faster scans
trivy image --download-db-only
grype db update
```