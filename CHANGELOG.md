# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- IAM integration tests used wrong environment variable (`IAM_HOST`) to configure
  the IAM emulator host; the `gcp-emulator-auth` library reads `IAM_EMULATOR_HOST`.
  Tests now use the correct variable for skip guards, connectivity simulation, and
  server setup.
- Integration test for "permissive mode without principal" had incorrect expectations:
  permissive mode only fails-open on connectivity errors, not on clean policy denials.
  Test renamed and expectation corrected to `PermissionDenied`.
- IAM integration tests now manage policy state explicitly per-test (`setIAMTestPolicy`
  / `clearIAMTestPolicy`) so deny tests always run against a clean emulator state,
  and allow tests set up and tear down their own policy bindings.

## [1.3.0] - 2026-01-28

### Changed
- **Breaking**: Startup log strings changed from "GCP Secret Manager Mock Server" to
  "GCP Secret Manager Emulator". Users parsing stdout for the old string (log alerts,
  CI grep scripts) must update their patterns.
- **Breaking**: PermissionDenied error messages now include context. The `checkPermission`
  error message format has changed from the generic `"Permission denied"` to a
  detailed message that includes the principal, the required permission, and the
  resource. Example:
  `"Permission denied: principal 'user:x@example.com' lacks 'secretmanager.secrets.get' on resource 'projects/p/secrets/s'"`
  When no principal header is present, the principal is rendered as `"(no principal)"`.
  Callers doing an exact string match on `"Permission denied"` must update their
  checks; callers using `strings.Contains(err.Error(), "Permission denied")` are
  unaffected.
- **Component Identification**: Pass "gcp-secret-manager-emulator" to auth client
  - Enables trace analysis tools to identify calling service
  - Authorization traces now show both policy engine and requesting component
- Upgraded to gcp-emulator-auth v0.3.0 (requires component parameter)
- Enhanced README with hermetic seal narrative
  - Explains pre-flight IAM enforcement vs post-hoc observation
  - Clarifies control plane/data plane architecture
  - Positions Secret Manager as data plane in Blackwell ecosystem

## [1.2.2] - 2026-01-27

### Added
- REST-only Docker image workflow for HTTP-only deployments
- Dual-protocol Docker image workflow (gRPC + HTTP)

## [1.2.1] - 2026-01-27

### Changed
- Updated Control Plane description to mention CLI orchestration
- Improved README clarity on standalone vs orchestrated deployment modes

## [1.2.0] - 2026-01-26

### Added
- **IAM Integration**: Optional permission checks with GCP IAM Emulator
  - Three authorization modes: `off` (legacy), `permissive` (fail-open), `strict` (fail-closed)
  - Environment variables: `IAM_MODE` and `IAM_HOST`
  - Principal injection via `x-emulator-principal` (gRPC) and `X-Emulator-Principal` (HTTP)
  - Complete permission mapping for all Secret Manager operations
  - Integration with `gcp-emulator-auth` shared library
  - Integration tests covering all three IAM modes
- **Documentation**: IAM Integration section in README
  - Configuration guide
  - Usage examples for all three modes
  - Permission mapping table
  - Mode comparison table
- Docker Compose orchestration with IAM emulator

### Changed
- `NewServer()` now returns `(*Server, error)` to handle IAM client initialization errors
- Server struct includes `iamClient` and `iamMode` fields
- All operations check permissions before storage calls (when IAM enabled)

### Technical Details
- Backward compatible: IAM enforcement is opt-in via `IAM_MODE` environment variable
- Permission checks use `gcp-emulator-auth` library (v0.1.0+)
- Fail-open mode allows graceful degradation during IAM unavailability
- Strict mode ensures production parity for CI/CD pipelines

## [1.1.0] - 2026-01-25

### Added
- **REST/HTTP API Support**: Full HTTP/JSON gateway alongside gRPC
  - Complete Secret Manager v1 REST API implementation
  - Dual-protocol server binary (`server-dual`)
  - REST-only server binary (`server-rest`)
  - HTTP port configuration via `--http-port` flag
  - Support for both gRPC and REST in same process
- Docker images for all protocol combinations
  - `ghcr.io/blackwell-systems/gcp-secret-manager-emulator:latest` (dual protocol)
  - `ghcr.io/blackwell-systems/gcp-secret-manager-emulator:rest-only` (HTTP only)

### Changed
- Improved README with Quick Start section prominently placed
- Moved API limitations section lower in documentation
- Enhanced usage examples for both protocols

### Technical Details
- REST API follows Google's HTTP/JSON mapping conventions
- Resource names in URL paths, request bodies as JSON
- Standard HTTP status codes (200, 404, 403, 500)
- Content-Type: application/json

## [1.0.0] - 2026-01-24

### Added
- Complete Secret Manager v1 gRPC API implementation
  - CreateSecret, GetSecret, UpdateSecret, DeleteSecret
  - AddSecretVersion, GetSecretVersion, AccessSecretVersion
  - ListSecrets, ListSecretVersions
  - Resource name validation and normalization
- In-memory storage with thread-safe access
- Project-scoped secret isolation
- Secret version lifecycle management (ENABLED, DISABLED, DESTROYED)
- Base64 payload encoding/decoding
- Comprehensive error handling with proper gRPC status codes
- Docker image with GitHub Container Registry publishing
- CI/CD with automated testing and image builds

### Technical Details
- Implements `google.cloud.secretmanager.v1.SecretManagerService`
- Compatible with official GCP client libraries
- No persistence between restarts (in-memory only)
- No authentication required (local development)
- No IAM enforcement (all operations allowed)

## [0.2.0] - 2026-01-23

### Added
- Initial functional release with core Secret Manager operations
- Basic secret creation and retrieval
- Version management
- gRPC server implementation

### Known Limitations
- No IAM integration (all requests allowed)
- No persistence (in-memory only)
- No REST API (gRPC only)

---

[Unreleased]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.3.0...HEAD
[1.3.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.2.2...v1.3.0
[1.2.2]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v0.2.0...v1.0.0
[0.2.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/releases/tag/v0.2.0
