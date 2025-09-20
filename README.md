# Keystone Security Platform

A comprehensive DevSecOps automation platform that integrates vulnerability scanning, policy enforcement, and cryptographic attestation into GitHub workflows.

## Overview

Keystone transforms security scanning from a reactive checklist item into a proactive, automated system that:

- **Correlates** vulnerability data across multiple scanners (Trivy, Grype, Snyk)
- **Enforces** security policies through Open Policy Agent (OPA) integration
- **Attests** to security compliance using Sigstore cryptographic signatures
- **Delivers** real-time security insights through an intuitive dashboard

## Architecture

### Core Components

- **Dashboard** (`apps/dashboard`): React-based security visualization interface
- **API Services** (`apps/api`): Go microservices for vulnerability correlation and policy evaluation
- **Shared Libraries** (`packages/shared`): Common types and utilities across frontend/backend
- **Security Components** (`packages/security-components`): Reusable React security UI components

### Technology Stack

- **Frontend**: React 18.2+ with TypeScript 5.3+ and Tailwind CSS
- **Backend**: Go 1.25+ with Gin HTTP framework
- **Database**: SQLite for embedded CI/CD environments
- **Authentication**: GitHub OAuth with OIDC for keyless signing
- **Infrastructure**: GitHub Actions for CI/CD orchestration

## Quick Start

### Prerequisites

- Go 1.25+
- Node.js 22+
- Docker with Colima (macOS) or Docker Desktop
- GitHub CLI (for deployment)

### Local Development

```bash
# Clone the repository
git clone https://github.com/salman-frs/keystone.git
cd keystone

# Start development environment
docker-compose up -d

# Access the dashboard
open http://localhost:3000
```

### Production Deployment

```bash
# Deploy to GitHub Actions
./scripts/deploy.sh

# Verify deployment
./scripts/verify.sh
```

## Key Features

### Security Scanning Integration
- Multi-scanner vulnerability correlation
- SBOM generation and attestation
- Real-time scan result processing

### Policy Enforcement
- Custom OPA/Rego policy definitions
- Automated compliance checking
- Risk-based approval workflows

### Cryptographic Attestation
- Sigstore keyless signing integration
- Supply chain security verification
- Tamper-evident security records

### Developer Experience
- Zero-configuration CI/CD integration
- Interactive security dashboard
- Automated remediation suggestions

## Documentation

- [Setup Guide](user-docs/setup.md)
- [Getting Started](user-docs/getting-started.md)
- [API Documentation](user-docs/api/)
- [Deployment Guide](user-docs/deployment/)
- [Security Model](user-docs/security/)

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- 📖 [Documentation](user-docs/)
- 🐛 [Issue Tracker](../../issues)
- 💬 [Discussions](../../discussions)