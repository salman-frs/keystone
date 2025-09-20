# Multi-stage Dockerfile for Keystone Security Platform
FROM golang:1.25-alpine AS go-builder

WORKDIR /app
RUN apk add --no-cache git ca-certificates

# Go services will be built here in future stories
COPY apps/api/go.mod* apps/api/go.sum* ./apps/api/
COPY packages/shared/go.mod* packages/shared/go.sum* ./packages/shared/

FROM node:22-alpine AS node-builder

WORKDIR /app
RUN apk add --no-cache git

# Frontend dashboard will be built here in future stories
COPY apps/dashboard/package*.json ./apps/dashboard/
COPY packages/security-components/package*.json ./packages/security-components/
COPY packages/shared/package*.json ./packages/shared/

FROM alpine:3.19 AS runtime

RUN apk add --no-cache \
    ca-certificates \
    sqlite \
    curl

WORKDIR /app

# Create directories for future services
RUN mkdir -p /app/data /app/logs /app/config

# Health check endpoint
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

EXPOSE 8080

CMD ["echo", "Keystone Security Platform - Container ready for service deployment"]