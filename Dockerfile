# Dockerfile for GCP Secret Manager Mock Server
# Multi-stage build for minimal final image size

# Build stage
FROM golang:alpine AS builder

WORKDIR /build

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the mock server binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gcp-secret-manager-mock ./cmd/gcp-secret-manager-mock

# Final stage - minimal image
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/gcp-secret-manager-mock .

# Expose gRPC port
EXPOSE 9090

# Run as non-root user for security
RUN addgroup -g 1000 gcpmock && \
    adduser -D -u 1000 -G gcpmock gcpmock && \
    chown -R gcpmock:gcpmock /app

USER gcpmock

# Set default environment variables
ENV GCP_MOCK_PORT=9090 \
    GCP_MOCK_LOG_LEVEL=info

# Health check (requires grpc_health_probe in CI, not in this minimal image)
# HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
#   CMD grpc_health_probe -addr=:9090 || exit 1

ENTRYPOINT ["/app/gcp-secret-manager-mock"]
