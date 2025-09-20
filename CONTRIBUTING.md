# Contributing to Keystone Security Platform

We welcome contributions to the Keystone Security Platform. This document provides guidelines for contributing to the project.

## Code of Conduct

This project adheres to a code of conduct. By participating, you are expected to uphold this code.

## How to Contribute

### Reporting Bugs

1. Check if the bug has already been reported in the [Issues](../../issues) section
2. Create a new issue with a clear title and description
3. Include steps to reproduce the bug
4. Provide system information (OS, Go version, Node.js version)

### Suggesting Features

1. Check existing [Issues](../../issues) for similar feature requests
2. Create a new issue with the "enhancement" label
3. Clearly describe the feature and its benefits
4. Include examples of how the feature would be used

### Security Vulnerabilities

**Do not report security vulnerabilities through public GitHub issues.**

Instead, please report them by email to: security@keystone-platform.dev

Include the following information:
- Type of vulnerability
- Full paths of source files related to the vulnerability
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if available)

We will acknowledge receipt of your vulnerability report and send you a more detailed response indicating next steps.

## Development Process

### Fork and Clone

```bash
# Fork the repository on GitHub
# Clone your fork locally
git clone https://github.com/salman-frs/keystone.git
cd keystone

# Add upstream remote
git remote add upstream https://github.com/salman-frs/keystone.git
```

### Set Up Development Environment

Follow the [Setup Guide](user-docs/setup.md) to configure your local development environment.

### Create a Branch

```bash
# Create a feature branch
git checkout -b feature/your-feature-name

# Or a bug fix branch
git checkout -b fix/issue-number
```

### Make Changes

1. Write code following the project's coding standards
2. Add tests for new functionality
3. Update documentation as needed
4. Ensure all tests pass

### Coding Standards

#### Go Code
- Follow standard Go formatting with `go fmt`
- Use `golint` and `go vet` for code quality
- Write meaningful variable and function names
- Include appropriate comments for exported functions

#### TypeScript/React Code
- Use TypeScript for all new code
- Follow existing component patterns
- Use Tailwind CSS for styling
- Write meaningful component and prop names

#### General Guidelines
- Keep functions small and focused
- Write comprehensive tests
- Follow security best practices
- Document public APIs

### Testing

```bash
# Run backend tests
cd apps/api
go test ./...

# Run frontend tests
cd apps/dashboard
npm test

# Run integration tests
./scripts/test.sh
```

### Commit Messages

Use clear and meaningful commit messages:

```bash
# Good examples
git commit -m "Add vulnerability correlation service"
git commit -m "Fix authentication middleware bug"
git commit -m "Update dashboard component styles"

# Bad examples
git commit -m "Fix bug"
git commit -m "Update code"
git commit -m "WIP"
```

### Submit Pull Request

1. Push your branch to your fork
2. Create a Pull Request from your branch to the main repository
3. Fill out the Pull Request template
4. Link any related issues

### Pull Request Requirements

- [ ] All tests pass
- [ ] Code follows project standards
- [ ] Documentation is updated
- [ ] Security implications are considered
- [ ] Breaking changes are documented

## Project Structure

```
keystone/
├── apps/                    # Application packages
│   ├── dashboard/          # React frontend
│   └── api/               # Go backend services
├── packages/              # Shared packages
│   ├── shared/           # Common utilities
│   └── security-components/ # React components
├── infrastructure/       # Infrastructure as Code
├── scripts/             # Build and deployment scripts
├── examples/           # Demo applications
└── user-docs/         # Public documentation
```

## Review Process

1. **Automated Checks**: All Pull Requests run through automated testing
2. **Code Review**: At least one maintainer reviews the code
3. **Security Review**: Security-related changes receive additional review
4. **Documentation Review**: Documentation changes are reviewed for clarity

## Release Process

1. Features are developed in feature branches
2. Pull Requests are merged to main branch
3. Releases are tagged and published from main branch
4. Release notes are generated automatically

## Community

- **Discussions**: Use [GitHub Discussions](../../discussions) for questions
- **Issues**: Use [GitHub Issues](../../issues) for bugs and feature requests
- **Documentation**: Improve documentation in [user-docs/](user-docs/)

## Recognition

Contributors are recognized in:
- Release notes
- Project documentation
- GitHub contributor graph

Thank you for contributing to Keystone Security Platform!