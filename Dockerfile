# Dockerfile for GCP Secret Manager Emulator
# Multi-stage build with variant selection via build args
#
# Build variants:
#   docker build --build-arg VARIANT=grpc -t emulator:grpc .      # gRPC only (default)
#   docker build --build-arg VARIANT=rest -t emulator:rest .      # REST only
#   docker build --build-arg VARIANT=dual -t emulator:dual .      # Both protocols

# Build stage
FROM golang:alpine AS builder

ARG VARIANT=grpc

WORKDIR /build

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the appropriate server binary based on variant
RUN case "${VARIANT}" in \
    grpc) \
        echo "Building gRPC-only server..." && \
        CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server \
        ;; \
    rest) \
        echo "Building REST-only server..." && \
        CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server-rest \
        ;; \
    dual) \
        echo "Building dual-protocol server..." && \
        CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o server ./cmd/server-dual \
        ;; \
    *) \
        echo "Invalid VARIANT: ${VARIANT}. Must be grpc, rest, or dual" && exit 1 \
        ;; \
    esac

# Final stage - minimal image
FROM alpine:latest

ARG VARIANT=grpc

# Install ca-certificates with --no-scripts to avoid trigger issues in ARM64 QEMU builds
RUN apk --no-cache add --no-scripts ca-certificates && \
    update-ca-certificates || true

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/server .

# Expose ports based on variant
# gRPC: 9090, REST: 8080 (internal gRPC) + 8080 (HTTP), Dual: 9090 + 8080
EXPOSE 9090
EXPOSE 8080

# Run as non-root user for security
RUN addgroup -g 1000 gcpmock && \
    adduser -D -u 1000 -G gcpmock gcpmock && \
    chown -R gcpmock:gcpmock /app

USER gcpmock

# Set default environment variables based on variant
ENV GCP_MOCK_LOG_LEVEL=info

# Label the image with build variant
LABEL org.opencontainers.image.title="GCP Secret Manager Emulator (${VARIANT})"
LABEL org.opencontainers.image.description="Local implementation of GCP Secret Manager API"
LABEL org.opencontainers.image.variant="${VARIANT}"

ENTRYPOINT ["/app/server"]
