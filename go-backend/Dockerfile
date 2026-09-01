# =============================================================================
# Go Backend - Multi-stage Dockerfile
# =============================================================================

# -----------------------------------------------------------------------------
# Development Stage
# -----------------------------------------------------------------------------
FROM golang:1.23-alpine AS development

ENV GOTOOLCHAIN=auto

WORKDIR /app

# Install development tools
RUN apk add --no-cache git curl

# Install air for hot reload
RUN go install github.com/cosmtrek/air@v1.49.0

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Expose port
EXPOSE 8080

# Run with hot reload
CMD ["air", "-c", ".air.toml"]

# -----------------------------------------------------------------------------
# Builder Stage
# -----------------------------------------------------------------------------
FROM golang:1.23-alpine AS builder

ENV GOTOOLCHAIN=auto

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Ensure dependencies are synced
RUN go mod tidy

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /app/server \
    ./cmd/server

# -----------------------------------------------------------------------------
# Production Stage
# -----------------------------------------------------------------------------
FROM gcr.io/distroless/static:nonroot AS production

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server /app/server

# Copy CA certificates for TLS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Use non-root user
USER nonroot:nonroot

# Expose port
EXPOSE 8080

# NOTE: Docker HEALTHCHECK removed - using distroless image without shell.
# Health checks are handled by container orchestrator (K8s/ECS) via ALB
# probes to GET /health endpoint.

# Run the binary
ENTRYPOINT ["/app/server"]
