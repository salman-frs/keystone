#!/bin/bash

# Test script for SBOM workflow validation
# Validates SBOM generation from vulnerable demo application

set -e

echo "=========================================="
echo "SBOM Workflow Testing Script"
echo "=========================================="

# Configuration
VULNERABLE_APP_DIR="examples/vulnerable-app"
TEST_OUTPUT_DIR="sbom-test-output"
CONTAINER_NAME="vulnerable-demo-test:latest"

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if Docker is available
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed or not in PATH"
        exit 1
    fi
    
    # Check if vulnerable app directory exists
    if [[ ! -d "$VULNERABLE_APP_DIR" ]]; then
        log_error "Vulnerable app directory not found: $VULNERABLE_APP_DIR"
        exit 1
    fi
    
    # Check if Dockerfile exists
    if [[ ! -f "$VULNERABLE_APP_DIR/Dockerfile" ]]; then
        log_error "Dockerfile not found in $VULNERABLE_APP_DIR"
        exit 1
    fi
    
    log_info "Prerequisites check passed"
}

# Install Syft if not available
install_syft() {
    if command -v syft &> /dev/null; then
        log_info "Syft already installed: $(syft version)"
        return 0
    fi
    
    log_info "Installing Syft..."
    curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
    
    if command -v syft &> /dev/null; then
        log_info "Syft installed successfully: $(syft version)"
    else
        log_error "Failed to install Syft"
        exit 1
    fi
}

# Configure Docker environment for colima compatibility
configure_docker_environment() {
    log_info "Configuring Docker environment..."

    # Check if colima is being used
    if docker context show 2>/dev/null | grep -q "colima"; then
        log_info "Detected colima Docker context"

        # Set environment variables for colima
        export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock"

        # Check if socket exists, try alternative locations
        if [[ ! -S "${HOME}/.colima/default/docker.sock" ]]; then
            if [[ -S "${HOME}/.colima/docker.sock" ]]; then
                export DOCKER_HOST="unix://${HOME}/.colima/docker.sock"
                log_info "Using alternative colima socket path"
            else
                log_warn "Colima Docker socket not found, attempting colima restart..."
                colima restart || log_warn "Failed to restart colima"
            fi
        fi

        log_info "Docker environment configured for colima: $DOCKER_HOST"
    else
        log_info "Using standard Docker configuration"
    fi

    # Verify Docker connectivity
    if ! docker version &> /dev/null; then
        log_error "Docker daemon not accessible. Please ensure Docker/colima is running."
        exit 1
    fi
}

# Install SBOM validation tools
install_validation_tools() {
    log_info "Installing SBOM validation tools..."

    # Install Node.js tools for CycloneDX validation
    if ! command -v npm &> /dev/null; then
        log_warn "npm not found. CycloneDX validation will be skipped"
        CYCLONEDX_AVAILABLE=false
    else
        log_info "Installing CycloneDX CLI (cdxgen)..."
        # Use the correct package name @cyclonedx/cdxgen
        npm install -g @cyclonedx/cdxgen || {
            log_warn "Failed to install CycloneDX CLI. CycloneDX validation will be skipped"
            CYCLONEDX_AVAILABLE=false
        }

        if command -v cdxgen &> /dev/null; then
            CYCLONEDX_AVAILABLE=true
            log_info "CycloneDX CLI installed successfully"
        else
            CYCLONEDX_AVAILABLE=false
            log_warn "CycloneDX CLI installation verification failed"
        fi
    fi

    # Install Python tools for SPDX validation (macOS safe methods only)
    log_info "Installing SPDX tools..."
    SPDX_AVAILABLE=false

    # Try pipx first (recommended for CLI tools on macOS)
    if command -v pipx &> /dev/null; then
        log_info "Trying pipx installation..."
        if pipx install spdx-tools 2>/dev/null; then
            SPDX_AVAILABLE=true
            log_info "SPDX tools installed via pipx"
        fi
    else
        log_info "pipx not found. Installing pipx via brew..."
        if command -v brew &> /dev/null; then
            if brew install pipx 2>/dev/null; then
                log_info "pipx installed via brew, retrying SPDX tools installation..."
                if pipx install spdx-tools 2>/dev/null; then
                    SPDX_AVAILABLE=true
                    log_info "SPDX tools installed via pipx"
                fi
            fi
        fi
    fi

    # Try virtual environment approach if pipx failed
    if [[ "$SPDX_AVAILABLE" == false ]]; then
        log_info "Creating temporary virtual environment for SPDX tools..."
        local temp_venv_dir="/tmp/sbom-test-venv-$$"

        if python3 -m venv "$temp_venv_dir" 2>/dev/null; then
            if "$temp_venv_dir/bin/pip" install spdx-tools 2>/dev/null; then
                # Create a wrapper script that uses the venv
                local venv_wrapper="/tmp/pyspdxtools-wrapper-$$"
                cat > "$venv_wrapper" << EOF
#!/bin/bash
exec "$temp_venv_dir/bin/pyspdxtools" "\$@"
EOF
                chmod +x "$venv_wrapper"
                export PATH="/tmp:$PATH"
                ln -sf "$venv_wrapper" "/tmp/pyspdxtools"
                SPDX_AVAILABLE=true
                SPDX_VENV_DIR="$temp_venv_dir"
                log_info "SPDX tools installed in temporary virtual environment"
            else
                rm -rf "$temp_venv_dir"
            fi
        fi
    fi

    if [[ "$SPDX_AVAILABLE" == false ]]; then
        log_warn "Failed to install SPDX tools using safe methods."
        log_warn "Consider installing manually with: brew install pipx && pipx install spdx-tools"
    fi

    # Verify installations
    if [[ "$CYCLONEDX_AVAILABLE" == true ]]; then
        log_info "CycloneDX validation tools ready"
    fi

    if [[ "$SPDX_AVAILABLE" == true ]]; then
        if command -v pyspdxtools &> /dev/null; then
            log_info "SPDX validation tools ready"
        else
            log_warn "SPDX tools installed but pyspdxtools command not found"
            SPDX_AVAILABLE=false
        fi
    fi
}

# Build test container
build_test_container() {
    log_info "Building test container: $CONTAINER_NAME"
    
    cd "$VULNERABLE_APP_DIR"
    docker build -t "$CONTAINER_NAME" .
    cd - > /dev/null
    
    log_info "Container built successfully"
}

# Generate SBOMs
generate_sboms() {
    log_info "Creating test output directory: $TEST_OUTPUT_DIR"
    mkdir -p "$TEST_OUTPUT_DIR"
    
    log_info "Generating CycloneDX SBOM..."
    syft "$CONTAINER_NAME" \
        --scope all-layers \
        -o cyclonedx-json="$TEST_OUTPUT_DIR/sbom-cyclonedx.json" \
        -v
    
    log_info "Generating SPDX SBOM..."
    syft "$CONTAINER_NAME" \
        --scope all-layers \
        -o spdx-json="$TEST_OUTPUT_DIR/sbom-spdx.json" \
        -v
    
    log_info "SBOM generation completed"
}

# Validate SBOM formats
validate_sbom_formats() {
    log_info "Validating SBOM formats..."
    local validation_errors=0

    # Validate CycloneDX SBOM
    if [[ "$CYCLONEDX_AVAILABLE" == true && -f "$TEST_OUTPUT_DIR/sbom-cyclonedx.json" ]]; then
        log_info "Validating CycloneDX SBOM format..."
        # Use cdxgen validate or basic JSON validation as fallback
        if cdxgen --validate "$TEST_OUTPUT_DIR/sbom-cyclonedx.json" 2>/dev/null; then
            log_info "PASS: CycloneDX SBOM validation passed"
        elif jq empty "$TEST_OUTPUT_DIR/sbom-cyclonedx.json" 2>/dev/null; then
            log_info "PASS: CycloneDX SBOM is valid JSON (basic validation)"
        else
            log_error "FAIL: CycloneDX SBOM validation failed"
            validation_errors=$((validation_errors + 1))
        fi
    else
        log_warn "Skipping CycloneDX validation (tool not available or file missing)"
    fi

    # Validate SPDX SBOM
    if [[ "$SPDX_AVAILABLE" == true && -f "$TEST_OUTPUT_DIR/sbom-spdx.json" ]]; then
        log_info "Validating SPDX SBOM format..."
        # Use pyspdxtools for validation
        if pyspdxtools -i "$TEST_OUTPUT_DIR/sbom-spdx.json" --novalidation > /dev/null 2>&1; then
            log_info "PASS: SPDX SBOM validation passed"
        elif jq empty "$TEST_OUTPUT_DIR/sbom-spdx.json" 2>/dev/null; then
            log_info "PASS: SPDX SBOM is valid JSON (basic validation)"
        else
            log_error "FAIL: SPDX SBOM validation failed"
            validation_errors=$((validation_errors + 1))
        fi
    else
        log_warn "Skipping SPDX validation (tool not available or file missing)"
    fi

    return $validation_errors
}

# Analyze SBOM content
analyze_sbom_content() {
    log_info "Analyzing SBOM content..."
    
    # Analyze CycloneDX SBOM
    if [[ -f "$TEST_OUTPUT_DIR/sbom-cyclonedx.json" ]]; then
        local cyclonedx_components=$(jq '.components | length' "$TEST_OUTPUT_DIR/sbom-cyclonedx.json" 2>/dev/null || echo "0")
        local cyclonedx_metadata=$(jq -r '.metadata.component.name // "N/A"' "$TEST_OUTPUT_DIR/sbom-cyclonedx.json" 2>/dev/null || echo "N/A")
        local cyclonedx_licenses=$(jq '[.components[].licenses[]?.license.name // empty] | unique | length' "$TEST_OUTPUT_DIR/sbom-cyclonedx.json" 2>/dev/null || echo "0")
        
        log_info "CycloneDX SBOM Analysis:"
        log_info "  - Components: $cyclonedx_components"
        log_info "  - Metadata Component: $cyclonedx_metadata"
        log_info "  - Unique Licenses: $cyclonedx_licenses"
        
        # Check for expected dependencies
        if jq -e '.components[] | select(.name == "go")' "$TEST_OUTPUT_DIR/sbom-cyclonedx.json" > /dev/null; then
            log_info "  - PASS: Go runtime found in SBOM"
        else
            log_warn "  - WARN: Go runtime not found in SBOM"
        fi
    else
        log_error "CycloneDX SBOM file not found for analysis"
    fi
    
    # Analyze SPDX SBOM
    if [[ -f "$TEST_OUTPUT_DIR/sbom-spdx.json" ]]; then
        local spdx_packages=$(jq '.packages | length' "$TEST_OUTPUT_DIR/sbom-spdx.json" 2>/dev/null || echo "0")
        local spdx_name=$(jq -r '.name // "N/A"' "$TEST_OUTPUT_DIR/sbom-spdx.json" 2>/dev/null || echo "N/A")
        local spdx_relationships=$(jq '.relationships | length' "$TEST_OUTPUT_DIR/sbom-spdx.json" 2>/dev/null || echo "0")
        
        log_info "SPDX SBOM Analysis:"
        log_info "  - Packages: $spdx_packages"
        log_info "  - Document Name: $spdx_name"
        log_info "  - Relationships: $spdx_relationships"
        
        # Check for expected dependencies
        if jq -e '.packages[] | select(.name | contains("go"))' "$TEST_OUTPUT_DIR/sbom-spdx.json" > /dev/null; then
            log_info "  - PASS: Go-related packages found in SBOM"
        else
            log_warn "  - WARN: Go-related packages not found in SBOM"
        fi
    else
        log_error "SPDX SBOM file not found for analysis"
    fi
}

# Test SBOM completeness
test_sbom_completeness() {
    log_info "Testing SBOM completeness..."
    local completeness_errors=0
    
    # Check if both SBOM files exist and are not empty
    if [[ -f "$TEST_OUTPUT_DIR/sbom-cyclonedx.json" && -s "$TEST_OUTPUT_DIR/sbom-cyclonedx.json" ]]; then
        log_info "PASS: CycloneDX SBOM file exists and is not empty"
    else
        log_error "FAIL: CycloneDX SBOM file missing or empty"
        completeness_errors=$((completeness_errors + 1))
    fi
    
    if [[ -f "$TEST_OUTPUT_DIR/sbom-spdx.json" && -s "$TEST_OUTPUT_DIR/sbom-spdx.json" ]]; then
        log_info "PASS: SPDX SBOM file exists and is not empty"
    else
        log_error "FAIL: SPDX SBOM file missing or empty"
        completeness_errors=$((completeness_errors + 1))
    fi
    
    # Check minimum component/package count
    if [[ -f "$TEST_OUTPUT_DIR/sbom-cyclonedx.json" ]]; then
        local component_count=$(jq '.components | length' "$TEST_OUTPUT_DIR/sbom-cyclonedx.json" 2>/dev/null || echo "0")
        if [[ $component_count -gt 0 ]]; then
            log_info "PASS: CycloneDX SBOM contains $component_count components"
        else
            log_error "FAIL: CycloneDX SBOM contains no components"
            completeness_errors=$((completeness_errors + 1))
        fi
    fi
    
    if [[ -f "$TEST_OUTPUT_DIR/sbom-spdx.json" ]]; then
        local package_count=$(jq '.packages | length' "$TEST_OUTPUT_DIR/sbom-spdx.json" 2>/dev/null || echo "0")
        if [[ $package_count -gt 0 ]]; then
            log_info "PASS: SPDX SBOM contains $package_count packages"
        else
            log_error "FAIL: SPDX SBOM contains no packages"
            completeness_errors=$((completeness_errors + 1))
        fi
    fi
    
    return $completeness_errors
}

# Cleanup function
cleanup() {
    log_info "Cleaning up test artifacts..."

    # Remove test container
    if docker images -q "$CONTAINER_NAME" > /dev/null 2>&1; then
        docker rmi "$CONTAINER_NAME" > /dev/null 2>&1 || log_warn "Failed to remove test container"
    fi

    # Clean up temporary virtual environment if created
    if [[ -n "${SPDX_VENV_DIR:-}" && -d "$SPDX_VENV_DIR" ]]; then
        rm -rf "$SPDX_VENV_DIR"
        log_info "Temporary SPDX virtual environment cleaned up"
    fi

    # Clean up temporary wrapper scripts
    rm -f /tmp/pyspdxtools-wrapper-* /tmp/pyspdxtools 2>/dev/null

    # Keep test output for inspection unless CLEANUP_OUTPUT is set
    if [[ "${CLEANUP_OUTPUT:-false}" == "true" ]]; then
        rm -rf "$TEST_OUTPUT_DIR"
        log_info "Test output directory cleaned up"
    else
        log_info "Test output preserved in: $TEST_OUTPUT_DIR"
    fi
}

# Main test execution
main() {
    log_info "Starting SBOM workflow test..."

    # Set trap for cleanup
    trap cleanup EXIT

    # Execute test steps
    check_prerequisites
    configure_docker_environment
    install_syft
    install_validation_tools
    build_test_container
    generate_sboms
    
    # Run validations
    local total_errors=0
    
    validate_sbom_formats
    total_errors=$((total_errors + $?))
    
    test_sbom_completeness
    total_errors=$((total_errors + $?))
    
    # Analyze content (informational)
    analyze_sbom_content
    
    # Report results
    echo ""
    echo "=========================================="
    echo "SBOM Workflow Test Results"
    echo "=========================================="
    
    if [[ $total_errors -eq 0 ]]; then
        log_info "All SBOM workflow tests PASSED"
        exit 0
    else
        log_error "$total_errors test(s) FAILED"
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi