# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- `server-rest` and `server-dual` no longer abort on graceful shutdown: the HTTP gateway goroutine treated the expected `http.ErrServerClosed` from `Stop` as a fatal error (`log.Fatalf` → `os.Exit`), which skipped the persistence final flush. As a result, a mutation made within the persistence debounce window just before shutdown could be lost. The gateway now ignores `ErrServerClosed`, so `srv.Close()` runs and the latest state is flushed.

## [1.8.0] - 2026-06-11

### Added

- **Optional persistence** — set `GCP_MOCK_PERSIST=true` to persist secrets across restarts. State is loaded on startup from and snapshotted to `/data/secrets.json` (atomic writes by a single background flusher with 100ms debounce; final flush on graceful shutdown). Opt-in and off by default, so the in-memory behavior and zero overhead are unchanged. JSON backend keeps zero new dependencies and the `CGO_ENABLED=0` static build. Only secrets are persisted (not IAM policies). `Dockerfile` now declares a `/data` volume; `docker-compose*.yml` show the volume wiring. ⚠️ Secrets are written as plaintext JSON — local/CI use only. (contributed by [@jmarette](https://github.com/jmarette) in [#4](https://github.com/blackwell-systems/gcp-secret-manager-emulator/pull/4))

### Fixed

- Corrected hardcoded version strings in all server binaries (was 1.3.0, now matches release)
- Fixed CHANGELOG.md comparison links for v1.5.0 through v1.7.2
- Fixed `.gitignore` patterns that silently ignored source directories (`internal/server`, `cmd/server*`)

## [1.7.2] - 2026-04-06

### Fixed

- Pin Dockerfile to golang:1.24-alpine for explicit version pinning

## [1.7.1] - 2026-04-05

### Changed

- Updated `emulatorauth.NewClient` to v0.4.0 signature (removed `component` parameter)
- Added `git` to Docker builder stage for private module downloads

## [1.7.0] - 2026-04-03

### Added
- `GetIamPolicy`, `SetIamPolicy`, `TestIamPermissions` — in-memory IAM policy store per resource; standard SDK policy management now works
- Etag on `Secret` and `SecretVersion` — generated on create, refreshed on update; validated on `DeleteSecret`, `UpdateSecret`, `Enable/Disable/DestroySecretVersion` (returns `ABORTED` on mismatch)
- `SecretVersion.destroy_time` — recorded when `DestroySecretVersion` is called
- `SecretPayload.data_crc32c` — verified on `AddSecretVersion` (returns `DATA_LOSS` on mismatch), stored, and returned in `AccessSecretVersionResponse`
- `version_aliases` — user-defined aliases stored via `UpdateSecret`; resolved in `GetSecretVersion` and `AccessSecretVersion`
- `ListSecrets` filter — supports `name:` prefix and `labels.key=value` expressions

### Changed
- `ListSecrets` default sort order changed to descending create_time (matches real GCP API)
- `ListSecretVersions` default sort order changed to descending version number
- `ListSecrets` and `ListSecretVersions` now return `total_size` (count before pagination)
- 14 new conformance tests covering all 6 critical fixes

## [1.6.0] - 2026-04-03

### Changed
- **REST gateway migrated from hand-rolled HTTP to grpc-gateway v2** — HTTP handlers are now auto-generated from the Secret Manager proto definitions, ensuring full API compatibility with real GCP

### Added
- `Register()` composition hook for unified `gcp-emulator`
- `NewGatewayHandler()` for mounting SM REST gateway in unified HTTP server
- `gateway.Handler()` method for embedding in parent HTTP multiplexer
- `/healthz` and `/readyz` health endpoints on REST gateway
- `jsonErrorHandler` returns clean 400 for malformed JSON bodies
- `buf.gen.yaml` for reproducible grpc-gateway stub generation

### Fixed
- REST gateway now returns correct HTTP status codes: NotFound→404, AlreadyExists→409, InvalidArgument→400, PermissionDenied→403 (previously all mapped to 500)
- REST gateway now returns structured GCP-format error responses (`{"code":N,"message":"..."}`)
- Malformed JSON request bodies now return 400 instead of being silently accepted
- `Register()` no longer calls `reflection.Register`, preventing fatal duplicate registration when composing multiple emulators

### Removed
- Hand-rolled HTTP gateway (480 lines, replaced by ~80 lines of grpc-gateway wiring)
- `gateway_test.go` (REST coverage now in gcp-emulator integration tests)

## [1.4.0] - 2026-03-22

### Fixed
- REST `UpdateSecret` (PATCH) now correctly forwards the `update_mask` query
  parameter to the gRPC layer. Previously the field mask was never set on the
  request, causing every PATCH to return an error regardless of payload.
- REST gateway now returns correct HTTP status codes for gRPC errors. Previously
  all gRPC errors mapped to 500; correct mapping is now enforced:
  `NotFound`→404, `AlreadyExists`→409, `InvalidArgument`/`FailedPrecondition`→400,
  `PermissionDenied`→403, `Unauthenticated`→401. **Behavior change**: HTTP
  clients that branched on 500 for not-found or permission errors must update
  their error handling.
- `GetSecret` and `GetSecretVersion` previously hardcoded 404 for all errors,
  masking `PermissionDenied` and internal errors as not-found responses. Both
  now use the correct HTTP status code derived from the gRPC status.
- `gateway.NewServer` no longer panics on gRPC dial failure. It now returns
  `(*Server, error)` so callers can handle startup failures gracefully.
- REST gateway (`server-dual`, `server-rest`) now correctly propagates the
  `X-Emulator-Principal` header to the gRPC layer. Previously the header was
  silently dropped, effectively bypassing IAM enforcement for all HTTP clients
  regardless of `IAM_MODE`. **Behavior change**: HTTP requests that send
  `X-Emulator-Principal` with `IAM_MODE=permissive` or `IAM_MODE=strict` will
  now be subject to IAM checks; requests that were previously allowed through
  may now return `PermissionDenied`.
- `ListSecretVersions` now returns versions in numeric order (`1, 2, 3, ..., 10,
  11, 12`) instead of lexicographic order (`1, 10, 11, 12, 2, 3, ...`). Only
  affects secrets with 10 or more versions. **Behavior change**: clients
  depending on the previous lexicographic order will see a different sequence.
- `ListSecrets` now returns secrets in stable alphabetical order on every call.
  Previously, results were in random map-iteration order, which could produce
  duplicates or skipped entries across paginated calls.
- Fixed data race in `gateway.Server` where `Start()` and `Stop()` accessed
  `httpServer` concurrently without synchronization. The field is now protected
  by a mutex, eliminating the race detected by `-race` in `TestGatewayStartStop`.
- README documented `IAM_HOST` as the IAM emulator address variable; the
  `gcp-emulator-auth` library reads `IAM_EMULATOR_HOST`. All occurrences
  updated.
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

[Unreleased]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.8.0...HEAD
[1.8.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.7.2...v1.8.0
[1.7.2]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.7.1...v1.7.2
[1.7.1]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.7.0...v1.7.1
[1.7.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.6.0...v1.7.0
[1.6.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.4.0...v1.6.0
[1.4.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.2.2...v1.3.0
[1.2.2]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v0.2.0...v1.0.0
[0.2.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/releases/tag/v0.2.0
