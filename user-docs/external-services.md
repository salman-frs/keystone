# External Service Integration Guide

## Overview

This guide provides step-by-step procedures for integrating with external services required by the Keystone DevSecOps platform. These services are critical for vulnerability scanning, artifact signing, and automated remediation workflows.

> **Architecture Reference:** For technical integration patterns and system design decisions, see the [External Service Integration Architecture](../docs/architecture.md#external-service-integration-architecture) section.

## Prerequisites

Before setting up external services, ensure you have:
- GitHub repository with Actions enabled
- Local development environment with Docker/Colima
- Administrative access to configure repository secrets
- Valid email address for API key registrations

## API Authentication Setup

### National Vulnerability Database (NVD)

The NVD provides comprehensive vulnerability data that enhances scanner accuracy and reduces false positives.

#### Registration Process

1. **Navigate to NVD Developer Portal**
   ```bash
   # Open registration page
   open "https://nvd.nist.gov/developers/request-an-api-key"
   ```

2. **Complete Application Form**
   - **Organization Name:** Your organization or "Personal Development"
   - **Contact Email:** Use a monitored email address
   - **Intended Use:** "DevSecOps vulnerability scanning automation"
   - **Expected Request Volume:** "< 1000 requests per day"

3. **Wait for Approval** ⏱️
   - **Typical Delivery:** 3-5 business days
   - **Status Check:** Check email for approval notification
   - **API Key Format:** `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`

#### Configuration Setup

**Local Development:**
```bash
# Set environment variable for local testing
export NVD_API_KEY="your_api_key_here"

# Add to your shell profile for persistence
echo 'export NVD_API_KEY="your_api_key_here"' >> ~/.zshrc
source ~/.zshrc
```

**GitHub Actions Configuration:**
1. **Navigate to Repository Settings**
   - Go to `Settings` → `Secrets and variables` → `Actions`
2. **Add Repository Secret**
   - Name: `NVD_API_KEY`
   - Value: `your_api_key_here`
3. **Verify Access in Workflow**
   ```yaml
   # In your GitHub Actions workflow
   env:
     NVD_API_KEY: ${{ secrets.NVD_API_KEY }}
   ```

#### Usage Validation

**Test API Key:**
```bash
# Test API connectivity
curl -H "apikey: $NVD_API_KEY" \
  "https://services.nvd.nist.gov/rest/json/cves/2.0?resultsPerPage=1"
```

**Expected Response:**
```json
{
  "resultsPerPage": 1,
  "startIndex": 0,
  "totalResults": 240000,
  "vulnerabilities": [...]
}
```

### GitHub Security Advisory Database

Leverages GitHub's curated security advisory data for enhanced vulnerability intelligence.

#### Authentication Setup

**Using Personal Access Token:**
1. **Generate Token**
   - Go to `Settings` → `Developer settings` → `Personal access tokens` → `Tokens (classic)`
   - Click `Generate new token (classic)`

2. **Configure Scopes**
   Required scopes:
   - ✅ `repo` (Full repository access)
   - ✅ `security_events` (Security advisory access)
   - ✅ `actions:read` (Actions workflow access)

3. **Store Token Securely**
   ```bash
   # Local development
   export GITHUB_TOKEN="ghp_your_token_here"

   # GitHub Actions (automatic)
   # Uses built-in GITHUB_TOKEN with appropriate permissions
   ```

#### API Endpoints Reference

**REST API Examples:**
```bash
# Get security advisories
curl -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/advisories?per_page=10"

# Get repository-specific advisories
curl -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/repos/owner/repo/security-advisories"
```

**GraphQL API (More Efficient):**
```bash
curl -H "Authorization: token $GITHUB_TOKEN" \
  -d '{"query": "query { securityAdvisories(first: 10) { nodes { ghsaId summary severity } } }"}' \
  "https://api.github.com/graphql"
```

### Sigstore/Rekor Integration

Provides cryptographic signing and transparency log capabilities without private key management.

#### Authentication (Automatic)

Sigstore integration uses **GitHub OIDC tokens** automatically in GitHub Actions:

```yaml
# GitHub Actions workflow - no manual setup required
permissions:
  id-token: write  # Required for OIDC token
  contents: read   # Required for repository access

steps:
  - name: Install Cosign
    uses: sigstore/cosign-installer@v3

  - name: Sign artifact (keyless)
    run: |
      cosign sign --yes ghcr.io/owner/repo:latest
    env:
      COSIGN_EXPERIMENTAL: 1  # Enable keyless signing
```

#### Service Endpoints

**Production Endpoints:**
- **Fulcio CA:** `https://fulcio.sigstore.dev`
- **Rekor Transparency Log:** `https://rekor.sigstore.dev`
- **TUF Root:** `https://tuf-repo-cdn.sigstore.dev`

**Verification Example:**
```bash
# Verify signed artifact
cosign verify \
  --certificate-identity-regexp="https://github.com/.*" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/owner/repo:latest
```

## Rate Limiting Management

### Understanding GitHub API Limits

**Primary Rate Limits:**
- **REST API:** 5,000 requests per hour (authenticated)
- **GraphQL API:** 5,000 points per hour (queries consume variable points)
- **Secondary Rate Limit:** 100 content creation requests per minute

**Checking Current Usage:**
```bash
# Check rate limit status
curl -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/rate_limit"
```

**Response Format:**
```json
{
  "resources": {
    "core": {
      "limit": 5000,
      "remaining": 4999,
      "reset": 1640995200,
      "used": 1
    }
  }
}
```

### Rate Limiting Implementation

**Circuit Breaker Configuration:**
```bash
# Environment variables for rate limiting
export GITHUB_API_RATE_LIMIT_THRESHOLD=1000  # Stop at 1000 remaining (20% buffer)
export GITHUB_API_BACKOFF_BASE=2             # Exponential backoff base
export GITHUB_API_MAX_BACKOFF=60             # Maximum 60 seconds backoff
export GITHUB_API_CIRCUIT_TIMEOUT=300       # 5 minutes circuit breaker timeout
```

**Monitoring Rate Limits in Workflow:**
```yaml
# GitHub Actions step to monitor rate limits
- name: Check Rate Limit
  run: |
    REMAINING=$(curl -s -H "Authorization: token $GITHUB_TOKEN" \
      "https://api.github.com/rate_limit" | jq -r '.resources.core.remaining')

    if [ "$REMAINING" -lt 1000 ]; then
      echo "::warning::Rate limit low: $REMAINING remaining"
      echo "rate_limited=true" >> $GITHUB_OUTPUT
    fi
  id: rate_check

- name: Skip High-Volume Operations
  if: steps.rate_check.outputs.rate_limited == 'true'
  run: |
    echo "Skipping automated issue creation due to rate limits"
    echo "Will retry in next workflow run"
```

### High-Volume Event Handling

When processing multiple vulnerabilities simultaneously:

**Batch Processing Strategy:**
```bash
#!/bin/bash
# Script: batch-issue-creation.sh

MAX_ISSUES_PER_HOUR=20
CURRENT_HOUR=$(date +%Y%m%d%H)
ISSUE_COUNT_FILE="/tmp/issue_count_$CURRENT_HOUR"

# Check current hour's issue count
CURRENT_COUNT=$(cat "$ISSUE_COUNT_FILE" 2>/dev/null || echo 0)

if [ "$CURRENT_COUNT" -ge "$MAX_ISSUES_PER_HOUR" ]; then
    echo "Rate limit reached for this hour. Queuing remaining issues."
    # Add to queue for next hour processing
    echo "$@" >> /tmp/issue_queue_$(date -d '+1 hour' +%Y%m%d%H)
    exit 0
fi

# Process issue creation
# ... issue creation logic here ...

# Update counter
echo $((CURRENT_COUNT + 1)) > "$ISSUE_COUNT_FILE"
```

**Priority Queue Implementation:**
```yaml
# GitHub Actions workflow with priority processing
- name: Process Vulnerabilities by Priority
  run: |
    # Critical vulnerabilities (immediate processing)
    jq -r '.vulnerabilities[] | select(.severity=="CRITICAL") | .id' scan_results.json | \
      xargs -I {} ./create-security-issue.sh {}

    # High vulnerabilities (rate-limited processing)
    jq -r '.vulnerabilities[] | select(.severity=="HIGH") | .id' scan_results.json | \
      head -20 | xargs -I {} ./create-security-issue.sh {}

    # Medium/Low vulnerabilities (queued for later)
    jq -r '.vulnerabilities[] | select(.severity=="MEDIUM" or .severity=="LOW") | .id' \
      scan_results.json > /tmp/queued_vulnerabilities.txt
```

## Offline/Limited API Access Modes

### Development Environment Offline Setup

**Local CVE Database Cache:**
```bash
# Create data directory
mkdir -p ~/.keystone/data

# Download CVE feeds (run weekly)
#!/bin/bash
# Script: update-cve-cache.sh

echo "Downloading NVD CVE feeds..."
curl -o ~/.keystone/data/nvdcve-1.1-recent.json.gz \
  "https://nvd.nist.gov/feeds/json/cve/1.1/nvdcve-1.1-recent.json.gz"

gunzip ~/.keystone/data/nvdcve-1.1-recent.json.gz

echo "Downloading GitHub Security Advisories..."
curl -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/advisories?per_page=100" \
  > ~/.keystone/data/github-advisories.json

echo "Cache updated: $(date)"
```

**Configure Offline Mode:**
```bash
# Environment configuration for offline development
export KEYSTONE_OFFLINE_MODE=true
export KEYSTONE_CVE_CACHE_PATH="$HOME/.keystone/data/nvdcve-1.1-recent.json"
export KEYSTONE_ADVISORY_CACHE_PATH="$HOME/.keystone/data/github-advisories.json"
export KEYSTONE_CACHE_MAX_AGE_HOURS=168  # 1 week
```

### Scanner Database Management

**Trivy Database Setup:**
```bash
# Download Trivy vulnerability database
trivy image --download-db-only

# Verify database location
ls -la ~/.cache/trivy/db/

# Update database (run weekly)
trivy image --download-db-only --skip-update=false
```

**Grype Database Setup:**
```bash
# Download Grype vulnerability database
grype db update

# Verify database location
ls -la ~/.cache/grype/db/

# Check database info
grype db status
```

**Docker Configuration for Offline Mode:**
```dockerfile
# Dockerfile for offline vulnerability scanning
FROM anchore/grype:latest as grype-db
RUN grype db update

FROM aquasec/trivy:latest as trivy-db
RUN trivy image --download-db-only

FROM alpine:latest
COPY --from=grype-db /home/grype/.cache/grype /opt/grype/db
COPY --from=trivy-db /root/.cache/trivy /opt/trivy/db

ENV GRYPE_DB_CACHE_DIR=/opt/grype/db
ENV TRIVY_CACHE_DIR=/opt/trivy/db
```

## Troubleshooting Common Issues

### NVD API Issues

**Problem: `403 Forbidden` responses**
```bash
# Check API key validity
curl -I -H "apikey: $NVD_API_KEY" \
  "https://services.nvd.nist.gov/rest/json/cves/2.0?resultsPerPage=1"

# Expected: HTTP/2 200
# Error: HTTP/2 403 - Invalid or expired API key
```

**Solutions:**
1. **Verify API Key Format:** Should be UUID format (`xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`)
2. **Check Email for Updates:** NVD may send key rotation notifications
3. **Request New Key:** If key is expired or invalid

**Problem: Rate limit exceeded**
```bash
# Check rate limit status
curl -H "apikey: $NVD_API_KEY" -I \
  "https://services.nvd.nist.gov/rest/json/cves/2.0?resultsPerPage=1"

# Check for rate limit headers
# X-RateLimit-Remaining: 0
# Retry-After: 30
```

**Solutions:**
1. **Implement Exponential Backoff:** Wait time specified in `Retry-After` header
2. **Enable Caching:** Use local cache with 24-hour TTL for CVE data
3. **Reduce Request Frequency:** Batch requests when possible

### GitHub API Issues

**Problem: `422 Unprocessable Entity` when creating issues**
```json
{
  "message": "Validation Failed",
  "errors": [
    {
      "resource": "Issue",
      "code": "missing_field",
      "field": "title"
    }
  ]
}
```

**Solutions:**
1. **Verify Required Fields:** Ensure title and body are provided
2. **Check Repository Permissions:** Token needs `repo` scope
3. **Validate Issue Template:** Ensure compliance with repository issue templates

**Problem: Secondary rate limit (abuse detection)**
```json
{
  "message": "You have exceeded a secondary rate limit. Please wait a few minutes before you try again."
}
```

**Solutions:**
1. **Reduce Concurrent Requests:** Limit parallel API calls
2. **Add Random Delays:** Insert 1-5 second delays between requests
3. **Use GraphQL:** More efficient for bulk operations

### Sigstore/Rekor Issues

**Problem: Signature verification fails in offline mode**
```bash
cosign verify ghcr.io/owner/repo:latest
# Error: verifying certificate chain: certificate signed by unknown authority
```

**Solutions:**
1. **Update TUF Root Metadata:**
   ```bash
   cosign initialize
   ```

2. **Use Cached Trust Bundle:**
   ```bash
   cosign verify --offline \
     --certificate-chain=cert-chain.pem \
     --signature=signature.sig \
     ghcr.io/owner/repo:latest
   ```

**Problem: `OIDC token expired` during signing**
```bash
cosign sign ghcr.io/owner/repo:latest
# Error: signing ghcr.io/owner/repo:latest: getting credentials: oauth2: token expired
```

**Solutions:**
1. **Check GitHub Actions Permissions:**
   ```yaml
   permissions:
     id-token: write  # Required for OIDC
     contents: read
   ```

2. **Verify Workflow Context:** Ensure signing happens in same job as checkout
3. **Reduce Time Between Steps:** Minimize delay between checkout and signing

## Integration Testing

### Automated Test Suite

**Test External Service Connectivity:**
```bash
#!/bin/bash
# Script: test-external-services.sh

echo "Testing external service connectivity..."

# Test NVD API
if curl -s -f -H "apikey: $NVD_API_KEY" \
  "https://services.nvd.nist.gov/rest/json/cves/2.0?resultsPerPage=1" > /dev/null; then
  echo "✅ NVD API: Connected"
else
  echo "❌ NVD API: Failed"
  exit 1
fi

# Test GitHub API
if curl -s -f -H "Authorization: token $GITHUB_TOKEN" \
  "https://api.github.com/rate_limit" > /dev/null; then
  echo "✅ GitHub API: Connected"
else
  echo "❌ GitHub API: Failed"
  exit 1
fi

# Test Sigstore connectivity
if curl -s -f "https://fulcio.sigstore.dev/api/v2/configuration" > /dev/null; then
  echo "✅ Sigstore: Connected"
else
  echo "❌ Sigstore: Failed"
  exit 1
fi

echo "All external services available"
```

**Test Rate Limiting Behavior:**
```bash
#!/bin/bash
# Script: test-rate-limiting.sh

# Simulate high request volume
for i in {1..10}; do
  REMAINING=$(curl -s -H "Authorization: token $GITHUB_TOKEN" \
    "https://api.github.com/rate_limit" | jq -r '.resources.core.remaining')

  echo "Request $i: $REMAINING remaining"

  if [ "$REMAINING" -lt 4990 ]; then
    echo "✅ Rate limiting working: Consumed requests detected"
    break
  fi

  # Make a test request
  curl -s -H "Authorization: token $GITHUB_TOKEN" \
    "https://api.github.com/user" > /dev/null
done
```

### GitHub Actions Integration Test

```yaml
# .github/workflows/test-external-services.yml
name: Test External Services Integration

on:
  workflow_dispatch:
  schedule:
    - cron: '0 8 * * 1'  # Weekly on Monday

jobs:
  test-external-services:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
      contents: read

    steps:
      - uses: actions/checkout@v4

      - name: Test NVD API Access
        env:
          NVD_API_KEY: ${{ secrets.NVD_API_KEY }}
        run: |
          ./scripts/test-nvd-connectivity.sh

      - name: Test GitHub API Rate Limits
        env:
          GITHUB_TOKEN: ${{ github.token }}
        run: |
          ./scripts/test-github-rate-limits.sh

      - name: Test Sigstore Integration
        run: |
          cosign version
          cosign initialize
          echo "Sigstore integration ready"

      - name: Test Offline Mode
        run: |
          KEYSTONE_OFFLINE_MODE=true ./scripts/test-vulnerability-scanning.sh
```

## Security Best Practices

### API Key Management

**Secure Storage:**
- **Never commit API keys** to version control
- **Use GitHub Secrets** for repository-level access
- **Implement key rotation** quarterly or when team members leave
- **Monitor key usage** for anomalous activity

**Key Rotation Process:**
1. **Generate new API key** from service provider
2. **Update GitHub Secrets** with new key
3. **Test with new key** in development environment
4. **Deploy to production** during maintenance window
5. **Revoke old key** after successful deployment

### Network Security

**TLS/HTTPS Enforcement:**
```bash
# Verify TLS certificate for critical services
openssl s_client -connect fulcio.sigstore.dev:443 -verify_return_error

# Expected: Verify return code: 0 (ok)
```

**Certificate Pinning for Critical Services:**
```bash
# Pin Sigstore certificate (example)
EXPECTED_CERT_HASH="sha256:abcd1234..."
ACTUAL_CERT_HASH=$(openssl s_client -connect fulcio.sigstore.dev:443 2>/dev/null | \
  openssl x509 -fingerprint -noout -sha256 | cut -d'=' -f2)

if [ "$ACTUAL_CERT_HASH" != "$EXPECTED_CERT_HASH" ]; then
  echo "Certificate mismatch detected!"
  exit 1
fi
```

### Audit and Logging

**API Call Logging:**
```bash
# Log all external API calls
export KEYSTONE_LOG_LEVEL=debug
export KEYSTONE_API_AUDIT_LOG="$HOME/.keystone/api-audit.log"

# Log format: timestamp, service, endpoint, status, duration
# Example: 2025-09-19T10:30:00Z,github,/rate_limit,200,150ms
```

**Security Event Monitoring:**
```yaml
# GitHub Actions logging
- name: Log Security Events
  run: |
    echo "::notice::External API call to NVD at $(date)"
    echo "::warning::Rate limit at 80% capacity"
    echo "::error::API authentication failed"
```

**Retention Policy:**
- **API Audit Logs:** 90 days minimum
- **Security Event Logs:** 1 year minimum
- **Error Logs:** 30 days minimum
- **Performance Metrics:** 6 months for trend analysis

## Support and Maintenance

### Regular Maintenance Tasks

**Weekly Tasks:**
- Update local CVE database cache
- Review API usage patterns and rate limit consumption
- Check for API key rotation notices
- Update vulnerability scanner databases

**Monthly Tasks:**
- Review and update external service configurations
- Analyze API performance metrics and optimize caching
- Test offline mode functionality
- Review security audit logs for anomalies

**Quarterly Tasks:**
- Rotate API keys and access tokens
- Review and update rate limiting thresholds
- Update integration test coverage
- Document any configuration changes

### Getting Help

**GitHub Repository Issues:**
- [Create an issue](https://github.com/salman-frs/keystone/issues/new) for bugs or feature requests
- Tag with `external-services` label for API-related issues
- Include relevant logs and configuration details

**External Service Support:**
- **NVD Support:** https://nvd.nist.gov/general/contact-form
- **GitHub Support:** https://support.github.com/
- **Sigstore Community:** https://github.com/sigstore/community
