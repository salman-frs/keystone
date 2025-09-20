# Troubleshooting Guide

This guide addresses common setup issues, error conditions, and diagnostic procedures for the Keystone Security Platform.

## Quick Diagnostic Commands

Run these commands to quickly assess your environment:

```bash
# Environment check
./scripts/setup/validate-scanners.sh

```


### Homebrew and Tool Installation

#### Issue: Homebrew security tool installation fails
```bash
# Error: Package not found or installation fails
brew install trivy
# Error: No available formula with name "trivy"
```

**Solutions:**
```bash
# Update Homebrew
brew update
brew upgrade

# Install from specific taps
brew install aquasecurity/trivy/trivy
brew install anchore/grype/grype
brew install anchore/syft/syft

# Verify installations
which trivy grype syft cosign
```

#### Issue: Permission denied during tool installation
```bash
# Error: Permission denied when installing tools
```

**Solutions:**
```bash
# Fix Homebrew permissions
sudo chown -R $(whoami) /usr/local/var/homebrew
sudo chmod u+w /usr/local/var/homebrew

# Alternative: Use --force if safe
brew install --force trivy

# Check directory permissions
ls -la /usr/local/bin/ | grep -E "(trivy|grype|syft|cosign)"
```

## GitHub CLI and Authentication Issues

### GitHub CLI Installation and Setup

#### Issue: GitHub CLI authentication fails
```bash
# Error: Authentication required
gh repo view salman-frs/keystone
# HTTP 401: Requires authentication
```

**Solutions:**
```bash
# Re-authenticate with GitHub
gh auth logout
gh auth login

# Check authentication status
gh auth status

# Verify token permissions
gh api user
gh api repos/salman-frs/keystone
```

#### Issue: GitHub CLI not found or outdated
```bash
# Error: command not found: gh
```

**Solutions:**
```bash
# Install GitHub CLI
brew install gh

# Update to latest version
brew upgrade gh

# Verify installation
gh --version
which gh
```

#### Issue: Repository access denied
```bash
# Error: Resource not accessible by integration
gh repo view salman-frs/keystone
# GraphQL: Resource not accessible by integration
```

**Solutions:**
```bash
# Check repository permissions
gh api repos/salman-frs/keystone --jq '.permissions'

# Re-authenticate with broader scopes
gh auth refresh --scopes repo,workflow,admin:org

# Test with public repository access
gh repo view microsoft/vscode
```

## Security Scanner Issues

### Trivy Scanner Problems

#### Issue: Trivy database update fails
```bash
# Error: Database update failure
trivy image --download-db-only
# FATAL: failed to download vulnerability DB
```

**Solutions:**
```bash
# Clear Trivy cache and retry
rm -rf ~/.cache/trivy
trivy image --download-db-only

# Use alternative DB source
trivy --db-repository ghcr.io/aquasecurity/trivy-db image --download-db-only

# Check network connectivity
curl -I https://github.com/aquasecurity/trivy-db/releases
```

#### Issue: Trivy scans fail with permission errors
```bash
# Error: Permission denied during scan
trivy fs examples/vulnerable-app/
# FATAL: scan error: failed to analyze
```

**Solutions:**
```bash
# Check file permissions
ls -la examples/vulnerable-app/
chmod -R 755 examples/vulnerable-app/

# Run with different user context
sudo trivy fs examples/vulnerable-app/

# Use Docker-based scanning
docker run --rm -v $(pwd):/workspace aquasecurity/trivy fs /workspace/examples/vulnerable-app/
```

#### Issue: Trivy produces no results
```bash
# Issue: Scan completes but finds no vulnerabilities
trivy fs examples/vulnerable-app/ --format json
# Returns empty results
```

**Solutions:**
```bash
# Update vulnerability database
trivy image --download-db-only

# Scan with verbose output
trivy fs examples/vulnerable-app/ --debug

# Check scan target
ls -la examples/vulnerable-app/go.mod
trivy fs examples/vulnerable-app/ --security-checks vuln,config
```

### Grype Scanner Problems

#### Issue: Grype database update fails
```bash
# Error: Database update error
grype db update
# failed to update vulnerability database
```

**Solutions:**
```bash
# Clear Grype cache
rm -rf ~/.grype
grype db update

# Check database status
grype db status

# Manual database location
export GRYPE_DB_CACHE_DIR=/tmp/grype-db
grype db update
```

#### Issue: Grype fails to detect vulnerabilities
```bash
# Issue: No vulnerabilities found in known vulnerable app
grype examples/vulnerable-app/
# No vulnerabilities found
```

**Solutions:**
```bash
# Update database first
grype db update

# Scan with verbose output
grype examples/vulnerable-app/ -v

# Check supported ecosystems
grype --help | grep -A 10 "supported ecosystems"

# Force specific cataloger
grype examples/vulnerable-app/ --catalogers go-module
```

#### Issue: Grype performance issues
```bash
# Issue: Grype scans are extremely slow
```

**Solutions:**
```bash
# Limit catalogers for faster scanning
grype examples/vulnerable-app/ --catalogers go-module,go-binary

# Use specific output format
grype examples/vulnerable-app/ -o json

# Check system resources
top | grep grype
```

### Syft SBOM Generation Issues

#### Issue: Syft fails to generate SBOM
```bash
# Error: SBOM generation fails
syft examples/vulnerable-app/ -o spdx-json
# failed to catalog
```

**Solutions:**
```bash
# Run with verbose output
syft examples/vulnerable-app/ -v

# Use specific catalogers
syft examples/vulnerable-app/ --catalogers go-module

# Check target directory
ls -la examples/vulnerable-app/
file examples/vulnerable-app/go.mod

# Try different output formats
syft examples/vulnerable-app/ -o table
```

#### Issue: SBOM content validation fails
```bash
# Issue: Generated SBOM appears incomplete or invalid
```

**Solutions:**
```bash
# Validate SBOM structure
cat sbom-output.json | jq 'keys'

# Check package count
cat sbom-output.json | jq '.packages | length'  # SPDX
cat sbom-output.json | jq '.components | length'  # CycloneDX

# Compare different formats
syft examples/vulnerable-app/ -o spdx-json=spdx.json
syft examples/vulnerable-app/ -o cyclonedx-json=cyclonedx.json
```

## GitHub Actions Workflow Issues

### Workflow Configuration Problems

#### Issue: Workflow fails to trigger
```bash
# Issue: GitHub Actions workflow doesn't run on push/PR
```

**Solutions:**
```bash
# Check workflow syntax
gh workflow list
gh workflow view security-pipeline

# Validate YAML syntax
yamllint .github/workflows/security-pipeline.yaml

# Check workflow permissions
cat .github/workflows/security-pipeline.yaml | grep -A 5 permissions:

# Manual workflow trigger
gh workflow run security-pipeline.yaml
```

#### Issue: Workflow runner out of disk space
```bash
# Error: No space left on device in GitHub Actions
```

**Solutions:**
```bash
# Check workflow for cleanup steps
grep -A 5 -B 5 "clean" .github/workflows/security-pipeline.yaml

# Add cleanup step to workflow
echo "
- name: Clean up space
  run: |
    docker system prune -af
    df -h
" >> .github/workflows/security-pipeline.yaml
```

#### Issue: Workflow artifact upload fails
```bash
# Error: Artifact upload fails in GitHub Actions
```

**Solutions:**
```bash
# Check artifact paths in workflow
grep -A 10 "upload-artifact" .github/workflows/security-pipeline.yaml

# Verify output directories exist
ls -la scanner-test-output/ sbom-test-output/

# Check file permissions
find scanner-test-output/ -type f -exec ls -la {} \;
```

### Local Workflow Testing

#### Issue: Act (local GitHub Actions) fails
```bash
# Error: act fails to run workflow locally
act push
# Error: failed to run workflow
```

**Solutions:**
```bash
# Install act if not present
brew install act

# Check available workflows
act -l

# Run with verbose output
act push -v

# Use specific runner image
act push --platform ubuntu-latest=catthehacker/ubuntu:act-latest

# Dry run to check workflow
act push --dry-run
```

## Go Development Issues

### Go Build and Dependency Problems

#### Issue: Go build fails for vulnerable app
```bash
# Error: Build fails in examples/vulnerable-app
cd examples/vulnerable-app
go build
# build constraints exclude all Go files
```

**Solutions:**
```bash
# Check Go version
go version  # Should be 1.21+

# Clean and rebuild
go clean
go mod tidy
go build -v

# Check for build constraints
grep -r "//go:build" .
grep -r "// +build" .

# Verify module structure
cat go.mod
go mod verify
```

#### Issue: Go module dependency resolution fails
```bash
# Error: Dependency resolution problems
go mod tidy
# go: errors parsing go.mod
```

**Solutions:**
```bash
# Check go.mod syntax
cat go.mod

# Clean module cache
go clean -modcache
go mod download

# Reinitialize module if corrupted
mv go.mod go.mod.backup
go mod init keystone-vulnerable-app
go mod tidy

# Compare with working version
git checkout go.mod go.sum
```

## File System and Permission Issues

### Output Directory Problems

#### Issue: Permission denied when creating output files
```bash
# Error: Cannot create output files
./scripts/setup/test-workflow.sh
# Permission denied: scanner-test-output/
```

**Solutions:**
```bash
# Create output directories with proper permissions
mkdir -p scanner-test-output sbom-test-output
chmod 755 scanner-test-output sbom-test-output

# Fix ownership if needed
sudo chown -R $(whoami) scanner-test-output sbom-test-output

# Check parent directory permissions
ls -la | grep -E "(scanner|sbom)"
```

#### Issue: Script execution permission denied
```bash
# Error: Permission denied for validation scripts
./scripts/setup/validate-scanners.sh
# Permission denied
```

**Solutions:**
```bash
# Make scripts executable
chmod +x scripts/setup/*.sh

# Check script permissions
ls -la scripts/setup/

# Alternative execution method
bash scripts/setup/validate-scanners.sh
```

## Network and Connectivity Issues

### Internet Connectivity Problems

#### Issue: Scanner database updates fail due to network
```bash
# Error: Network timeouts during database updates
```

**Solutions:**
```bash
# Test basic connectivity
ping github.com
curl -I https://github.com

# Check proxy settings
echo $HTTP_PROXY $HTTPS_PROXY

# Use alternative DNS
echo "nameserver 8.8.8.8" | sudo tee /etc/resolver/github.com

# Retry with longer timeout
trivy --timeout 10m image --download-db-only
```

#### Issue: GitHub API rate limiting
```bash
# Error: API rate limit exceeded
gh api repos/salman-frs/keystone
# API rate limit exceeded
```

**Solutions:**
```bash
# Check rate limit status
gh api rate_limit

# Wait for rate limit reset
date -r $(gh api rate_limit --jq '.resources.core.reset')

# Use authenticated requests
gh auth status
gh auth refresh
```

## Performance and Resource Issues

### Memory and CPU Problems

#### Issue: System becomes unresponsive during scans
```bash
# Issue: High memory/CPU usage during security scanning
```

**Solutions:**
```bash
# Monitor resource usage
top | grep -E "(trivy|grype|syft)"

# Limit concurrent operations
# Edit scripts to run scanners sequentially instead of parallel

# Increase VM resources for Colima
colima stop
colima start --cpu 6 --memory 12

# Use Docker resource limits
docker run --memory=2g --cpus=2 aquasecurity/trivy:latest
```

#### Issue: Disk space exhaustion
```bash
# Error: No space left on device
```

**Solutions:**
```bash
# Check disk usage
df -h
du -sh ~/.cache/trivy ~/.grype

# Clean up scanner caches
rm -rf ~/.cache/trivy ~/.grype

# Clean Docker resources
docker system prune -af
colima ssh 'docker system df'

# Remove old output files
find . -name "*-test-output" -type d -exec rm -rf {} +
```

## Debugging Tools and Techniques

### Comprehensive Diagnostic Script

```bash
#!/bin/bash
# comprehensive-diagnostics.sh

echo "=== Keystone Diagnostic Report ==="
echo "Generated: $(date)"
echo ""

echo "=== System Information ==="
uname -a
sw_vers
echo ""

echo "=== Tool Versions ==="
echo "Go: $(go version 2>/dev/null || echo 'NOT FOUND')"
echo "Docker: $(docker --version 2>/dev/null || echo 'NOT FOUND')"
echo "Colima: $(colima version 2>/dev/null || echo 'NOT FOUND')"
echo "GitHub CLI: $(gh --version 2>/dev/null || echo 'NOT FOUND')"
echo "Trivy: $(trivy --version 2>/dev/null || echo 'NOT FOUND')"
echo "Grype: $(grype version 2>/dev/null || echo 'NOT FOUND')"
echo "Syft: $(syft version 2>/dev/null || echo 'NOT FOUND')"
echo "Cosign: $(cosign version 2>/dev/null || echo 'NOT FOUND')"
echo ""

echo "=== Docker Status ==="
docker context ls 2>/dev/null || echo "Docker not available"
docker ps 2>/dev/null || echo "Cannot connect to Docker daemon"
echo ""

echo "=== Colima Status ==="
colima status 2>/dev/null || echo "Colima not running"
colima list 2>/dev/null || echo "No Colima instances"
echo ""

echo "=== GitHub Authentication ==="
gh auth status 2>/dev/null || echo "GitHub CLI not authenticated"
echo ""

echo "=== Project Structure ==="
ls -la | grep -E "(examples|scripts|user-docs)"
ls -la examples/vulnerable-app/ 2>/dev/null || echo "Vulnerable app directory not found"
echo ""

echo "=== Validation Test ==="
./scripts/setup/validate-scanners.sh 2>&1 || echo "Validation script failed"
```

### Log Analysis Commands

```bash
# Check system logs for Docker/Colima issues
log show --predicate 'process == "colima"' --last 1h

# Check GitHub CLI logs
gh config list
cat ~/.config/gh/hosts.yml

# Check tool cache status
ls -la ~/.cache/trivy/
ls -la ~/.grype/

# Check network connectivity
nslookup github.com
curl -v https://api.github.com/zen
```

## Recovery Procedures

### Complete Environment Reset

```bash
#!/bin/bash
# reset-environment.sh

echo "Resetting Keystone development environment..."

# Stop and remove Colima
colima stop
colima delete

# Clean tool caches
rm -rf ~/.cache/trivy
rm -rf ~/.grype
rm -rf ~/.syft

# Reset GitHub CLI
gh auth logout --hostname github.com

# Clean project output directories
rm -rf scanner-test-output sbom-test-output

# Reinstall tools
brew uninstall trivy grype syft cosign
brew install aquasecurity/trivy/trivy
brew install anchore/grype/grype
brew install anchore/syft/syft
brew install sigstore/tap/cosign

# Restart Colima
colima start --cpu 4 --memory 8 --disk 60

# Re-authenticate GitHub CLI
gh auth login

# Validate setup
./scripts/setup/validate-scanners.sh

echo "Environment reset complete!"
```

### Selective Component Reset

```bash
# Reset only Docker/Colima
colima restart

# Reset only security tools
rm -rf ~/.cache/trivy ~/.grype
trivy image --download-db-only
grype db update

# Reset only GitHub authentication
gh auth logout
gh auth login

# Reset only project outputs
rm -rf scanner-test-output sbom-test-output
mkdir -p scanner-test-output sbom-test-output
```

## Getting Help

### Community Resources
- **Trivy Documentation**: https://aquasecurity.github.io/trivy/
- **Grype Documentation**: https://github.com/anchore/grype
- **Syft Documentation**: https://github.com/anchore/syft
- **GitHub CLI Documentation**: https://cli.github.com/manual/
- **Colima Documentation**: https://github.com/abiosoft/colima

### Issue Reporting Template

When reporting issues, include:

1. **Environment Information**:
   ```bash
   ./comprehensive-diagnostics.sh > diagnostic-report.txt
   ```

2. **Specific Error Messages**: Copy exact error text and commands

3. **Reproduction Steps**: Detailed steps to reproduce the issue

4. **Expected vs Actual Behavior**: What should happen vs what actually happens

5. **System Context**: macOS version, available resources, network environment

This troubleshooting guide should help resolve most common issues encountered during Keystone setup and operation.