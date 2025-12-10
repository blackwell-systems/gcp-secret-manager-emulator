# GCP Secret Manager Emulator

[![Blackwell Systems](https://raw.githubusercontent.com/blackwell-systems/blackwell-docs-theme/main/badge-trademark.svg)](https://github.com/blackwell-systems)
[![Version](https://img.shields.io/github/v/release/blackwell-systems/gcp-secret-manager-emulator)](https://github.com/blackwell-systems/gcp-secret-manager-emulator/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/blackwell-systems/gcp-secret-manager-emulator.svg)](https://pkg.go.dev/github.com/blackwell-systems/gcp-secret-manager-emulator)
[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Sponsor](https://img.shields.io/badge/Sponsor-Buy%20Me%20a%20Coffee-yellow?logo=buy-me-a-coffee&logoColor=white)](https://buymeacoffee.com/blackwellsystems)

> Lightweight gRPC emulator for Google Cloud Secret Manager API

A standalone gRPC server that implements the Google Cloud Secret Manager API for local testing and CI/CD environments. No GCP credentials or internet connectivity required.

## Features

- **Full gRPC API Implementation** - Complete Secret Manager v1 API
- **No GCP Credentials** - Works entirely offline without authentication
- **Fast & Lightweight** - In-memory storage, starts in milliseconds
- **Docker Support** - Pre-built container for easy deployment
- **Thread-Safe** - Concurrent access with proper synchronization
- **Real SDK Compatible** - Works with official `cloud.google.com/go/secretmanager` client
- **High Test Coverage** - 87% coverage with comprehensive integration tests

## Supported Operations

### Secrets
- `CreateSecret` - Create new secrets with labels
- `GetSecret` - Retrieve secret metadata
- `ListSecrets` - List all secrets with pagination
- `DeleteSecret` - Remove secrets

### Secret Versions
- `AddSecretVersion` - Add new version with payload
- `AccessSecretVersion` - Retrieve version payload
- `ListSecretVersions` - List all versions for a secret
- `DestroySecretVersion` - Permanently destroy a version

## Quick Start

### Install

```bash
go install github.com/blackwell-systems/gcp-secret-manager-emulator/cmd/server@latest
```

### Run Server

```bash
# Start on default port 9090
server

# Custom port
server --port 8080

# With debug logging
server --log-level debug
```

### Use with GCP SDK

```go
package main

import (
    "context"
    "fmt"

    secretmanager "cloud.google.com/go/secretmanager/apiv1"
    "google.golang.org/api/option"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    ctx := context.Background()

    // Connect to emulator instead of real GCP
    conn, _ := grpc.NewClient(
        "localhost:9090",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )

    client, _ := secretmanager.NewClient(ctx, option.WithGRPCConn(conn))
    defer client.Close()

    // Use client normally - API is identical to real GCP
    // ...
}
```

## Docker

```bash
# Build
docker build -t gcp-secret-manager-emulator .

# Run
docker run -p 9090:9090 gcp-secret-manager-emulator

# In CI/CD
services:
  gcp-emulator:
    image: gcp-secret-manager-emulator:latest
    ports:
      - "9090:9090"
```

## Use Cases

- **Local Development** - Test GCP Secret Manager integration without cloud access
- **CI/CD Pipelines** - Fast integration tests without GCP credentials
- **Unit Testing** - Deterministic test environment
- **Demos & Prototyping** - Showcase GCP integrations offline
- **Cost Reduction** - Avoid GCP API charges during development

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GCP_MOCK_PORT` | `9090` | Port to listen on |
| `GCP_MOCK_LOG_LEVEL` | `info` | Log level: debug, info, warn, error |

### Command Line Flags

```bash
server --help

Flags:
  --port int           Port to listen on (default 9090)
  --log-level string   Log level (default "info")
```

## Architecture

See [docs/DESIGN.md](docs/DESIGN.md) for implementation details, API coverage, and design decisions.

## Testing

```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# With race detector
go test -race ./...
```

## Differences from Real GCP

**Intentional Simplifications:**
- No authentication/authorization (all requests succeed)
- No IAM permissions or resource policies
- No encryption at rest (in-memory storage)
- No replication or regional constraints
- Simplified error responses (no retry-after headers)

**Perfect for:**
- Development and testing workflows
- CI/CD environments
- Local integration testing

**Not for:**
- Production use
- Security testing
- Performance benchmarking

## Project Status

Extracted from [vaultmux](https://github.com/blackwell-systems/vaultmux) where it powers GCP backend integration tests. Used in production CI pipelines.

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.
