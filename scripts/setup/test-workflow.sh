#!/bin/bash
set -euo pipefail

# Test workflow validation script for GitHub Actions security pipeline
# This script validates the security pipeline workflow components locally

echo " Starting GitHub Actions workflow validation tests..."

# Check if required tools are available
check_dependencies() {
    echo " Checking dependencies..."

    local missing_deps=()

    command -v docker >/dev/null 2>&1 || missing_deps+=("docker")
    command -v jq >/dev/null 2>&1 || missing_deps+=("jq")
    command -v curl >/dev/null 2>&1 || missing_deps+=("curl")

    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        echo " Missing dependencies: ${missing_deps[*]}"
        echo "Please install missing dependencies and try again."
        exit 1
    fi

    echo " All dependencies available"
}

# Test vulnerable demo app container build
test_container_build() {
    echo "  Testing vulnerable demo container build..."

    if [[ ! -f "examples/vulnerable-app/Dockerfile" ]]; then
        echo " Vulnerable app Dockerfile not found"
        return 1
    fi

    if docker build -t test-vulnerable-demo:latest examples/vulnerable-app/; then
        echo " Container build successful"
        return 0
    else
        echo " Container build failed"
        return 1
    fi
}

# Test Trivy scanner functionality
test_trivy_scanner() {
    echo " Testing Trivy scanner..."

    # Create test output directory
    mkdir -p test-output

    # Test Trivy scan (using a small test image)
    if docker run --rm -v "$(pwd)/test-output":/output \
        aquasec/trivy:latest image \
        --format json \
        --output /output/test-trivy-results.json \
        alpine:3.17; then
        echo " Trivy scanner test passed"

        # Validate JSON output
        if jq empty test-output/test-trivy-results.json 2>/dev/null; then
            echo " Trivy JSON output is valid"
        else
            echo " Trivy JSON output is invalid"
            return 1
        fi

        return 0
    else
        echo " Trivy scanner test failed"
        return 1
    fi
}

# Test Grype scanner functionality
test_grype_scanner() {
    echo " Testing Grype scanner..."

    # Install Grype locally for testing
    if ! command -v grype >/dev/null 2>&1; then
        echo " Installing Grype for testing..."
        curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /tmp
        export PATH="/tmp:$PATH"
    fi

    # Create test output directory
    mkdir -p test-output

    # Test Grype scan
    if grype alpine:3.17 \
        -o json \
        --file test-output/test-grype-results.json; then
        echo " Grype scanner test passed"

        # Validate JSON output
        if jq empty test-output/test-grype-results.json 2>/dev/null; then
            echo " Grype JSON output is valid"
        else
            echo " Grype JSON output is invalid"
            return 1
        fi

        return 0
    else
        echo " Grype scanner test failed"
        return 1
    fi
}

# Test JSON parsing logic
test_json_parsing() {
    echo " Testing JSON parsing logic..."

    # Create mock Trivy results
    cat > test-output/mock-trivy-results.json << 'EOF'
{
  "Results": [
    {
      "Vulnerabilities": [
        {"Severity": "CRITICAL"},
        {"Severity": "HIGH"},
        {"Severity": "MEDIUM"},
        {"Severity": "LOW"}
      ]
    }
  ]
}
EOF

    # Create mock Grype results
    cat > test-output/mock-grype-results.json << 'EOF'
{
  "matches": [
    {"vulnerability": {"severity": "Critical"}},
    {"vulnerability": {"severity": "High"}},
    {"vulnerability": {"severity": "Medium"}},
    {"vulnerability": {"severity": "Low"}}
  ]
}
EOF

    # Test Trivy parsing
    trivy_critical=$(jq '[.Results[]?.Vulnerabilities[]? | select(.Severity == "CRITICAL")] | length' test-output/mock-trivy-results.json)
    trivy_high=$(jq '[.Results[]?.Vulnerabilities[]? | select(.Severity == "HIGH")] | length' test-output/mock-trivy-results.json)

    # Test Grype parsing
    grype_critical=$(jq '[.matches[]? | select(.vulnerability.severity == "Critical")] | length' test-output/mock-grype-results.json)
    grype_high=$(jq '[.matches[]? | select(.vulnerability.severity == "High")] | length' test-output/mock-grype-results.json)

    if [[ "$trivy_critical" == "1" && "$trivy_high" == "1" && "$grype_critical" == "1" && "$grype_high" == "1" ]]; then
        echo " JSON parsing logic works correctly"
        return 0
    else
        echo " JSON parsing logic failed"
        echo "  Trivy critical: $trivy_critical (expected: 1)"
        echo "  Trivy high: $trivy_high (expected: 1)"
        echo "  Grype critical: $grype_critical (expected: 1)"
        echo "  Grype high: $grype_high (expected: 1)"
        return 1
    fi
}

# Test workflow artifact simulation
test_artifact_handling() {
    echo " Testing artifact handling simulation..."

    mkdir -p test-output/artifacts

    # Simulate artifact creation
    if [[ -f test-output/test-trivy-results.json ]]; then
        cp test-output/test-trivy-results.json test-output/artifacts/trivy-vulnerability-results.json
        echo " Trivy artifact simulation successful"
    fi

    if [[ -f test-output/test-grype-results.json ]]; then
        cp test-output/test-grype-results.json test-output/artifacts/grype-vulnerability-results.json
        echo " Grype artifact simulation successful"
    fi

    # Validate artifacts exist
    if [[ -f test-output/artifacts/trivy-vulnerability-results.json && -f test-output/artifacts/grype-vulnerability-results.json ]]; then
        echo " Artifact handling test passed"
        return 0
    else
        echo " Artifact handling test failed"
        return 1
    fi
}

# Main test execution
main() {
    local exit_code=0

    echo " Running workflow validation tests..."
    echo "Working directory: $(pwd)"

    # Run all tests
    check_dependencies || exit_code=1
    test_container_build || exit_code=1
    test_trivy_scanner || exit_code=1
    test_grype_scanner || exit_code=1
    test_json_parsing || exit_code=1
    test_artifact_handling || exit_code=1

    # Cleanup
    echo " Cleaning up test artifacts..."
    rm -rf test-output
    docker rmi test-vulnerable-demo:latest 2>/dev/null || true

    if [[ $exit_code -eq 0 ]]; then
        echo " All workflow validation tests passed!"
    else
        echo " Some tests failed. Please check the output above."
    fi

    exit $exit_code
}

# Run main function
main "$@"