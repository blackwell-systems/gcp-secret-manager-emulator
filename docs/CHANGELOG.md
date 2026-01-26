# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v0.2.0...v1.0.0
[0.2.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/blackwell-systems/gcp-secret-manager-emulator/releases/tag/v0.1.0
