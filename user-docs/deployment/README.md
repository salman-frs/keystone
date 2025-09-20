# Deployment Guide

This directory contains deployment documentation for the Keystone Security Platform.

## Current Deployment

Keystone currently implements GitHub Actions-based deployment for security scanning workflows.

## GitHub Actions Deployment

### Automated Security Pipeline

The primary deployment model uses GitHub Actions for:

- **Vulnerability Scanning**: Automated Trivy and Grype scanning on code changes
- **SBOM Generation**: Software bill of materials creation with Syft
- **Artifact Storage**: Secure storage in GitHub Container Registry
- **Workflow Orchestration**: Event-driven security pipeline execution

### Workflow Configuration

```yaml
# .github/workflows/security-pipeline.yaml
name: Security Pipeline
on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]
  release:
    types: [ created ]
```

## Local Development Deployment

### Setup and Configuration

For local development deployment:

1. **Prerequisites**: Follow [Setup Guide](../setup.md)
2. **Environment**: Configure with [Getting Started](../getting-started.md)
3. **Testing**: Validate with [Demo Guide](../demo.md)

### Local Testing with Act

```bash
# Install act for local GitHub Actions testing
brew install act

# Test workflow locally
act push --dry-run
act push
```

## Planned Deployment Models

### Container Deployment (Future)

When the full application is implemented:

```bash
# Docker Compose deployment
docker-compose up -d

# Kubernetes deployment
kubectl apply -f k8s/
```

### Cloud Deployment (Future)

Planned cloud deployment options:

- **GitHub Codespaces**: Development environment deployment
- **AWS/GCP/Azure**: Container orchestration platforms
- **Serverless**: Function-based deployment for scanning services

## Deployment Documentation (Future)

This directory will contain:

- `github-actions.md`: GitHub Actions deployment guide
- `local-development.md`: Local environment setup
- `production.md`: Production deployment procedures
- `docker.md`: Container deployment configuration
- `kubernetes.md`: Kubernetes orchestration setup

## Current Deployment Verification

Verify successful deployment with:

```bash
# Validate GitHub Actions workflow
gh workflow list
gh workflow view security-pipeline

# Test local environment
./scripts/setup/validate-scanners.sh
./scripts/setup/test-workflow.sh

# Check artifact storage
gh run list --workflow=security-pipeline.yaml
```

For detailed deployment procedures, see:
- [Architecture Overview](../architecture.md)
- [Setup Guide](../setup.md)
- [Troubleshooting Guide](../troubleshooting.md)