# Examples and Tutorials

This directory contains usage examples and tutorials for the Keystone Security Platform.

## Current Examples

### Vulnerable Application Demo

The primary example is the intentionally vulnerable Go application located in `../../examples/vulnerable-app/`.

**Purpose**: Demonstrates realistic security scanning scenarios with:
- Known CVE vulnerabilities in Go dependencies
- Documented security issues for testing workflows
- Build and deployment examples
- SBOM generation testing

**Usage**:
```bash
cd examples/vulnerable-app
go build -o vulnerable-app .
./vulnerable-app &
curl http://localhost:8080/health
```

### Security Scanning Examples

**Trivy Scanning Examples**:
```bash
# Basic filesystem scan
trivy fs examples/vulnerable-app/

# JSON output for automation
trivy fs examples/vulnerable-app/ --format json --output scan-results.json

# Specific severity filtering
trivy fs examples/vulnerable-app/ --severity HIGH,CRITICAL
```

**Grype Scanning Examples**:
```bash
# Package vulnerability scanning
grype examples/vulnerable-app/

# JSON output with detailed information
grype examples/vulnerable-app/ -o json > grype-results.json

# Table format for human review
grype examples/vulnerable-app/ -o table
```

**SBOM Generation Examples**:
```bash
# SPDX format SBOM
syft examples/vulnerable-app/ -o spdx-json=app-sbom.spdx.json

# CycloneDX format SBOM
syft examples/vulnerable-app/ -o cyclonedx-json=app-sbom.cyclonedx.json

# Multiple formats simultaneously
syft examples/vulnerable-app/ -o spdx-json,cyclonedx-json
```

## Workflow Examples

### GitHub Actions Integration

**Basic Security Workflow**:
```yaml
name: Security Scan
on: [push, pull_request]
jobs:
  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run Trivy
        run: trivy fs .
      - name: Generate SBOM
        run: syft . -o spdx-json=sbom.json
```

**Advanced Multi-Scanner Workflow**:
```yaml
name: Advanced Security Pipeline
on: [push, pull_request, release]
jobs:
  vulnerability-scan:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        scanner: [trivy, grype]
    steps:
      - uses: actions/checkout@v4
      - name: Run Security Scan
        run: |
          if [ "${{ matrix.scanner }}" = "trivy" ]; then
            trivy fs . --format json --output ${{ matrix.scanner }}-results.json
          else
            grype . -o json > ${{ matrix.scanner }}-results.json
          fi
      - name: Upload Results
        uses: actions/upload-artifact@v4
        with:
          name: ${{ matrix.scanner }}-results
          path: ${{ matrix.scanner }}-results.json
```

## Tutorial Examples

### Tutorial 1: Basic Security Scanning

**Objective**: Learn basic vulnerability scanning with Keystone tools.

**Steps**:
1. Clone and build the vulnerable application
2. Run Trivy scan and analyze results
3. Run Grype scan and compare findings
4. Generate SBOM and examine contents

**Expected Learning**:
- Understanding vulnerability scanner outputs
- Comparing different scanner results
- Basic SBOM structure and content

### Tutorial 2: Workflow Integration

**Objective**: Integrate security scanning into development workflow.

**Steps**:
1. Create GitHub Actions workflow
2. Configure automated scanning triggers
3. Set up artifact storage
4. Implement basic policy enforcement

**Expected Learning**:
- GitHub Actions workflow configuration
- Automated security pipeline setup
- Artifact management and storage

### Tutorial 3: Vulnerability Analysis

**Objective**: Analyze and correlate vulnerability findings.

**Steps**:
1. Execute multi-scanner analysis
2. Extract and compare CVE findings
3. Analyze severity distributions
4. Identify remediation priorities

**Expected Learning**:
- Vulnerability correlation techniques
- Risk assessment and prioritization
- Remediation planning strategies

## Code Examples

### Scanner Integration Examples

**Python Integration Example**:
```python
import json
import subprocess

def run_trivy_scan(target_path):
    """Run Trivy scan and return JSON results."""
    result = subprocess.run([
        'trivy', 'fs', target_path,
        '--format', 'json'
    ], capture_output=True, text=True)
    
    if result.returncode == 0:
        return json.loads(result.stdout)
    else:
        raise Exception(f"Trivy scan failed: {result.stderr}")

def extract_vulnerabilities(scan_results):
    """Extract vulnerability information from scan results."""
    vulnerabilities = []
    for result in scan_results.get('Results', []):
        for vuln in result.get('Vulnerabilities', []):
            vulnerabilities.append({
                'id': vuln.get('VulnerabilityID'),
                'severity': vuln.get('Severity'),
                'package': vuln.get('PkgName'),
                'version': vuln.get('InstalledVersion')
            })
    return vulnerabilities
```

**Go Integration Example**:
```go
package main

import (
    "encoding/json"
    "fmt"
    "os/exec"
)

type TrivyResult struct {
    Results []struct {
        Vulnerabilities []struct {
            VulnerabilityID string `json:"VulnerabilityID"`
            Severity       string `json:"Severity"`
            PkgName        string `json:"PkgName"`
        } `json:"Vulnerabilities"`
    } `json:"Results"`
}

func runTrivyScan(targetPath string) (*TrivyResult, error) {
    cmd := exec.Command("trivy", "fs", targetPath, "--format", "json")
    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }
    
    var result TrivyResult
    err = json.Unmarshal(output, &result)
    return &result, err
}
```

### SBOM Processing Examples

**SBOM Validation Script**:
```bash
#!/bin/bash
# validate-sbom.sh

SBOM_FILE="$1"

if [ ! -f "$SBOM_FILE" ]; then
    echo "Error: SBOM file not found: $SBOM_FILE"
    exit 1
fi

# Validate JSON structure
if ! jq empty "$SBOM_FILE" 2>/dev/null; then
    echo "Error: Invalid JSON in SBOM file"
    exit 1
fi

# Check SPDX format
if jq -e '.spdxVersion' "$SBOM_FILE" >/dev/null 2>&1; then
    echo "SPDX SBOM detected"
    PACKAGE_COUNT=$(jq '.packages | length' "$SBOM_FILE")
    echo "Package count: $PACKAGE_COUNT"
fi

# Check CycloneDX format
if jq -e '.bomFormat' "$SBOM_FILE" >/dev/null 2>&1; then
    echo "CycloneDX SBOM detected"
    COMPONENT_COUNT=$(jq '.components | length' "$SBOM_FILE")
    echo "Component count: $COMPONENT_COUNT"
fi
```

## Planned Examples

### Future Tutorial Content

When the full platform is implemented, this directory will contain:

- `basic-security-scan.md`: Step-by-step vulnerability scanning tutorial
- `policy-configuration.md`: Security policy setup and management
- `vulnerability-remediation.md`: Vulnerability remediation workflows
- `dashboard-usage.md`: Security dashboard navigation and usage
- `api-integration.md`: API integration examples and patterns

### Advanced Integration Examples

- **CI/CD Integration**: Examples for various CI/CD platforms
- **Policy as Code**: OPA/Rego policy examples
- **Cryptographic Signing**: Sigstore integration examples
- **Dashboard Customization**: React component examples
- **API Usage**: REST and GraphQL API examples

## Example Data

### Sample Vulnerability Data

The vulnerable application provides realistic examples of:
- **CVE-2023-45142**: Example high-severity vulnerability
- **CVE-2023-39326**: Example medium-severity vulnerability
- **Dependency vulnerabilities**: Realistic Go module vulnerabilities

### Sample SBOM Data

Generated SBOMs include examples of:
- Go module dependencies
- Standard library components
- License information
- Package relationships

## Contributing Examples

To contribute new examples:

1. **Create Example**: Develop working example with documentation
2. **Test Thoroughly**: Ensure example works in clean environment
3. **Document Clearly**: Provide step-by-step instructions
4. **Submit PR**: Follow contribution guidelines

For current examples and tutorials, see:
- [Demo Guide](../demo.md)
- [Getting Started](../getting-started.md)
- [Vulnerable Application](../../examples/vulnerable-app/README.md)