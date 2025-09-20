# Keystone Security Platform

A comprehensive DevSecOps automation platform that integrates vulnerability scanning, policy enforcement, and cryptographic attestation into GitHub workflows.

## What Keystone Delivers

Keystone automates security pipeline operations to provide:

- **Multi-Scanner Vulnerability Detection**: Correlates findings across Trivy and Grype scanners with automated result processing
- **SBOM Generation and Artifact Management**: Creates software bill of materials with secure storage in GitHub Container Registry
- **Automated Security Workflows**: GitHub Actions integration for continuous security scanning and compliance checking
- **Portfolio-Ready Implementation**: Demonstrates advanced DevSecOps capabilities for technical evaluation

## Project Goals

Keystone demonstrates enterprise-grade security automation by:

1. **Streamlining Security Operations**: Automated vulnerability detection with minimal manual intervention
2. **Ensuring Supply Chain Security**: Comprehensive SBOM generation and cryptographic attestation capabilities
3. **Enabling Policy-Driven Security**: Framework for automated security policy enforcement
4. **Providing Developer-Friendly Integration**: Zero-configuration security scanning in CI/CD pipelines

## Current Implementation Status

### Completed 

- **Repository Foundation**: Monorepo structure with GitHub Actions CI/CD pipeline
- **Multi-Scanner Integration**: Trivy and Grype vulnerability detection with automated correlation
- **SBOM Workflow**: Syft integration for software bill of materials generation and artifact storage
- **Demo Application**: Vulnerable Go application with documented CVEs for testing workflows

### In Progress 

- **Documentation Suite**: Comprehensive setup guides and technical documentation

## Quick Start for Technical Evaluation


### Immediate Evaluation Steps

```bash
# Clone and setup
git clone https://github.com/salman-frs/keystone.git
cd keystone

# Follow detailed setup
open user-docs/setup.md

# Run vulnerability detection demo
./scripts/setup/test-workflow.sh

# View demo application scanning
open examples/vulnerable-app/README.md
```

## Project Structure Overview

```
keystone/
├── .github/workflows/     # GitHub Actions security pipeline
├── apps/                  # Application packages (dashboard, API)
├── packages/              # Shared libraries and components
├── examples/              # Vulnerable demo application
├── scripts/setup/         # Testing and validation scripts
├── user-docs/            # Public documentation
└── infrastructure/       # Security policies and configurations
```

### Implemented Security Components

- **Security Pipeline** (`.github/workflows/security-pipeline.yaml`): Automated vulnerability scanning workflow
- **Multi-Scanner Integration**: Trivy and Grype vulnerability detection with JSON output correlation
- **SBOM Generation**: Syft integration producing CycloneDX and SPDX formats
- **Vulnerable Test Application** (`examples/vulnerable-app/`): Go application with documented security vulnerabilities
- **Validation Scripts** (`scripts/setup/`): Automated testing for scanner integration and SBOM workflows

## Architecture Overview

### Technology Stack (Implemented)

- **CI/CD Pipeline**: GitHub Actions with security workflow automation
- **Container Runtime**: Docker with Colima for macOS development
- **Vulnerability Scanners**: Trivy (container/filesystem) and Grype (package analysis)
- **SBOM Generation**: Syft for software bill of materials creation
- **Artifact Storage**: GitHub Container Registry for secure SBOM storage
- **Application Framework**: Go 1.21+ for demo vulnerable application

### Planned Architecture Components

- **Frontend**: React 18.2+ with TypeScript 5.3+ and Tailwind CSS
- **Backend**: Go 1.25+ microservices with Gin HTTP framework
- **Database**: SQLite for embedded CI/CD environments
- **Authentication**: GitHub OAuth with OIDC integration
- **Policy Engine**: Open Policy Agent (OPA) for security policy enforcement

## Implemented Security Features

### Automated Vulnerability Detection
- **Multi-Scanner Integration**: Trivy and Grype scanners with JSON output processing
- **Correlation Capability**: Framework for comparing vulnerability findings across scanners
- **GitHub Actions Integration**: Automated scanning triggered by code changes and releases

### Software Bill of Materials (SBOM)
- **Comprehensive Generation**: Syft integration producing CycloneDX and SPDX formats
- **Secure Storage**: GitHub Container Registry artifact management
- **Validation Workflows**: Automated SBOM format verification and content validation

### Demonstration Infrastructure
- **Vulnerable Application**: Go-based demo with documented CVEs for realistic testing
- **Testing Scripts**: Automated validation of scanner integration and SBOM workflows
- **CI/CD Pipeline**: Production-ready GitHub Actions workflow for security automation

### Developer Experience
- **Zero-Configuration Setup**: Automated scanner installation and configuration

## Complete Documentation

### Setup and Getting Started
- **[Setup Guide](user-docs/setup.md)**: Complete development environment setup
- **[Getting Started](user-docs/getting-started.md)**: Prerequisite installation and basic workflows
- **[Demo Guide](user-docs/demo.md)**: Step-by-step vulnerability detection demonstration
- **[Architecture Overview](user-docs/architecture.md)**: System design 
- **[Troubleshooting](user-docs/troubleshooting.md)**: Common issues and diagnostic procedures

### Technical Implementation
- **[API Documentation](user-docs/api/)**: Security pipeline API specifications
- **[Deployment Guide](user-docs/deployment/)**: GitHub Actions workflow configuration
- **[Security Model](user-docs/security/)**: Threat model and compliance framework
- **[Examples](user-docs/examples/)**: Usage tutorials and implementation patterns

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.