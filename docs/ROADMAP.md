# Roadmap

This document outlines the planned features and improvements for the GCP Secret Manager Emulator.

## v1.1.0 - Current Release ✓

**Dual Protocol Support**
- ✅ REST/HTTP API alongside gRPC (complete feature parity)
- ✅ Three server variants: gRPC-only, REST-only, Dual-protocol
- ✅ Custom HTTP gateway with GCP-compatible endpoints
- ✅ All 11 methods accessible via REST
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
- 11 of 12 Secret Manager methods (92% API coverage)
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

**IAM Methods**
- SetIamPolicy, GetIamPolicy, TestIamPermissions
- Not needed for testing (no authentication by design)
- Would add complexity without real benefit

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
