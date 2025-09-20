# Demo Guide: Vulnerability Detection Workflow

This guide provides a step-by-step demonstration of Keystone's vulnerability detection and SBOM generation capabilities for technical evaluation.

## Overview

The demo showcases the complete security pipeline implemented in Keystone:

1. **Vulnerable Application Scanning**: Using the intentionally vulnerable Go application
2. **Multi-Scanner Integration**: Trivy and Grype vulnerability detection
3. **SBOM Generation**: Software bill of materials creation with Syft
4. **GitHub Actions Workflow**: Automated security pipeline demonstration
5. **Result Analysis**: Comparing and correlating security findings

## Prerequisites

Ensure you have completed the [Setup Guide](setup.md) and [Getting Started](getting-started.md) steps:

- Go 1.21+ installed
- GitHub CLI authenticated
- Colima running with Docker functionality
- Trivy, Grype, Syft, and Cosign installed
- Repository cloned and directory structure verified

## Demo Preparation

### 1. Verify Environment

```bash
# Confirm you're in the Keystone project root
pwd  # Should show /path/to/keystone

# Verify all tools are available
./scripts/setup/validate-scanners.sh

# Create output directories
mkdir -p demo-output
mkdir -p demo-sbom
```

### 2. Examine the Vulnerable Application

```bash
# Navigate to the vulnerable demo application
cd examples/vulnerable-app

# Review the application code
cat main.go

# Check dependencies that contain vulnerabilities
cat go.mod
cat go.sum

# Return to project root
cd ../..
```

## Step-by-Step Demonstration

### Step 1: Build the Vulnerable Application

```bash
# Build the Go application
cd examples/vulnerable-app
go build -o vulnerable-app .

# Verify the build
ls -la vulnerable-app
file vulnerable-app

# Test the application (optional)
./vulnerable-app &
APP_PID=$!

# Test health endpoint
curl http://localhost:8080/health

# Test version endpoint
curl http://localhost:8080/version

# Stop the application
kill $APP_PID
cd ../..
```

### Step 2: Vulnerability Scanning with Trivy

```bash
# Scan with Trivy for filesystem vulnerabilities
echo "=== Running Trivy Vulnerability Scan ==="
trivy fs examples/vulnerable-app/ \
    --format json \
    --output demo-output/trivy-demo.json

# Generate human-readable Trivy report
trivy fs examples/vulnerable-app/ \
    --format table \
    --output demo-output/trivy-demo.txt

# View summary of findings
echo "Trivy Scan Results:"
cat demo-output/trivy-demo.json | jq '.Results[].Vulnerabilities | length'

# Show critical and high severity vulnerabilities
echo "Critical and High Severity Vulnerabilities:"
cat demo-output/trivy-demo.json | jq '.Results[].Vulnerabilities[] | select(.Severity == "CRITICAL" or .Severity == "HIGH") | {ID: .VulnerabilityID, Severity: .Severity, Package: .PkgName}'
```

### Step 3: Vulnerability Scanning with Grype

```bash
# Scan with Grype for package vulnerabilities
echo "=== Running Grype Vulnerability Scan ==="
grype examples/vulnerable-app/ \
    -o json > demo-output/grype-demo.json

# Generate human-readable Grype report
grype examples/vulnerable-app/ \
    -o table > demo-output/grype-demo.txt

# View summary of findings
echo "Grype Scan Results:"
cat demo-output/grype-demo.json | jq '.matches | length'

# Show high severity vulnerabilities
echo "High and Critical Severity Vulnerabilities:"
cat demo-output/grype-demo.json | jq '.matches[] | select(.vulnerability.severity == "High" or .vulnerability.severity == "Critical") | {ID: .vulnerability.id, Severity: .vulnerability.severity, Package: .artifact.name}'
```

### Step 4: SBOM Generation with Syft

```bash
# Generate SPDX format SBOM
echo "=== Generating SPDX SBOM ==="
syft examples/vulnerable-app/ \
    -o spdx-json=demo-sbom/demo-spdx.json

# Generate CycloneDX format SBOM
echo "=== Generating CycloneDX SBOM ==="
syft examples/vulnerable-app/ \
    -o cyclonedx-json=demo-sbom/demo-cyclonedx.json

# View SBOM summary
echo "SPDX SBOM Package Count:"
cat demo-sbom/demo-spdx.json | jq '.packages | length'

echo "CycloneDX SBOM Component Count:"
cat demo-sbom/demo-cyclonedx.json | jq '.components | length'

# Show Go module dependencies
echo "Go Module Dependencies:"
cat demo-sbom/demo-spdx.json | jq '.packages[] | select(.name | contains("github.com")) | .name'
```

### Step 5: Multi-Scanner Correlation Analysis

```bash
# Extract vulnerability IDs from both scanners
echo "=== Vulnerability Correlation Analysis ==="

# Get Trivy CVE IDs
cat demo-output/trivy-demo.json | jq -r '.Results[].Vulnerabilities[].VulnerabilityID' | sort | uniq > demo-output/trivy-cves.txt

# Get Grype CVE IDs
cat demo-output/grype-demo.json | jq -r '.matches[].vulnerability.id' | sort | uniq > demo-output/grype-cves.txt

# Find common CVEs
echo "Common CVEs found by both scanners:"
comm -12 demo-output/trivy-cves.txt demo-output/grype-cves.txt

# Find unique CVEs per scanner
echo "CVEs found only by Trivy:"
comm -23 demo-output/trivy-cves.txt demo-output/grype-cves.txt | head -5

echo "CVEs found only by Grype:"
comm -13 demo-output/trivy-cves.txt demo-output/grype-cves.txt | head -5

# Count total unique CVEs across both scanners
echo "Total unique CVEs across both scanners:"
cat demo-output/trivy-cves.txt demo-output/grype-cves.txt | sort | uniq | wc -l
```

### Step 6: GitHub Actions Workflow Demonstration

```bash
# Examine the implemented GitHub Actions workflow
echo "=== GitHub Actions Workflow Analysis ==="
cat .github/workflows/security-pipeline.yaml

# Show workflow structure
echo "Workflow Jobs:"
cat .github/workflows/security-pipeline.yaml | grep -E "^  [a-zA-Z-]+:" | sed 's/://'

# Test workflow locally using act (if installed)
if command -v act &> /dev/null; then
    echo "Testing workflow locally with act:"
    act push --dry-run
else
    echo "Install 'act' to test workflows locally: brew install act"
fi

# Check if workflow has run in the repository
gh run list --workflow=security-pipeline.yaml --limit 5 2>/dev/null || echo "No workflow runs found (repository may not have GitHub Actions enabled)"
```

### Step 7: Results Summary and Analysis

```bash
# Generate comprehensive demo summary
echo "=== Demo Results Summary ===" > demo-output/demo-summary.txt
echo "Generated on: $(date)" >> demo-output/demo-summary.txt
echo "" >> demo-output/demo-summary.txt

echo "Trivy Scan Results:" >> demo-output/demo-summary.txt
echo "  Total Vulnerabilities: $(cat demo-output/trivy-demo.json | jq '.Results[].Vulnerabilities | length')" >> demo-output/demo-summary.txt
echo "  Critical: $(cat demo-output/trivy-demo.json | jq '.Results[].Vulnerabilities[] | select(.Severity == "CRITICAL") | .VulnerabilityID' | wc -l)" >> demo-output/demo-summary.txt
echo "  High: $(cat demo-output/trivy-demo.json | jq '.Results[].Vulnerabilities[] | select(.Severity == "HIGH") | .VulnerabilityID' | wc -l)" >> demo-output/demo-summary.txt
echo "" >> demo-output/demo-summary.txt

echo "Grype Scan Results:" >> demo-output/demo-summary.txt
echo "  Total Vulnerabilities: $(cat demo-output/grype-demo.json | jq '.matches | length')" >> demo-output/demo-summary.txt
echo "  Critical: $(cat demo-output/grype-demo.json | jq '.matches[] | select(.vulnerability.severity == "Critical") | .vulnerability.id' | wc -l)" >> demo-output/demo-summary.txt
echo "  High: $(cat demo-output/grype-demo.json | jq '.matches[] | select(.vulnerability.severity == "High") | .vulnerability.id' | wc -l)" >> demo-output/demo-summary.txt
echo "" >> demo-output/demo-summary.txt

echo "SBOM Generation Results:" >> demo-output/demo-summary.txt
echo "  SPDX Packages: $(cat demo-sbom/demo-spdx.json | jq '.packages | length')" >> demo-output/demo-summary.txt
echo "  CycloneDX Components: $(cat demo-sbom/demo-cyclonedx.json | jq '.components | length')" >> demo-output/demo-summary.txt
echo "" >> demo-output/demo-summary.txt

# Display the summary
cat demo-output/demo-summary.txt
```

## Advanced Demonstration Features

### Container Scanning (Optional)

```bash
# Build container image for scanning
cd examples/vulnerable-app
docker build -t keystone-demo:latest .
cd ../..

# Scan container image with Trivy
trivy image keystone-demo:latest \
    --format json \
    --output demo-output/trivy-container.json

# Compare filesystem vs container scan results
echo "Filesystem scan vulnerabilities: $(cat demo-output/trivy-demo.json | jq '.Results[].Vulnerabilities | length')"
echo "Container scan vulnerabilities: $(cat demo-output/trivy-container.json | jq '.Results[].Vulnerabilities | length')"
```

### SBOM Validation and Analysis

```bash
# Validate SBOM structure and content
echo "=== SBOM Validation ==="

# Check SPDX SBOM structure
echo "SPDX SBOM validation:"
cat demo-sbom/demo-spdx.json | jq 'has("spdxVersion") and has("packages") and has("documentName")'

# Check CycloneDX SBOM structure
echo "CycloneDX SBOM validation:"
cat demo-sbom/demo-cyclonedx.json | jq 'has("bomFormat") and has("components") and has("metadata")'

# Extract package licenses
echo "Package licenses found:"
cat demo-sbom/demo-spdx.json | jq '.packages[].licenseConcluded' | sort | uniq -c | sort -nr
```

### Security Metrics Analysis

```bash
# Generate security metrics
echo "=== Security Metrics Analysis ==="

# Calculate severity distribution for Trivy
echo "Trivy Severity Distribution:"
cat demo-output/trivy-demo.json | jq '.Results[].Vulnerabilities[].Severity' | sort | uniq -c

# Calculate severity distribution for Grype
echo "Grype Severity Distribution:"
cat demo-output/grype-demo.json | jq '.matches[].vulnerability.severity' | sort | uniq -c

# Identify most vulnerable packages
echo "Most vulnerable packages (Trivy):"
cat demo-output/trivy-demo.json | jq '.Results[].Vulnerabilities[].PkgName' | sort | uniq -c | sort -nr | head -5

echo "Most vulnerable packages (Grype):"
cat demo-output/grype-demo.json | jq '.matches[].artifact.name' | sort | uniq -c | sort -nr | head -5
```

## Evaluation Checklist for Technical Interviewers

Use this checklist to evaluate the demo results:

### Security Scanning Capabilities
- [ ] Trivy scanner successfully detects vulnerabilities in Go dependencies
- [ ] Grype scanner provides complementary vulnerability detection
- [ ] Multiple output formats (JSON, table) are generated correctly
- [ ] Vulnerability correlation identifies common and unique findings

### SBOM Generation
- [ ] SPDX format SBOM contains comprehensive package information
- [ ] CycloneDX format SBOM includes component metadata
- [ ] Package licenses are properly identified and listed
- [ ] Dependency tree structure is accurately represented

### Automation and Integration
- [ ] GitHub Actions workflow configuration is production-ready
- [ ] Local testing scripts execute without errors
- [ ] Output files are properly structured and machine-readable
- [ ] Container scanning capabilities are demonstrated (optional)

### Code Quality and Documentation
- [ ] Vulnerable application demonstrates realistic security issues
- [ ] Demo scripts are well-documented and easy to follow
- [ ] Error handling and edge cases are considered
- [ ] Results are clearly summarized and analyzed

## Expected Demo Results

When the demo runs successfully, you should see:

1. **Vulnerable Application**: Builds and runs with intentional security vulnerabilities
2. **Trivy Scan**: Detects 15-25 vulnerabilities including CRITICAL and HIGH severity issues
3. **Grype Scan**: Identifies similar vulnerabilities with some overlap and differences
4. **SBOM Generation**: Produces comprehensive software bill of materials in both formats
5. **Correlation Analysis**: Shows common vulnerabilities and scanner-specific findings
6. **GitHub Actions**: Demonstrates automated security pipeline configuration

## Troubleshooting Demo Issues

### Scanner Database Issues
```bash
# Update scanner databases if scans fail
trivy image --download-db-only
grype db update

# Verify database status
trivy --version
grype db status
```

### Output Directory Issues
```bash
# Ensure output directories exist and have proper permissions
mkdir -p demo-output demo-sbom
chmod 755 demo-output demo-sbom
```

### Go Build Issues
```bash
# Clean and rebuild if Go build fails
cd examples/vulnerable-app
go clean
go mod tidy
go build -o vulnerable-app .
cd ../..
```

## Next Steps After Demo

1. **Review Architecture**: Study [Architecture Overview](architecture.md) for system design details
2. **Examine Workflows**: Analyze GitHub Actions configuration in `.github/workflows/`
3. **Explore Validation Scripts**: Review testing scripts in `scripts/setup/`
4. **Study Vulnerability Data**: Analyze JSON outputs for integration patterns
5. **Consider Extensions**: Plan additional security tools and policy enforcement

## Demo Cleanup

```bash
# Clean up demo files (optional)
rm -rf demo-output demo-sbom

# Stop any running containers
docker ps -q --filter ancestor=keystone-demo:latest | xargs -r docker stop

# Remove demo container image (optional)
docker rmi keystone-demo:latest 2>/dev/null || true
```

This demo showcases Keystone's current implementation status and provides a foundation for evaluating the security automation capabilities in a technical interview context.