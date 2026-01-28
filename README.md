# GCP Secret Manager Emulator

[![Blackwell Systems](https://raw.githubusercontent.com/blackwell-systems/blackwell-docs-theme/main/badge-trademark.svg)](https://github.com/blackwell-systems)
[![Go Reference](https://pkg.go.dev/badge/github.com/blackwell-systems/gcp-secret-manager-emulator.svg)](https://pkg.go.dev/github.com/blackwell-systems/gcp-secret-manager-emulator)
[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://go.dev/)
[![Test Status](https://github.com/blackwell-systems/gcp-secret-manager-emulator/workflows/CI/badge.svg)](https://github.com/blackwell-systems/gcp-secret-manager-emulator/actions)
[![Version](https://img.shields.io/github/v/release/blackwell-systems/gcp-secret-manager-emulator)](https://github.com/blackwell-systems/gcp-secret-manager-emulator/releases)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Sponsor](https://img.shields.io/badge/Sponsor-Buy%20Me%20a%20Coffee-yellow?logo=buy-me-a-coffee&logoColor=white)](https://buymeacoffee.com/blackwellsystems)

> **IAM-enforced Secret Manager emulator** — Test secrets AND permissions locally, fail like production would.

A production-grade Secret Manager implementation with optional **pre-flight IAM enforcement**. Unlike standard emulators that allow everything, this can deny unauthorized requests using real IAM policies.

**Dual protocol support**: Native gRPC + REST/HTTP. No GCP credentials required.

## Quick Start

```bash
# Install (dual protocol: gRPC + REST)
go install github.com/blackwell-systems/gcp-secret-manager-emulator/cmd/server-dual@latest

# Run
server-dual
# gRPC listening on :9090
# REST API listening on :8080

# Test with curl
curl http://localhost:8080/v1/projects/test-project/secrets

# Or use with official GCP SDK (point at localhost:9090)
```

**In production:** Used by enterprise teams for hermetic CI/CD testing

---

## Why This Emulator Is Different

Most Secret Manager emulators skip authorization. This one can **enforce real IAM policies** using the [IAM Emulator](https://github.com/blackwell-systems/gcp-iam-emulator) as a control plane.

| Approach | Example | When | Behavior |
|----------|---------|------|----------|
| Mock | Standard emulators | Never | Always allows |
| Observer | iamlive (AWS) | After | Records what you used |
| **Control Plane** | **Blackwell (this)** | **Before** | **Denies unauthorized** |

**Key insight:** Pre-flight enforcement catches permission bugs in development/CI, not production.

### Enforcement Modes

- **Off** (default) - No IAM checks, fast iteration
- **Permissive** - Enforce when IAM available, allow on connectivity errors (fail-open)
- **Strict** - Always enforce, deny on connectivity errors (fail-closed, CI-ready)

See the [category definition](https://github.com/blackwell-systems/gcp-emulator-auth/blob/master/CATEGORY.md) for the complete control plane architecture.

---

## Usage Modes

**Standalone** - Run independently for Secret Manager-only testing:
```bash
server-dual
# Single service, no IAM enforcement (mode=off)
```

**With IAM Enforcement** - Run standalone with IAM checks:
```bash
# Start IAM emulator first
cd ../gcp-iam-emulator && ./bin/server --config policy.yaml

# Start Secret Manager with enforcement
IAM_MODE=strict IAM_EMULATOR_HOST=localhost:8080 server-dual
# Now requires valid permissions for all operations
```

**Orchestrated Ecosystem** - Use with [GCP IAM Control Plane](https://github.com/blackwell-systems/gcp-iam-control-plane) for multi-service testing:
```bash
gcp-emulator start
# Secret Manager + KMS + IAM emulator
# Single policy file, unified authorization
```

**Choose standalone for simple workflows, IAM-enforced for production-like testing.**

---

## Features

### Core Functionality
- **Dual Protocol Support** - Native gRPC + REST/HTTP APIs (choose what fits your workflow)
- **SDK Compatible** - Drop-in replacement for official `cloud.google.com/go/secretmanager` (gRPC)
- **curl Friendly** - Full REST API with JSON, test from any language or terminal
- **Complete API** - 11 of 12 methods implemented (92% API coverage)
- **High Test Coverage** - 90.8% coverage with comprehensive integration tests

### IAM Enforcement (Optional)
- **Pre-Flight Authorization** - Checks permissions before data access
- **Real Policy Evaluation** - Uses IAM Emulator control plane for decisions
- **Three Modes** - Off (default), Permissive (fail-open), Strict (fail-closed)
- **Production Semantics** - Same permission names as real GCP (`secretmanager.secrets.get`)
- **Fail Like Production** - Catch permission bugs in CI, not production

### Operations
- **No GCP Credentials** - Works entirely offline without authentication
- **Fast & Lightweight** - In-memory storage, starts in milliseconds
- **Docker Support** - Pre-built containers (gRPC-only, REST-only, or dual)
- **Thread-Safe** - Concurrent access with proper synchronization

## Supported Operations

### Secrets
- `CreateSecret` - Create new secrets with labels
- `GetSecret` - Retrieve secret metadata
- `UpdateSecret` - Modify secret metadata (labels, annotations)
- `ListSecrets` - List all secrets with pagination
- `DeleteSecret` - Remove secrets

### Secret Versions
- `AddSecretVersion` - Add new version with payload
- `GetSecretVersion` - Retrieve version metadata
- `AccessSecretVersion` - Retrieve version payload (respects version state)
- `ListSecretVersions` - List all versions with pagination and filtering
- `EnableSecretVersion` - Enable a disabled version
- `DisableSecretVersion` - Disable a version (prevents access)
- `DestroySecretVersion` - Permanently destroy a version (irreversible)

## Quick Start

### Choose Your Protocol

**Three server variants available:**

| Variant | Protocols | Use Case | Install Command |
|---------|-----------|----------|-----------------|
| `server` | gRPC only | SDK users, fastest startup | `go install .../cmd/server@latest` |
| `server-rest` | REST/HTTP | curl, scripts, any language | `go install .../cmd/server-rest@latest` |
| `server-dual` | Both gRPC + REST | Maximum flexibility | `go install .../cmd/server-dual@latest` |

### Install

```bash
# gRPC only (recommended for SDK users)
go install github.com/blackwell-systems/gcp-secret-manager-emulator/cmd/server@latest

# REST API only
go install github.com/blackwell-systems/gcp-secret-manager-emulator/cmd/server-rest@latest

# Both protocols
go install github.com/blackwell-systems/gcp-secret-manager-emulator/cmd/server-dual@latest
```

### Run Server

**gRPC server:**
```bash
# Start on default port 9090
server

# Custom port
server --port 8080
```

**REST server:**
```bash
# Start on default ports (gRPC: 9090, HTTP: 8080)
server-rest

# Custom ports
server-rest --grpc-port 9090 --http-port 8080
```

**Dual protocol server:**
```bash
# Start both protocols (gRPC: 9090, HTTP: 8080)
server-dual

# Custom ports
server-dual --grpc-port 9090 --http-port 8080
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

### Use with REST API

**Start REST server:**
```bash
server-rest
# HTTP gateway listening at :8080
# Example: curl http://localhost:8080/v1/projects/test-project/secrets
```

**Create a secret:**
```bash
curl -X POST "http://localhost:8080/v1/projects/my-project/secrets?secretId=my-secret" \
  -H "Content-Type: application/json" \
  -d '{"replication":{"automatic":{}}}'
```

**Add a secret version:**
```bash
curl -X POST "http://localhost:8080/v1/projects/my-project/secrets/my-secret:addVersion" \
  -H "Content-Type: application/json" \
  -d '{"payload":{"data":"'$(echo -n "my-secret-data" | base64)'"}}'
```

**Access secret data:**
```bash
curl "http://localhost:8080/v1/projects/my-project/secrets/my-secret/versions/1:access"
```

**List secrets:**
```bash
curl "http://localhost:8080/v1/projects/my-project/secrets"
```

**Disable a version:**
```bash
curl -X POST "http://localhost:8080/v1/projects/my-project/secrets/my-secret/versions/1:disable"
```

**REST API matches GCP's official REST endpoints** - same paths, same JSON format, same behavior.

## Docker

### Build Docker Images

```bash
# Build all variants
make docker

# Or build individually
docker build --build-arg VARIANT=grpc -t emulator:grpc .  # gRPC only (default)
docker build --build-arg VARIANT=rest -t emulator:rest .  # REST only
docker build --build-arg VARIANT=dual -t emulator:dual .  # Both protocols
```

### Run Docker Containers

**gRPC only:**
```bash
docker run -p 9090:9090 gcp-secret-manager-emulator:grpc
```

**REST only:**
```bash
docker run -p 8080:8080 gcp-secret-manager-emulator:rest
# Access via: curl http://localhost:8080/v1/projects/test/secrets
```

**Dual protocol (both gRPC + REST):**
```bash
docker run -p 9090:9090 -p 8080:8080 gcp-secret-manager-emulator:dual
# gRPC on :9090, REST on :8080
```

### In CI/CD

**GitHub Actions:**
```yaml
services:
  gcp-emulator:
    image: gcp-secret-manager-emulator:dual
    ports:
      - 9090:9090
      - 8080:8080
```

**Docker Compose:**
```yaml
services:
  gcp-emulator:
    image: gcp-secret-manager-emulator:dual
    ports:
      - "9090:9090"  # gRPC
      - "8080:8080"  # REST
    environment:
      - GCP_MOCK_LOG_LEVEL=debug
```

## Use Cases

- **Local Development** - Test GCP Secret Manager integration without cloud access
- **CI/CD Pipelines** - Fast integration tests without GCP credentials
- **Unit Testing** - Deterministic test environment
- **Demos & Prototyping** - Showcase GCP integrations offline
- **Cost Reduction** - Avoid GCP API charges during development

## IAM Integration

The Secret Manager emulator supports optional permission checks using the [GCP IAM Emulator](https://github.com/blackwell-systems/gcp-iam-emulator).

### Configuration

**Environment Variables:**

- `IAM_MODE` - Controls permission enforcement (default: `off`)
  - `off` - No permission checks (legacy behavior)
  - `permissive` - Check permissions, fail-open on connectivity errors
  - `strict` - Check permissions, fail-closed on connectivity errors (for CI)
- `IAM_HOST` - IAM emulator address (default: `localhost:8080`)

### Usage

**Without IAM (default):**
```bash
server-dual
```

**With IAM (permissive mode):**
```bash
IAM_MODE=permissive IAM_HOST=localhost:8080 server-dual
```

**With IAM (strict mode for CI):**
```bash
IAM_MODE=strict IAM_HOST=localhost:8080 server-dual
```

### Principal Injection

Specify the calling principal for permission checks:

**gRPC:**
```go
ctx := metadata.AppendToOutgoingContext(ctx, "x-emulator-principal", "user:admin@example.com")
resp, err := client.CreateSecret(ctx, req)
```

**REST:**
```bash
curl -H "X-Emulator-Principal: user:admin@example.com" \
  -X POST "http://localhost:8080/v1/projects/my-project/secrets" \
  -H "Content-Type: application/json" \
  -d '{"secretId":"my-secret"}'
```

### Permissions

Secret Manager operations map to GCP IAM permissions:

| Operation | Permission | Resource |
|-----------|-----------|----------|
| CreateSecret | `secretmanager.secrets.create` | Parent project |
| GetSecret | `secretmanager.secrets.get` | Secret |
| UpdateSecret | `secretmanager.secrets.update` | Secret |
| DeleteSecret | `secretmanager.secrets.delete` | Secret |
| ListSecrets | `secretmanager.secrets.list` | Parent project |
| AddSecretVersion | `secretmanager.versions.add` | Secret |
| AccessSecretVersion | `secretmanager.versions.access` | Secret version |
| GetSecretVersion | `secretmanager.versions.get` | Secret version |
| ListSecretVersions | `secretmanager.versions.list` | Secret |
| EnableSecretVersion | `secretmanager.versions.enable` | Secret version |
| DisableSecretVersion | `secretmanager.versions.disable` | Secret version |
| DestroySecretVersion | `secretmanager.versions.destroy` | Secret version |

### Mode Differences

| Scenario | `off` | `permissive` | `strict` |
|----------|-------|--------------|----------|
| No IAM emulator | Allow | Allow | Deny |
| IAM unavailable | Allow | Allow | Deny |
| No principal | Allow | Deny | Deny |
| Permission denied | Allow | Deny | Deny |

**Use `off` for local dev, `permissive` for integration tests, `strict` for CI.**

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

## Documentation

📚 **[View Full Documentation](https://blackwell-systems.github.io/gcp-secret-manager-emulator/)**

- **[API Reference](docs/API-REFERENCE.md)** - Complete API documentation with examples
- **[Architecture Guide](docs/ARCHITECTURE.md)** - System design, components, and diagrams
- **[Roadmap](docs/ROADMAP.md)** - Planned features and future direction
- **[Changelog](docs/CHANGELOG.md)** - Version history and release notes
- **[Security Policy](SECURITY.md)** - Security guidelines and reporting
- **[Brand Guidelines](BRAND.md)** - Trademark and logo usage
- **[Maintainers](MAINTAINERS.md)** - Project maintainers and contact info

## Testing

```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# With race detector
go test -race ./...
```

## API Coverage

**Implemented (11 of 12 methods):**
All Secret Manager operations except IAM methods.

**Not Implemented:**
- IAM methods (`SetIamPolicy`, `GetIamPolicy`, `TestIamPermissions`)

**Rationale:** IAM methods are not needed for local testing since the emulator has no authentication. All requests succeed regardless of permissions.

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

Extracted from [vaultmux](https://github.com/blackwell-systems/vaultmux) where it powers GCP backend integration tests.

## Disclaimer

This project is not affiliated with, endorsed by, or sponsored by Google LLC or Google Cloud Platform. "Google Cloud", "Secret Manager", and related trademarks are property of Google LLC. This is an independent open-source implementation for testing and development purposes.

## Maintained By

Maintained by **Dayna Blackwell** — founder of Blackwell Systems, building reference infrastructure for cloud-native development.

[GitHub](https://github.com/blackwell-systems) · [LinkedIn](https://linkedin.com/in/dayna-blackwell) · [Blog](https://blog.blackwell-systems.com)

## Related Projects

- [**GCP IAM Control Plane**](https://github.com/blackwell-systems/gcp-iam-control-plane) - CLI to orchestrate the Local IAM Control Plane (this emulator + IAM + others)
- [GCP IAM Emulator](https://github.com/blackwell-systems/gcp-iam-emulator) - Policy engine (the brain) for IAM enforcement
- [GCP KMS Emulator](https://github.com/blackwell-systems/gcp-kms-emulator) - IAM-enforced KMS data plane
- [gcp-emulator-auth](https://github.com/blackwell-systems/gcp-emulator-auth) - Enforcement proxy library (the guard)

## Trademarks

**Blackwell Systems™** and the **Blackwell Systems logo** are trademarks of Dayna Blackwell. You may use the name "Blackwell Systems" to refer to this project, but you may not use the name or logo in a way that suggests endorsement or official affiliation without prior written permission. See [BRAND.md](BRAND.md) for usage guidelines.

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.
