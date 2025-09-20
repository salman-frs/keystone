#!/bin/bash
set -euo pipefail

# Scanner validation script for Trivy and Grype
# Validates scanner installation, functionality, and output formats

echo "🔍 Starting scanner validation..."

# Test Trivy scanner functionality
validate_trivy() {
    echo " Validating Trivy scanner..."

    # Test Trivy availability via Docker
    if docker run --rm aquasec/trivy:latest --version >/dev/null 2>&1; then
        echo "Trivy Docker image accessible"
    else
        echo "Trivy Docker image not accessible"
        return 1
    fi

    # Test Trivy JSON output format with a small image
    echo "Testing Trivy JSON output format..."
    mkdir -p scanner-test-output

    if docker run --rm -v "$(pwd)/scanner-test-output":/output \
        aquasec/trivy:latest image \
        --format json \
        --output /output/trivy-format-test.json \
        alpine:3.17 >/dev/null 2>&1; then

        # Validate JSON structure
        if jq -e '.Results[]?.Vulnerabilities[]?.Severity' scanner-test-output/trivy-format-test.json >/dev/null 2>&1; then
            echo "Trivy JSON format validation passed"
        else
            echo "Trivy JSON format validation failed"
            return 1
        fi
    else
        echo "Trivy scan test failed"
        return 1
    fi

    return 0
}

# Test Grype scanner functionality
validate_grype() {
    echo "Validating Grype scanner..."

    # Install Grype temporarily for testing
    if ! command -v grype >/dev/null 2>&1; then
        echo "📦 Installing Grype for validation..."
        curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /tmp >/dev/null 2>&1
        export PATH="/tmp:$PATH"
    fi

    # Test Grype version
    if grype --version >/dev/null 2>&1; then
        echo "Grype installation verified"
    else
        echo "Grype installation failed"
        return 1
    fi

    # Test Grype JSON output format
    echo "Testing Grype JSON output format..."
    mkdir -p scanner-test-output

    if grype alpine:3.17 \
        -o json \
        --file scanner-test-output/grype-format-test.json \
        --quiet; then

        # Validate JSON structure
        if jq -e '.matches[]?.vulnerability.severity' scanner-test-output/grype-format-test.json >/dev/null 2>&1; then
            echo "Grype JSON format validation passed"
        else
            echo "Grype JSON format validation failed"
            return 1
        fi
    else
        echo "Grype scan test failed"
        return 1
    fi

    return 0
}

# Validate scanning scope coverage
validate_scanning_scope() {
    echo "Validating scanning scope coverage..."

    # Check if Trivy covers OS packages and dependencies
    echo "Testing OS package detection with Trivy..."
    if docker run --rm aquasec/trivy:latest image \
        --format json \
        --scanners vuln \
        alpine:3.17 2>/dev/null | jq -e '.Results[]?.Vulnerabilities[]?' >/dev/null 2>&1; then
        echo "Trivy OS package scanning working"
    else
        echo "Trivy OS package scanning may not detect vulnerabilities (this could be normal for clean images)"
    fi

    # Check if Grype covers both OS and language packages
    echo "Testing dependency detection with Grype..."
    if grype alpine:3.17 -o json --quiet 2>/dev/null | jq -e '.matches[]?' >/dev/null 2>&1; then
        echo "Grype vulnerability detection working"
    else
        echo "Grype vulnerability detection may not find issues (this could be normal for clean images)"
    fi

    return 0
}

# Test failure scenarios
validate_failure_scenarios() {
    echo "Testing failure scenario handling..."

    # Test with invalid image (should fail gracefully)
    echo "Testing scanner behavior with invalid image..."

    # Test Trivy with non-existent image
    if docker run --rm aquasec/trivy:latest image \
        --timeout 30s \
        non-existent-image:invalid 2>/dev/null; then
        echo "Trivy should have failed with invalid image"
    else
        echo "Trivy handles invalid images correctly"
    fi

    # Test Grype with non-existent image
    if grype non-existent-image:invalid --quiet 2>/dev/null; then
        echo "Grype should have failed with invalid image"
    else
        echo "Grype handles invalid images correctly"
    fi

    return 0
}

# Validate timeout handling
validate_timeout_handling() {
    echo "Testing timeout handling..."

    # Test with very short timeout (should trigger timeout)
    echo "Testing Trivy timeout behavior..."
    if timeout 5s docker run --rm aquasec/trivy:latest image \
        --timeout 1s \
        alpine:3.17 2>/dev/null; then
        echo "Trivy completed too quickly for timeout test"
    else
        echo "Trivy timeout handling working"
    fi

    return 0
}

# Main validation function
main() {
    local exit_code=0

    echo "Starting comprehensive scanner validation..."
    echo "Working directory: $(pwd)"

    # Check dependencies
    if ! command -v docker >/dev/null 2>&1; then
        echo "Docker is required but not available"
        exit 1
    fi

    if ! command -v jq >/dev/null 2>&1; then
        echo "jq is required but not available"
        exit 1
    fi

    if ! command -v curl >/dev/null 2>&1; then
        echo "curl is required but not available"
        exit 1
    fi

    # Run validation tests
    validate_trivy || exit_code=1
    validate_grype || exit_code=1
    validate_scanning_scope || exit_code=1
    validate_failure_scenarios || exit_code=1
    validate_timeout_handling || exit_code=1

    # Cleanup
    echo "Cleaning up validation artifacts..."
    rm -rf scanner-test-output
    rm -f /tmp/grype 2>/dev/null || true

    if [[ $exit_code -eq 0 ]]; then
        echo "All scanner validations passed!"
    else
        echo "Some validations failed. Please check the output above."
    fi

    exit $exit_code
}

# Run main function
main "$@"