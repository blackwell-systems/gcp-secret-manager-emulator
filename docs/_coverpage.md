# GCP Secret Manager Emulator

> Lightweight gRPC emulator for the Google Cloud Secret Manager API

```bash
go install github.com/blackwell-systems/gcp-secret-manager-emulator/cmd/server@latest
server
```

- **No GCP Credentials** - Works entirely offline without authentication
- **Complete API** - 11 of 12 methods implemented (92% API coverage)
- **Fast & Lightweight** - In-memory storage, starts in milliseconds
- **Thread-Safe** - Concurrent access with proper synchronization
- **Docker Support** - Multi-arch images (amd64, arm64)
- **Real SDK Compatible** - Works with official `cloud.google.com/go/secretmanager` client
- **90.8% Test Coverage** - Comprehensive integration tests

[Get Started](#quick-start)
[VIEW ON GITHUB](https://github.com/blackwell-systems/gcp-secret-manager-emulator)

![color](#1a1a1a)
