# Roadmap

This document outlines the planned features and improvements for the GCP Secret Manager Emulator.

## v1.0.0 - Current Release ✓

**Complete API Implementation**
- 11 of 12 Secret Manager methods (92% API coverage)
- Full version lifecycle (Enable, Disable, Destroy)
- UpdateSecret with FieldMask support
- 90.8% test coverage
- Complete documentation
- Docker support with multi-arch images

## v1.1.0 - Planned

**REST API Support**
- Add REST/HTTP endpoints alongside existing gRPC API
- Auto-generated using [grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway)
- Full feature parity with gRPC interface
- Swagger/OpenAPI documentation
- `curl` examples in documentation
- HTTP server on configurable port (default: 8080)

**Benefits:**
- Test with `curl` without gRPC clients
- Broader language support (any HTTP client)
- Lower barrier to entry for new users
- Complete feature coverage vs competitors

**Docker example:**
```bash
docker run -p 9090:9090 -p 8080:8080 gcp-secret-manager-emulator
# gRPC on :9090, REST on :8080
```

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
