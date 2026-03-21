# Roadmap

This document outlines the planned features and improvements for the GCP Secret Manager Emulator.

## v1.3.0 - Current Release ✓  (2026-01-28)

**IAM Enforcement — Production Parity**
- Breaking: startup log strings changed from "Mock" to "Emulator"
- Breaking: PermissionDenied messages now include principal/permission/resource
- Component ID "gcp-secret-manager-emulator" passed to auth client
- Upgraded to gcp-emulator-auth v0.3.0

## v1.2.2 - Released ✓  (2026-01-27)

- REST-only Docker image workflow for HTTP-only deployments
- Dual-protocol Docker image workflow (gRPC + HTTP)

## v1.2.1 - Released ✓  (2026-01-27)

- Updated Control Plane description to mention CLI orchestration
- Improved README clarity on standalone vs orchestrated deployment modes

## v1.2.0 - Released ✓  (2026-01-26)

**IAM Integration**
- Optional permission checks via GCP IAM Emulator
- Three modes: off (legacy), permissive (fail-open), strict (fail-closed)
- Docker Compose orchestration with IAM emulator

## v1.1.0 - Released ✓  (2026-01-25)

**Dual Protocol Support**
- ✅ REST/HTTP API alongside gRPC (complete feature parity)
- ✅ Three server variants: gRPC-only, REST-only, Dual-protocol
- ✅ Custom HTTP gateway with GCP-compatible endpoints
- ✅ All 12 methods accessible via REST
- ✅ JSON request/response with protobuf marshaling
- ✅ Health check endpoint (`/health`)
- ✅ Docker multi-variant builds
- ✅ Complete REST documentation and examples
- ✅ Makefile targets for all variants

**Why this matters:**
- Use official GCP SDK (gRPC) OR curl/scripts (REST)
- Deploy only what you need (16MB gRPC vs 18MB REST/Dual)
- Maximum flexibility: choose protocol per use case
- Complete coverage: only emulator with both protocols

**Docker usage:**
```bash
# gRPC only
docker run -p 9090:9090 gcp-secret-manager-emulator:grpc

# REST only
docker run -p 8080:8080 gcp-secret-manager-emulator:rest

# Both protocols
docker run -p 9090:9090 -p 8080:8080 gcp-secret-manager-emulator:dual
```

## v1.0.0 - Released 2026-01-26 ✓

**Complete API Implementation**
- 12 of 12 Secret Manager methods (100% API coverage)
- Full version lifecycle (Enable, Disable, Destroy)
- UpdateSecret with FieldMask support
- 90.8% test coverage
- Complete documentation
- Docker support with multi-arch images

## Future Considerations

These features may be considered based on user demand:

### Optional Persistence
- File-based storage option for long-running instances
- JSON or SQLite backend
- Opt-in (default remains in-memory)
- Use case: Development environments, integration test suites

### Prometheus Metrics
- Export operation counts, latency, error rates
- Help users monitor emulator performance in CI/CD
- Standard `/metrics` endpoint

### Enhanced Filtering
- Label-based secret filtering in ListSecrets
- More complex filter expressions
- Match production GCP filtering capabilities

### Web UI (Low Priority)
- Simple web interface to view/manage secrets
- Useful for local development and demos
- Not critical (CLI tools and SDKs are primary interface)

## Not Planned

These features are explicitly out of scope:

**Production Use**
- No plans for production-ready features
- Emulator is designed for testing only
- Use real GCP Secret Manager for production

**Per-Resource IAM Methods**
- SetIamPolicy, GetIamPolicy, TestIamPermissions
- Authorization is handled via the IAM Emulator control plane instead
- Per-resource policy storage would add complexity without real benefit
- IAM enforcement via control plane was shipped in v1.2.0 and extended in v1.3.0.

**Encryption at Rest**
- In-memory storage is intentionally plaintext
- Testing doesn't require encryption
- Use real GCP for encryption requirements

**Multi-Region Replication**
- Single in-memory store by design
- Fast and simple for testing
- Replication is a production concern

**Cloud Logging / Audit Trails**
- Not needed for testing workflows
- Emulator is ephemeral by design

## Contributing Ideas

Have a feature request? 

- Open an issue: https://github.com/blackwell-systems/gcp-secret-manager-emulator/issues
- Start a discussion: https://github.com/blackwell-systems/gcp-secret-manager-emulator/discussions
- Contact maintainer: See [MAINTAINERS.md](../MAINTAINERS.md)

We prioritize features that:
- Improve testing workflows
- Enhance CI/CD integration
- Maintain simplicity and speed
- Don't compromise the "zero-configuration" principle

## Release Schedule

- **Minor versions (1.x.0)**: New features, typically every 2-3 months
- **Patch versions (1.0.x)**: Bug fixes, as needed
- **Major versions (2.0.0)**: Breaking changes, when necessary

## Changelog

For detailed release history, see [CHANGELOG.md](CHANGELOG.md).
