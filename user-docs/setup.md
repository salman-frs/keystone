# Setup Guide

This guide walks you through setting up the Keystone Security Platform for local development.

## Prerequisites

### Required Software

- **Go 1.25+**: High-performance backend services
- **Node.js 22+**: Frontend dashboard development  
- **Docker**: Container runtime (Colima recommended for macOS)
- **GitHub CLI**: Deployment and authentication

### macOS Setup with Colima

```bash
# Install Colima for Docker runtime
brew install colima

# Start Colima
colima start

# Verify Docker is working
docker --version
```

### Verification Commands

```bash
# Check Go version
go version

# Check Node.js version  
node --version

# Check Docker connectivity
docker ps

# Check GitHub CLI
gh --version
```

## Local Development Environment

### 1. Clone and Setup

```bash
git clone https://github.com/salman-frs/keystone.git
cd keystone

# Verify directory structure
ls -la
```

### 2. Environment Configuration

```bash
# Copy environment template
cp .env.example .env.local

# Edit configuration
vim .env.local
```

### 3. Start Development Services

```bash
# Start all services
docker-compose up -d

# Check service status
docker-compose ps

# View logs
docker-compose logs -f
```

### 4. Verify Installation

- Dashboard: http://localhost:3000
- API Health: http://localhost:8080/health

## Development Workflow

### Backend Development (Go)

```bash
# Navigate to API directory
cd apps/api

# Install dependencies
go mod tidy

# Run tests
go test ./...

# Build services
go build ./cmd/...
```

### Frontend Development (React)

```bash
# Navigate to dashboard directory  
cd apps/dashboard

# Install dependencies
npm install

# Start development server
npm run dev

# Run tests
npm test
```

## Database Setup

SQLite database files are created automatically in the `data/` directory:

```bash
# Database location
./data/keystone.db

# View database schema (after first run)
sqlite3 ./data/keystone.db ".schema"
```

## Troubleshooting

### Common Issues

**Docker connection errors:**
```bash
# Restart Colima
colima restart

# Check Docker daemon
docker system info
```

**Port conflicts:**
```bash
# Check port usage
lsof -i :3000
lsof -i :8080

# Update docker-compose.yml ports if needed
```

**Permission issues:**
```bash
# Fix directory permissions
chmod -R 755 ./data ./logs
```

## Next Steps

Once setup is complete:

1. Read the [Getting Started Guide](getting-started.md)
2. Explore the [API Documentation](api/)
3. Review [Security Guidelines](security/)

## Development Tools

### Recommended VSCode Extensions

- Go extension
- TypeScript and JavaScript Language Features
- Tailwind CSS IntelliSense
- Docker extension

### Useful Commands

```bash
# Format Go code
go fmt ./...

# Format TypeScript/React code
npm run format

# Run full test suite
./scripts/test.sh

# Build for production
./scripts/build.sh
```