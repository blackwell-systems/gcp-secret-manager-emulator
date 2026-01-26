# GCP Secret Manager Emulator

![version](https://img.shields.io/github/v/release/blackwell-systems/gcp-secret-manager-emulator)

> The reference local implementation of the Google Cloud Secret Manager API for development and CI

**Dual Protocol Support**: Native gRPC + REST/HTTP

```bash
# gRPC only (fastest)
go install .../cmd/server@latest && server

# REST API (curl-friendly)
go install .../cmd/server-rest@latest && server-rest

# Both protocols
go install .../cmd/server-dual@latest && server-dual

# Docker
docker run -p 9090:9090 -p 8080:8080 ghcr.io/blackwell-systems/gcp-secret-manager-emulator:dual
```

**Quick Test Examples:**
```bash
# gRPC with SDK
go test ./...  # Works with cloud.google.com/go/secretmanager

# REST with curl
curl http://localhost:8080/v1/projects/test/secrets
```

- **Dual Protocol** - Both gRPC and REST/HTTP APIs
- **Perfect for Testing** - No GCP credentials, works entirely offline
- **CI/CD Ready** - Docker support, starts in milliseconds
- **Real SDK Compatible** - Drop-in replacement for official SDK
- **REST Compatible** - Matches GCP's REST endpoint format
- **Integration Testing** - Deterministic behavior, thread-safe operations
- **Production Tested** - 90.8% test coverage, 92% API coverage

[Get Started](#quick-start)
[VIEW ON GITHUB](https://github.com/blackwell-systems/gcp-secret-manager-emulator)

![color](#1a1a1a)
