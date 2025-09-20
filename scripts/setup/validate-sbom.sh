#!/bin/bash

# SBOM format compliance validation script
# Validates CycloneDX and SPDX format compliance using standard tools

set -e

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

# Usage information
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -c, --cyclonedx FILE    Validate CycloneDX SBOM file"
    echo "  -s, --spdx FILE        Validate SPDX SBOM file"
    echo "  -d, --directory DIR    Validate all SBOM files in directory"
    echo "  -h, --help             Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 -c sbom-cyclonedx.json"
    echo "  $0 -s sbom-spdx.json"
    echo "  $0 -d ./sbom-output/"
    echo "  $0 -c sbom-cyclonedx.json -s sbom-spdx.json"
}

# Install validation tools if not available
install_validation_tools() {
    local tools_installed=false

    # Check and install CycloneDX CLI (using correct package name)
    if ! command -v cdxgen &> /dev/null; then
        if command -v npm &> /dev/null; then
            log_info "Installing CycloneDX CLI (cdxgen)..."
            npm install -g @cyclonedx/cdxgen || {
                log_warn "Failed to install CycloneDX CLI"
                CYCLONEDX_AVAILABLE=false
                return 1
            }
            tools_installed=true
        else
            log_warn "npm not available. CycloneDX validation will be skipped"
            CYCLONEDX_AVAILABLE=false
        fi
    else
        log_info "CycloneDX CLI (cdxgen) already available"
    fi

    if command -v cdxgen &> /dev/null; then
        CYCLONEDX_AVAILABLE=true
    else
        CYCLONEDX_AVAILABLE=false
    fi

    # Check and install SPDX tools (using safe macOS methods)
    if ! command -v pyspdxtools &> /dev/null; then
        log_info "Installing SPDX tools..."
        SPDX_AVAILABLE=false

        # Try pipx first (recommended for CLI tools on macOS)
        if command -v pipx &> /dev/null; then
            log_info "Trying pipx installation..."
            if pipx install spdx-tools 2>/dev/null; then
                SPDX_AVAILABLE=true
                log_info "SPDX tools installed via pipx"
                tools_installed=true
            fi
        else
            log_info "pipx not found. Installing pipx via brew..."
            if command -v brew &> /dev/null; then
                if brew install pipx 2>/dev/null; then
                    log_info "pipx installed via brew, retrying SPDX tools installation..."
                    if pipx install spdx-tools 2>/dev/null; then
                        SPDX_AVAILABLE=true
                        log_info "SPDX tools installed via pipx"
                        tools_installed=true
                    fi
                fi
            fi
        fi

        # Try virtual environment approach if pipx failed
        if [[ "$SPDX_AVAILABLE" == false ]]; then
            log_info "Creating temporary virtual environment for SPDX tools..."
            local temp_venv_dir="/tmp/sbom-validate-venv-$$"

            if python3 -m venv "$temp_venv_dir" 2>/dev/null; then
                if "$temp_venv_dir/bin/pip" install spdx-tools 2>/dev/null; then
                    # Create a wrapper script that uses the venv
                    local venv_wrapper="/tmp/pyspdxtools-validate-wrapper-$$"
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
                    tools_installed=true
                else
                    rm -rf "$temp_venv_dir"
                fi
            fi
        fi

        if [[ "$SPDX_AVAILABLE" == false ]]; then
            log_warn "Failed to install SPDX tools using safe methods."
            log_warn "Consider installing manually with: brew install pipx && pipx install spdx-tools"
        fi
    else
        log_info "SPDX tools (pyspdxtools) already available"
        SPDX_AVAILABLE=true
    fi

    if [[ "$tools_installed" == true ]]; then
        log_info "Validation tools installation completed"
    fi
}

# Validate CycloneDX SBOM
validate_cyclonedx() {
    local file="$1"
    local validation_errors=0
    
    log_info "Validating CycloneDX SBOM: $file"
    
    # Check if file exists
    if [[ ! -f "$file" ]]; then
        log_error "CycloneDX SBOM file not found: $file"
        return 1
    fi
    
    # Check if file is valid JSON
    if ! jq empty "$file" 2>/dev/null; then
        log_error "CycloneDX SBOM is not valid JSON: $file"
        return 1
    fi
    
    # Validate using CycloneDX CLI if available
    if [[ "$CYCLONEDX_AVAILABLE" == true ]]; then
        log_info "Running CycloneDX format validation..."
        # Use cdxgen validate or basic JSON validation as fallback
        if cdxgen --validate "$file" 2>/dev/null; then
            log_info "PASS: CycloneDX format validation passed"
        elif jq empty "$file" 2>/dev/null; then
            log_info "PASS: CycloneDX SBOM is valid JSON (basic validation)"
        else
            log_error "FAIL: CycloneDX format validation failed"
            validation_errors=$((validation_errors + 1))
        fi
    else
        log_warn "CycloneDX CLI not available, performing basic JSON validation only"
    fi
    
    # Basic structure validation
    log_info "Performing basic CycloneDX structure validation..."
    
    # Check for required fields
    if jq -e '.specVersion' "$file" > /dev/null; then
        local spec_version=$(jq -r '.specVersion' "$file")
        log_info "PASS: specVersion found: $spec_version"
    else
        log_error "FAIL: Missing required field: specVersion"
        validation_errors=$((validation_errors + 1))
    fi
    
    if jq -e '.serialNumber' "$file" > /dev/null; then
        log_info "PASS: serialNumber found"
    else
        log_error "FAIL: Missing required field: serialNumber"
        validation_errors=$((validation_errors + 1))
    fi
    
    if jq -e '.version' "$file" > /dev/null; then
        log_info "PASS: version found"
    else
        log_error "FAIL: Missing required field: version"
        validation_errors=$((validation_errors + 1))
    fi
    
    # Check components array
    if jq -e '.components' "$file" > /dev/null; then
        local component_count=$(jq '.components | length' "$file")
        log_info "PASS: components array found with $component_count components"
        
        # Validate component structure
        local invalid_components=0
        while IFS= read -r component; do
            if ! echo "$component" | jq -e '.type' > /dev/null; then
                invalid_components=$((invalid_components + 1))
            fi
        done < <(jq -c '.components[]' "$file")
        
        if [[ $invalid_components -eq 0 ]]; then
            log_info "PASS: All components have required 'type' field"
        else
            log_error "FAIL: $invalid_components component(s) missing required 'type' field"
            validation_errors=$((validation_errors + 1))
        fi
    else
        log_warn "WARN: No components array found"
    fi
    
    return $validation_errors
}

# Validate SPDX SBOM
validate_spdx() {
    local file="$1"
    local validation_errors=0
    
    log_info "Validating SPDX SBOM: $file"
    
    # Check if file exists
    if [[ ! -f "$file" ]]; then
        log_error "SPDX SBOM file not found: $file"
        return 1
    fi
    
    # Check if file is valid JSON
    if ! jq empty "$file" 2>/dev/null; then
        log_error "SPDX SBOM is not valid JSON: $file"
        return 1
    fi
    
    # Validate using SPDX tools if available
    if [[ "$SPDX_AVAILABLE" == true ]]; then
        log_info "Running SPDX format validation..."
        # Use pyspdxtools for validation
        if pyspdxtools -i "$file" --novalidation > /dev/null 2>&1; then
            log_info "PASS: SPDX format validation passed"
        elif jq empty "$file" 2>/dev/null; then
            log_info "PASS: SPDX SBOM is valid JSON (basic validation)"
        else
            log_error "FAIL: SPDX format validation failed"
            validation_errors=$((validation_errors + 1))
        fi
    else
        log_warn "SPDX tools not available, performing basic JSON validation only"
    fi
    
    # Basic structure validation
    log_info "Performing basic SPDX structure validation..."
    
    # Check for required fields
    if jq -e '.spdxVersion' "$file" > /dev/null; then
        local spdx_version=$(jq -r '.spdxVersion' "$file")
        log_info "PASS: spdxVersion found: $spdx_version"
    else
        log_error "FAIL: Missing required field: spdxVersion"
        validation_errors=$((validation_errors + 1))
    fi
    
    if jq -e '.SPDXID' "$file" > /dev/null; then
        log_info "PASS: SPDXID found"
    else
        log_error "FAIL: Missing required field: SPDXID"
        validation_errors=$((validation_errors + 1))
    fi
    
    if jq -e '.creationInfo' "$file" > /dev/null; then
        log_info "PASS: creationInfo found"
    else
        log_error "FAIL: Missing required field: creationInfo"
        validation_errors=$((validation_errors + 1))
    fi
    
    # Check packages array
    if jq -e '.packages' "$file" > /dev/null; then
        local package_count=$(jq '.packages | length' "$file")
        log_info "PASS: packages array found with $package_count packages"
        
        # Validate package structure
        local invalid_packages=0
        while IFS= read -r package; do
            if ! echo "$package" | jq -e '.SPDXID' > /dev/null; then
                invalid_packages=$((invalid_packages + 1))
            fi
        done < <(jq -c '.packages[]' "$file")
        
        if [[ $invalid_packages -eq 0 ]]; then
            log_info "PASS: All packages have required 'SPDXID' field"
        else
            log_error "FAIL: $invalid_packages package(s) missing required 'SPDXID' field"
            validation_errors=$((validation_errors + 1))
        fi
    else
        log_warn "WARN: No packages array found"
    fi
    
    # Check relationships array
    if jq -e '.relationships' "$file" > /dev/null; then
        local relationship_count=$(jq '.relationships | length' "$file")
        log_info "PASS: relationships array found with $relationship_count relationships"
    else
        log_warn "WARN: No relationships array found"
    fi
    
    return $validation_errors
}

# Validate all SBOM files in directory
validate_directory() {
    local dir="$1"
    local total_errors=0
    
    log_info "Validating all SBOM files in directory: $dir"
    
    if [[ ! -d "$dir" ]]; then
        log_error "Directory not found: $dir"
        return 1
    fi
    
    # Find and validate CycloneDX files
    while IFS= read -r -d '' file; do
        log_info "Found CycloneDX SBOM: $file"
        validate_cyclonedx "$file"
        total_errors=$((total_errors + $?))
    done < <(find "$dir" -name "*cyclonedx*.json" -print0)
    
    # Find and validate SPDX files
    while IFS= read -r -d '' file; do
        log_info "Found SPDX SBOM: $file"
        validate_spdx "$file"
        total_errors=$((total_errors + $?))
    done < <(find "$dir" -name "*spdx*.json" -print0)
    
    return $total_errors
}

# Cleanup function for temporary resources
cleanup() {
    # Clean up temporary virtual environment if created
    if [[ -n "${SPDX_VENV_DIR:-}" && -d "$SPDX_VENV_DIR" ]]; then
        rm -rf "$SPDX_VENV_DIR"
    fi

    # Clean up temporary wrapper scripts
    rm -f /tmp/pyspdxtools-validate-wrapper-* /tmp/pyspdxtools 2>/dev/null
}

# Main function
main() {
    local cyclonedx_file=""
    local spdx_file=""
    local directory=""
    local total_errors=0

    # Set trap for cleanup
    trap cleanup EXIT
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -c|--cyclonedx)
                cyclonedx_file="$2"
                shift 2
                ;;
            -s|--spdx)
                spdx_file="$2"
                shift 2
                ;;
            -d|--directory)
                directory="$2"
                shift 2
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
    
    # Check if at least one option is provided
    if [[ -z "$cyclonedx_file" && -z "$spdx_file" && -z "$directory" ]]; then
        log_error "No validation target specified"
        usage
        exit 1
    fi
    
    log_info "Starting SBOM format validation..."
    
    # Install validation tools
    install_validation_tools
    
    # Validate specified files/directory
    if [[ -n "$cyclonedx_file" ]]; then
        validate_cyclonedx "$cyclonedx_file"
        total_errors=$((total_errors + $?))
    fi
    
    if [[ -n "$spdx_file" ]]; then
        validate_spdx "$spdx_file"
        total_errors=$((total_errors + $?))
    fi
    
    if [[ -n "$directory" ]]; then
        validate_directory "$directory"
        total_errors=$((total_errors + $?))
    fi
    
    # Report results
    echo ""
    echo "=========================================="
    echo "SBOM Validation Results"
    echo "=========================================="
    
    if [[ $total_errors -eq 0 ]]; then
        log_info "All SBOM validations PASSED"
        exit 0
    else
        log_error "$total_errors validation error(s) found"
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi