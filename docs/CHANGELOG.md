# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.0] - 2026-01-27

### Added
- **IAM Integration**: Optional permission checks with GCP IAM Emulator
  - Three authorization modes: `off` (legacy), `permissive` (fail-open), `strict` (fail-closed)
  - Environment variables: `IAM_MODE` and `IAM_HOST`
  - Principal injection via `x-emulator-principal` (gRPC) and `X-Emulator-Principal` (HTTP)
  - Complete permission mapping for all 12 Secret Manager operations
  - Integration with `gcp-emulator-auth` shared library
  - Resource normalization for secrets, versions, and parent projects
  - Integration tests covering all three modes
- **Docker Compose**: Multi-mode orchestration examples
  - Default service with `IAM_MODE=permissive`
  - Strict mode service for CI workflows
  - Legacy mode service (IAM disabled)
  - Health checks and service dependencies
- **Documentation**: IAM Integration section in README
  - Configuration guide
  - Usage examples for all three modes
  - Permission mapping table
  - Mode comparison table

### Changed
- `NewServer()` now returns `(*Server, error)` to handle IAM client initialization errors
- Server struct includes `iamClient` and `iamMode` fields
- All operations check permissions before storage calls (when IAM enabled)
- Backward compatible: IAM disabled by default (`IAM_MODE=off`)

### Technical Details
- Uses `gcp-emulator-auth v0.0.0-20260126234751-6976d522b21f`
- Permission checks placed after validation, before storage operations
- Non-breaking change: existing deployments unaffected
- Fail-open vs fail-closed behavior configurable per environment

## [1.1.0] - 2026-01-26

### Added
- **REST/HTTP API Support**: Full REST API implementation alongside existing gRPC
  - Three server variants: `server` (gRPC), `server-rest` (REST), `server-dual` (both)
  - HTTP gateway at `internal/gateway` with GCP-compatible endpoints
  - All 11 methods accessible via REST: `POST /v1/projects/{p}/secrets`, `GET /v1/.../versions/{v}:access`, etc.
  - JSON request/response format with protobuf marshaling
  - Base64 encoding for secret payloads
  - Health check endpoint at `/health`
- **Docker Multi-Variant Builds**: Build-time selection via `VARIANT` argument
  - `docker build --build-arg VARIANT=grpc` - gRPC only (default)
  - `docker build --build-arg VARIANT=rest` - REST only
  - `docker build --build-arg VARIANT=dual` - Both protocols
- **Makefile Targets**: `make build-rest`, `make build-dual`, `make docker-grpc`, `make docker-rest`, `make docker-dual`
- **Documentation**: 
  - REST API examples in README and API-REFERENCE
  - REST endpoint quick reference table
  - Dual protocol architecture diagrams
  - Docker usage for all variants

### Changed
- Binary sizes: gRPC-only 16MB (unchanged), REST/Dual 18MB (+2MB for gateway)
- Port exposure: Dual mode exposes both 9090 (gRPC) and 8080 (HTTP)
- Documentation updated across README, API-REFERENCE, ARCHITECTURE, coverpage

### Technical Details
- REST gateway uses internal gRPC client (no code generation required)
- Custom HTTP router parsing GCP REST paths
- Thread-safe operation (shared storage layer)
- Zero protocol-specific bloat in gRPC-only builds

## [1.0.0] - 2026-01-26

### Added
- **UpdateSecret**: Modify secret metadata (labels, annotations) with field mask support
  - Selective field updates via FieldMask (labels, annotations)
  - Idempotent operation
  - Comprehensive error handling (InvalidArgument, NotFound)
- **DestroySecretVersion**: Permanently destroy a version (irreversible, clears payload)
  - Sets version state to DESTROYED
  - Permanently removes payload data
  - Idempotent operation (destroying twice succeeds)
  - AccessSecretVersion returns FailedPrecondition for destroyed versions
  - Latest alias skips destroyed versions
- **Documentation**: SECURITY.md, MAINTAINERS.md, BRAND.md
- **Examples**: Added version lifecycle and metadata update examples

### Changed
- Latest alias resolution now skips both disabled and destroyed versions
- API coverage increased to 92% (11 of 12 methods implemented)
- Test coverage increased to 90.8%
- Documentation reorganized: single source in docs/ directory

### Fixed
- Pagination bug in ListSecretVersions causing duplicate results (map iteration was non-deterministic)

## [0.2.0] - 2026-01-25

### Added
- **ListSecretVersions**: List all versions of a secret with pagination support (#1)
- **DisableSecretVersion**: Disable a secret version to prevent access (#1)
- **EnableSecretVersion**: Re-enable a previously disabled version (#1)
- **Filter support**: ListSecretVersions now supports `state:ENABLED`, `state:DISABLED`, `state:DESTROYED` filters
- **Soft-delete testing**: Disabling all versions makes `AccessSecretVersion(latest)` return NotFound

### Fixed
- CI coverage upload condition now uses correct Go version (1.24 instead of 1.23)

### Changed
- AccessSecretVersion now returns `FailedPrecondition` when accessing disabled versions (per GCP API spec)
- Latest alias resolution skips disabled versions (resolves to highest ENABLED version only)

## [0.1.0] - 2026-01-21

### Added
- Initial release
- Core Secret Manager API implementation:
  - CreateSecret
  - GetSecret
  - ListSecrets (with pagination)
  - DeleteSecret
  - AddSecretVersion
  - GetSecretVersion
  - AccessSecretVersion (with "latest" alias support)
- In-memory storage with thread-safe operations
- gRPC server implementation
- Docker container support
- Comprehensive documentation (README, API Reference, Architecture)
- Docsify documentation site
- Integration tests with real GCP SDK client
- CI/CD with multi-platform testing (Ubuntu, macOS, Windows)
- Race detection in tests

### Security
- Runs as non-root user in Docker
- No authentication by design (testing-only emulator)

[Unreleased]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v0.2.0...v1.0.0
[0.2.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/releases/tag/v0.1.0
