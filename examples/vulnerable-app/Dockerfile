# Build stage - using Go 1.21+ for slog support
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache \
    git \
    ca-certificates \
    openssl \
    && rm -rf /var/cache/apk/*

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# Runtime stage - using older Ubuntu with vulnerable packages
FROM ubuntu:18.04

# Install packages (older Ubuntu contains vulnerable packages by default)
# Using older base image avoids version conflicts while maintaining vulnerabilities
RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    openssl \
    libc6 \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -r -s /bin/false appuser

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/main .

# Add metadata for SBOM generation
LABEL \
    org.opencontainers.image.title="Vulnerable Demo Application" \
    org.opencontainers.image.description="Demo application with intentional vulnerabilities for security testing" \
    org.opencontainers.image.version="1.0.0" \
    org.opencontainers.image.vendor="Keystone Security Platform" \
    org.opencontainers.image.source="https://github.com/salman-frs/keystone"

# Switch to non-root user
USER appuser

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

EXPOSE 8080

CMD ["./main"]