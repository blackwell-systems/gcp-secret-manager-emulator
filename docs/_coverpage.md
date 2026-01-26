# GCP Secret Manager Emulator

![version](https://img.shields.io/github/v/release/blackwell-systems/gcp-secret-manager-emulator)

> Test GCP Secret Manager locally without credentials

```bash
# Option 1: Go install
go install github.com/blackwell-systems/gcp-secret-manager-emulator/cmd/server@latest
server

# Option 2: Docker
docker run -p 9090:9090 ghcr.io/blackwell-systems/gcp-secret-manager-emulator
```

**Quick Test Example:**
```bash
# Terminal 1 - Start emulator
server

# Terminal 2 - Run your tests
go test ./...  # Works with any GCP Secret Manager tests
```

- **Perfect for Testing** - No GCP credentials, works entirely offline
- **CI/CD Ready** - Docker support, starts in milliseconds
- **Mock GCP Locally** - Full gRPC API with 92% method coverage
- **Real SDK Compatible** - Drop-in replacement for `cloud.google.com/go/secretmanager`
- **Integration Testing** - Deterministic behavior, thread-safe operations
- **Multi-Platform** - Docker images for amd64 and arm64
- **Production Tested** - 90.8% test coverage, used in real CI pipelines

[Get Started](#quick-start)
[VIEW ON GITHUB](https://github.com/blackwell-systems/gcp-secret-manager-emulator)

![color](#1a1a1a)
