#!/bin/bash

# Test script for vulnerable demo application
# Validates application startup, health check, and scanner integration

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
VULN_APP_DIR="$PROJECT_ROOT/examples/vulnerable-app"

echo "Testing Vulnerable Demo Application"
echo "===================================="
echo "Project root: $PROJECT_ROOT"
echo "Vulnerable app directory: $VULN_APP_DIR"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

# Test results tracking
TEST_COUNT=0
PASS_COUNT=0
FAIL_COUNT=0

log_test() {
    TEST_COUNT=$((TEST_COUNT + 1))
    echo -e "${YELLOW}[TEST $TEST_COUNT]${NC} $1"
}

log_pass() {
    PASS_COUNT=$((PASS_COUNT + 1))
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_fail() {
    FAIL_COUNT=$((FAIL_COUNT + 1))
    echo -e "${RED}[FAIL]${NC} $1"
}

log_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

# Check prerequisites
log_test "Checking prerequisites"
command -v docker >/dev/null 2>&1 || { log_fail "Docker is required but not installed"; exit 1; }
command -v curl >/dev/null 2>&1 || { log_fail "curl is required but not installed"; exit 1; }
command -v jq >/dev/null 2>&1 || { log_fail "jq is required but not installed"; exit 1; }
log_pass "All prerequisites available"

# Test 1: Verify application files exist
log_test "Verifying application files"
if [[ -f "$VULN_APP_DIR/main.go" && -f "$VULN_APP_DIR/go.mod" && -f "$VULN_APP_DIR/Dockerfile" ]]; then
    log_pass "All required application files present"
else
    log_fail "Missing required application files"
    exit 1
fi

# Test 2: Go module validation
log_test "Validating Go module dependencies"
cd "$VULN_APP_DIR"
if go mod verify; then
    log_pass "Go module dependencies verified"
else
    log_fail "Go module verification failed"
fi

# Test 3: Run Go unit tests
log_test "Running Go unit tests"
if go test -v; then
    log_pass "Go unit tests passed"
else
    log_fail "Go unit tests failed"
fi

# Test 4: Build Docker container
log_test "Building Docker container"
if docker build -t vulnerable-demo-test:latest . >/dev/null 2>&1; then
    log_pass "Docker container built successfully"
else
    log_fail "Docker container build failed"
    exit 1
fi

# Test 5: Start container and test health check
log_test "Starting container and testing health check"
docker run -d --name vulnerable-demo-test -p 8080:8080 vulnerable-demo-test:latest >/dev/null

# Wait for container to start
log_info "Waiting for container to start..."
sleep 5

# Check if container is running
if docker ps | grep vulnerable-demo-test >/dev/null; then
    log_pass "Container started successfully"
else
    log_fail "Container failed to start"
    docker logs vulnerable-demo-test
    docker rm -f vulnerable-demo-test >/dev/null 2>&1
    exit 1
fi

# Test health endpoint
log_test "Testing health endpoint"
START_TIME=$(date +%s%3N)  # Get milliseconds directly
HEALTH_RESPONSE=$(curl -s http://localhost:8080/health || echo "")
END_TIME=$(date +%s%3N)
RESPONSE_TIME=$(( END_TIME - START_TIME )) # Already in milliseconds

if [[ -n "$HEALTH_RESPONSE" ]]; then
    log_pass "Health endpoint responded (${RESPONSE_TIME}ms)"

    # Validate response time (should be < 200ms)
    if [[ $RESPONSE_TIME -lt 200 ]]; then
        log_pass "Health check response time within requirements (${RESPONSE_TIME}ms < 200ms)"
    else
        log_fail "Health check response time too slow (${RESPONSE_TIME}ms >= 200ms)"
    fi

    # Validate JSON structure
    if echo "$HEALTH_RESPONSE" | jq -e '.status == "healthy"' >/dev/null 2>&1; then
        log_pass "Health endpoint returned valid JSON with healthy status"
    else
        log_fail "Health endpoint returned invalid JSON or non-healthy status"
        echo "Response: $HEALTH_RESPONSE"
    fi
else
    log_fail "Health endpoint did not respond"
fi

# Test version endpoint
log_test "Testing version endpoint"
VERSION_RESPONSE=$(curl -s http://localhost:8080/version || echo "")
if [[ -n "$VERSION_RESPONSE" ]]; then
    log_pass "Version endpoint responded"

    # Validate dependencies are listed
    if echo "$VERSION_RESPONSE" | jq -e '.dependencies | has("gin-gonic/gin")' >/dev/null 2>&1; then
        log_pass "Version endpoint lists vulnerable dependencies"
    else
        log_fail "Version endpoint missing dependency information"
    fi
else
    log_fail "Version endpoint did not respond"
fi

# Test ping endpoint (legacy)
log_test "Testing ping endpoint"
PING_RESPONSE=$(curl -s http://localhost:8080/ping || echo "")
if echo "$PING_RESPONSE" | jq -e '.message == "pong"' >/dev/null 2>&1; then
    log_pass "Ping endpoint responded correctly"
else
    log_fail "Ping endpoint response invalid"
fi

# Cleanup container
log_info "Cleaning up test container"
docker stop vulnerable-demo-test >/dev/null 2>&1
docker rm vulnerable-demo-test >/dev/null 2>&1

# Test 6: Scanner integration (if scanners are available)
log_test "Testing scanner integration"

# Test Trivy if available
if command -v trivy >/dev/null 2>&1; then
    log_info "Running Trivy scan..."
    if trivy image --format json --output trivy-test-results.json vulnerable-demo-test:latest >/dev/null 2>&1; then
        TRIVY_VULNS=$(jq '[.Results[]?.Vulnerabilities[]?] | length' trivy-test-results.json 2>/dev/null || echo "0")
        if [[ $TRIVY_VULNS -gt 0 ]]; then
            log_pass "Trivy detected $TRIVY_VULNS vulnerabilities"
        else
            log_fail "Trivy detected no vulnerabilities (expected some)"
        fi
        rm -f trivy-test-results.json
    else
        log_fail "Trivy scan failed"
    fi
else
    log_info "Trivy not available, skipping scan test"
fi

# Test Grype if available
if command -v grype >/dev/null 2>&1; then
    log_info "Running Grype scan..."
    if grype vulnerable-demo-test:latest -o json --file grype-test-results.json >/dev/null 2>&1; then
        GRYPE_VULNS=$(jq '.matches | length' grype-test-results.json 2>/dev/null || echo "0")
        if [[ $GRYPE_VULNS -gt 0 ]]; then
            log_pass "Grype detected $GRYPE_VULNS vulnerabilities"
        else
            log_fail "Grype detected no vulnerabilities (expected some)"
        fi
        rm -f grype-test-results.json
    else
        log_fail "Grype scan failed"
    fi
else
    log_info "Grype not available, skipping scan test"
fi

# Test 7: SBOM generation (if Syft is available)
log_test "Testing SBOM generation"
if command -v syft >/dev/null 2>&1; then
    log_info "Running Syft SBOM generation..."
    if syft vulnerable-demo-test:latest -o spdx-json=test-sbom.json >/dev/null 2>&1; then
        SBOM_PACKAGES=$(jq '.packages | length' test-sbom.json 2>/dev/null || echo "0")
        if [[ $SBOM_PACKAGES -gt 0 ]]; then
            log_pass "SBOM generated with $SBOM_PACKAGES packages"
        else
            log_fail "SBOM generated but contains no packages"
        fi
        rm -f test-sbom.json
    else
        log_fail "SBOM generation failed"
    fi
else
    log_info "Syft not available, skipping SBOM test"
fi

# Cleanup Docker image
log_info "Cleaning up test image"
docker rmi vulnerable-demo-test:latest >/dev/null 2>&1

# Test Summary
echo ""
echo "Test Summary"
echo "============"
echo "Total tests: $TEST_COUNT"
echo -e "${GREEN}Passed: $PASS_COUNT${NC}"
echo -e "${RED}Failed: $FAIL_COUNT${NC}"

if [[ $FAIL_COUNT -eq 0 ]]; then
    echo -e "\n${GREEN}All tests passed! Vulnerable demo application is working correctly.${NC}"
    exit 0
else
    echo -e "\n${RED}$FAIL_COUNT test(s) failed. Please check the output above.${NC}"
    exit 1
fi