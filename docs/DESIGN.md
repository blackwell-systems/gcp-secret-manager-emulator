# GCP Secret Manager Mock Server

A lightweight mock implementation of Google Cloud Secret Manager API for local testing and CI/CD integration.

## Overview

This mock server implements the GCP Secret Manager gRPC API, allowing vaultmux (and other projects) to run integration tests without requiring real GCP credentials or incurring API costs.

**Status**: Design Document (Not Yet Implemented)

### Why Build This?

- **No Official Emulator**: Unlike AWS (LocalStack), GCP doesn't provide a Secret Manager emulator
- **CI/CD Testing**: Enable integration tests in GitHub Actions without GCP credentials
- **Fast Iteration**: Local development without network latency or rate limits
- **Zero Cost**: Unlimited testing without consuming GCP free tier quota
- **Ecosystem Value**: Can be extracted as standalone tool for broader Go/GCP community

### Design Philosophy

1. **Extraction-Ready**: Architected from day one to be extracted as standalone project
2. **Zero vaultmux Coupling**: No imports of vaultmux code in mock server implementation
3. **Standard Compliance**: Implements official Secret Manager gRPC protocol
4. **Minimal Dependencies**: Only official GCP protobuf definitions required
5. **Production-Like Behavior**: Match real API responses for realistic testing

## Architecture

### Directory Structure

```
vaultmux/
├── cmd/
│   └── gcp-secret-manager-mock/
│       ├── main.go                    # Standalone server binary
│       ├── config.go                  # Configuration (flags, env vars)
│       └── README.md                  # Usage documentation
├── internal/
│   └── gcpmock/
│       ├── server.go                  # gRPC server setup and lifecycle
│       ├── storage.go                 # In-memory secret storage
│       ├── secret_service.go          # SecretManagerService implementation
│       ├── validation.go              # Request validation
│       ├── errors.go                  # Error response helpers
│       ├── server_test.go             # Unit tests for mock
│       ├── integration_test.go        # Integration test with GCP SDK
│       └── README.md                  # Implementation documentation
├── scripts/
│   └── run-gcp-mock.sh                # Helper script for local testing
├── Dockerfile.gcpmock                 # Separate Docker image
└── docs/
    └── gcp-mock-secret-server.md      # This file
```

### Component Separation

**`internal/gcpmock`** - Core Implementation
- Pure Go package, no vaultmux imports
- Can be copied to new repo with zero modifications
- Implements `secretmanagerpb.SecretManagerServiceServer` interface

**`cmd/gcp-secret-manager-mock`** - Standalone Binary
- Runnable server: `go run ./cmd/gcp-secret-manager-mock`
- Command-line flags and environment variable configuration
- Can be distributed as standalone binary

**`Dockerfile.gcpmock`** - Container Image
- Multi-stage build for minimal image size
- Can be published to Docker Hub independently
- Used in GitHub Actions CI

## API Implementation

### Official gRPC Service Interface

Based on the official `google.cloud.secretmanager.v1.SecretManagerService` proto definition.

**Service**: `google.cloud.secretmanager.v1.SecretManagerService`
**Protocol**: gRPC (Protocol Buffers)
**Proto Package**: `google.cloud.secretmanager.v1`

#### Complete Method List (from official proto)

**Secret Management Operations**:
1. `ListSecrets(ListSecretsRequest) returns (ListSecretsResponse)` - Paginated secret listing
2. `CreateSecret(CreateSecretRequest) returns (Secret)` - Create secret metadata
3. `GetSecret(GetSecretRequest) returns (Secret)` - Get secret metadata
4. `UpdateSecret(UpdateSecretRequest) returns (Secret)` - Update secret labels/annotations
5. `DeleteSecret(DeleteSecretRequest) returns (google.protobuf.Empty)` - Delete secret and all versions

**Secret Version Operations**:
6. `AddSecretVersion(AddSecretVersionRequest) returns (SecretVersion)` - Add new version with payload
7. `GetSecretVersion(GetSecretVersionRequest) returns (SecretVersion)` - Get version metadata
8. `AccessSecretVersion(AccessSecretVersionRequest) returns (AccessSecretVersionResponse)` - Access payload data
9. `ListSecretVersions(ListSecretVersionsRequest) returns (ListSecretVersionsResponse)` - List versions
10. `DisableSecretVersion(DisableSecretVersionRequest) returns (SecretVersion)` - Disable a version
11. `EnableSecretVersion(EnableSecretVersionRequest) returns (SecretVersion)` - Enable a version
12. `DestroySecretVersion(DestroySecretVersionRequest) returns (SecretVersion)` - Permanently destroy version

**IAM Operations** (optional for MVP):
13. `SetIamPolicy(SetIamPolicyRequest) returns (Policy)` - Set access control
14. `GetIamPolicy(GetIamPolicyRequest) returns (Policy)` - Get access control
15. `TestIamPermissions(TestIamPermissionsRequest) returns (TestIamPermissionsResponse)` - Test permissions

### MVP Scope (vaultmux Requirements)

Based on analysis of `backends/gcpsecrets/gcpsecrets.go`:

**Phase 1 - Core Operations (6 methods)**:
1. ✅ **CreateSecret** - Used in CreateItem() line 310
2. ✅ **GetSecret** - Used in GetItem() line 189 for metadata
3. ✅ **ListSecrets** - Used in ListItems() line 239 with iterator
4. ✅ **DeleteSecret** - Used in DeleteItem() line 389
5. ✅ **AddSecretVersion** - Used in CreateItem() line 323 and UpdateItem() line 358
6. ✅ **AccessSecretVersion** - Used in GetItem() line 182 to retrieve payload

**Phase 2 - Future Enhancement**:
- UpdateSecret (for labels/annotations)
- ListSecretVersions (version history)
- GetSecretVersion (version metadata)
- DisableSecretVersion/EnableSecretVersion (lifecycle management)
- DestroySecretVersion (permanent deletion)

**Not Needed**:
- IAM operations (mock has no authentication)

## Message Structures (from Official Proto)

### Key Message Types

Based on `google/cloud/secretmanager/v1/*.proto`:

**Secret** (secret metadata):
```protobuf
message Secret {
  string name = 1;                           // projects/{project}/secrets/{secret-id}
  Replication replication = 2;               // Automatic or UserManaged
  google.protobuf.Timestamp create_time = 3;
  map<string, string> labels = 4;
  repeated Topic topics = 5;                 // Pub/Sub notifications
  google.protobuf.Timestamp expire_time = 6;
  oneof expiration {
    google.protobuf.Timestamp expire_time = 6;
    google.protobuf.Duration ttl = 7;
  }
  map<string, string> annotations = 13;
  string version_aliases = 14;
}
```

**SecretVersion** (version metadata):
```protobuf
message SecretVersion {
  string name = 1;  // projects/{project}/secrets/{secret-id}/versions/{version-id}
  google.protobuf.Timestamp create_time = 2;
  google.protobuf.Timestamp destroy_time = 3;
  State state = 4;  // ENABLED, DISABLED, DESTROYED
  ReplicationStatus replication_status = 5;
}
```

**SecretPayload** (actual secret data):
```protobuf
message SecretPayload {
  bytes data = 1;  // The actual secret content
}
```

**AccessSecretVersionResponse**:
```protobuf
message AccessSecretVersionResponse {
  string name = 1;         // Version resource name
  SecretPayload payload = 2;  // The secret data
}
```

### Resource Naming Patterns

**Official GCP Resource Name Format**:
- **Project**: `projects/{project-id}` or `projects/{project-number}`
- **Secret**: `projects/{project-id}/secrets/{secret-id}`
- **Version**: `projects/{project-id}/secrets/{secret-id}/versions/{version-id}`
- **Version ID**: Integer (`1`, `2`, `3`) or special alias `latest`

**Examples from vaultmux code**:
- Full version path: `projects/test-project/secrets/test-vaultmux-myapp/versions/latest` (line 176)
- Secret path: `projects/test-project/secrets/test-vaultmux-myapp` (line 188)
- Parent (for listing): `projects/test-project` (line 232)

## Data Model

### In-Memory Storage Implementation

```go
// Storage is the mock's in-memory secret store
type Storage struct {
    mu      sync.RWMutex
    secrets map[string]*StoredSecret  // key: "projects/{project}/secrets/{secret-id}"
}

// StoredSecret represents a secret with all its versions
type StoredSecret struct {
    // Secret metadata (from SecretManagerPb.Secret)
    Name        string                        // projects/{project}/secrets/{secret-id}
    CreateTime  *timestamppb.Timestamp
    Labels      map[string]string
    Annotations map[string]string
    Replication *secretmanagerpb.Replication // Always Automatic for mock

    // Version management
    Versions    map[string]*StoredVersion     // key: "1", "2", "3", etc. (not "latest")
    NextVersion int64                         // Auto-increment: 1, 2, 3...
}

// StoredVersion represents a single secret version
type StoredVersion struct {
    // Version metadata
    Name       string                           // full resource name with version
    CreateTime *timestamppb.Timestamp
    State      secretmanagerpb.SecretVersion_State  // ENABLED, DISABLED, DESTROYED

    // Actual secret data
    Payload    []byte                           // The secret content
}
```

### Version Resolution

The mock implements GCP's version alias behavior:
- `versions/latest` → resolves to highest version number in ENABLED state
- `versions/1`, `versions/2` → specific version numbers
- Version IDs are sequential integers starting from 1

## Error Handling

### gRPC Status Codes

The mock must return proper gRPC status codes matching real GCP behavior:

**From vaultmux's handleGCPError() (line 418-441)**:
- `codes.NotFound` → Secret/version doesn't exist
- `codes.AlreadyExists` → Secret ID already taken
- `codes.PermissionDenied` → IAM check failed (not implemented in mock)
- `codes.Unauthenticated` → No valid credentials (not implemented in mock)
- `codes.InvalidArgument` → Malformed request

**Additional Standard Codes**:
- `codes.OK` → Success (implicit)
- `codes.Internal` → Server error
- `codes.Unimplemented` → Method not implemented

### Error Response Format

```go
import "google.golang.org/grpc/status"
import "google.golang.org/grpc/codes"

// Example: Secret not found
return nil, status.Error(codes.NotFound,
    fmt.Sprintf("Secret [projects/%s/secrets/%s] not found", projectID, secretID))

// Example: Already exists
return nil, status.Error(codes.AlreadyExists,
    fmt.Sprintf("Secret [%s] already exists", resourceName))
```

### Critical Behaviors to Match

**1. NotFound for Missing Resources**:
- GetSecret on non-existent secret → `codes.NotFound`
- AccessSecretVersion on non-existent version → `codes.NotFound`
- DeleteSecret on non-existent secret → Should we return NotFound or succeed? (Check GCP docs)

**2. AlreadyExists on Conflicts**:
- CreateSecret with duplicate secret_id → `codes.AlreadyExists`

**3. InvalidArgument for Bad Input**:
- Empty parent in ListSecrets → `codes.InvalidArgument`
- Invalid resource name format → `codes.InvalidArgument`
- Negative page_size → `codes.InvalidArgument`

**4. Version "latest" Resolution**:
- Must return highest version number where State == ENABLED
- If no ENABLED versions exist → `codes.NotFound`

## Implementation Plan

### Phase 1: Core Server (MVP)

**Step 1: Project Setup**
- Create directory structure
- Add gRPC dependencies to go.mod
- Generate Go code from Secret Manager proto files

**Step 2: Storage Layer**
- Implement in-memory storage (storage.go)
- Thread-safe operations with sync.RWMutex
- Basic CRUD for secrets and versions

**Step 3: gRPC Service**
- Implement SecretManagerService methods
- Request validation
- Error responses matching GCP format

**Step 4: Server Binary**
- Command-line interface (cmd/gcp-secret-manager-mock)
- Configuration (port, project ID patterns)
- Graceful shutdown

**Step 5: Testing**
- Unit tests for storage layer
- Integration test with real GCP SDK client
- Verify vaultmux integration tests pass

### Phase 2: Containerization

**Step 6: Docker Image**
- Create Dockerfile.gcpmock
- Multi-stage build
- Health check endpoint
- Test locally

**Step 7: CI Integration**
- Add to GitHub Actions workflow
- Service container like LocalStack
- Update vaultmux integration tests

### Phase 3: Documentation & Polish

**Step 8: Documentation**
- Usage examples
- API compatibility notes
- Limitations and differences from real GCP

**Step 9: Extraction Preparation**
- Verify zero vaultmux dependencies
- Standalone README
- License headers

## Configuration

### Command-Line Flags

```bash
gcp-secret-manager-mock \
  --port 9090 \
  --project-id test-project \
  --verbose
```

### Environment Variables

```bash
GCP_MOCK_PORT=9090
GCP_MOCK_PROJECT_ID=test-project
GCP_MOCK_LOG_LEVEL=debug
```

### Docker Usage

```bash
# Run container
docker run -d \
  -p 9090:9090 \
  -e GCP_MOCK_PROJECT_ID=test-project \
  --name gcp-mock \
  blackwell-systems/gcp-secret-manager-mock:latest

# Health check
curl http://localhost:9090/health
```

## Integration with vaultmux

### Local Testing

```bash
# Terminal 1: Start mock server
go run ./cmd/gcp-secret-manager-mock --port 9090

# Terminal 2: Run integration tests
GCP_MOCK_ENDPOINT=localhost:9090 \
GCP_PROJECT_ID=test-project \
go test -v ./backends/gcpsecrets/
```

### Update Integration Test

```go
// backends/gcpsecrets/gcpsecrets_integration_test.go
func TestIntegration(t *testing.T) {
    // Check for mock endpoint first
    endpoint := os.Getenv("GCP_MOCK_ENDPOINT")
    projectID := os.Getenv("GCP_PROJECT_ID")

    if endpoint == "" && projectID == "" {
        t.Skip("Neither GCP_MOCK_ENDPOINT nor GCP_PROJECT_ID set - skipping")
    }

    // Use mock if available, otherwise real GCP
    options := map[string]string{
        "project_id": projectID,
        "prefix":     "test-vaultmux-",
    }
    if endpoint != "" {
        options["endpoint"] = endpoint
    }

    backend, err := New(options, "")
    // ... rest of test
}
```

### CI Configuration

```yaml
# .github/workflows/ci.yml
integration-gcp:
  name: GCP Integration Tests (Mock)
  runs-on: ubuntu-latest
  services:
    gcp-mock:
      image: blackwell-systems/gcp-secret-manager-mock:latest
      ports:
        - 9090:9090
      env:
        GCP_MOCK_PROJECT_ID: test-project
      options: >-
        --health-cmd "grpc_health_probe -addr=:9090"
        --health-interval 10s
        --health-timeout 5s
        --health-retries 5
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version: '1.23'

    - name: Run GCP integration tests
      env:
        GCP_MOCK_ENDPOINT: localhost:9090
        GCP_PROJECT_ID: test-project
      run: |
        go test -v -race -coverprofile=coverage-gcp.out ./backends/gcpsecrets/
        go tool cover -func=coverage-gcp.out | grep total

    - name: Upload GCP coverage
      uses: codecov/codecov-action@v4
      with:
        file: ./coverage-gcp.out
        flags: integration-gcp
```

## Testing Strategy

### Mock Server Tests

**Unit Tests** (internal/gcpmock/server_test.go):
- Storage CRUD operations
- Resource name parsing
- Version auto-increment
- Error conditions

**Integration Tests** (internal/gcpmock/integration_test.go):
- Real GCP SDK client connects to mock
- Full CRUD lifecycle
- Pagination
- Error handling

### vaultmux Integration

**GCP Backend Tests** (backends/gcpsecrets/gcpsecrets_integration_test.go):
- All existing tests should pass against mock
- Coverage should increase significantly (target: 75%+)
- Test execution time: <5 seconds

## API Compatibility

### What We Implement

✅ **Core CRUD Operations**: Create, Read, Update, Delete secrets
✅ **Version Management**: Multiple versions per secret
✅ **Latest Version Alias**: `versions/latest` automatically resolves
✅ **Resource Names**: Standard GCP resource name format
✅ **gRPC Protocol**: Full gRPC implementation, not REST
✅ **Error Codes**: Match GCP error status codes
✅ **Pagination**: List operations support page tokens

### Limitations (vs Real GCP)

❌ **No IAM**: No permission/role checking (all operations allowed)
❌ **No Persistence**: In-memory storage only (resets on restart)
❌ **No Replication**: Single-instance, no multi-region
❌ **No Audit Logs**: No Cloud Logging integration
❌ **Simplified Metadata**: Minimal secret metadata tracking
❌ **No Quotas**: No rate limiting or quota enforcement

### Acceptable Differences

These differences don't affect vaultmux integration tests:
- Faster response times (no network latency)
- Simpler resource IDs (sequential numbers)
- No eventual consistency (immediate consistency)
- No billing/metering

## Future Extraction

### As Standalone Project: `gcp-secret-manager-mock`

**Repository Structure**:
```
gcp-secret-manager-mock/
├── cmd/
│   └── server/
│       └── main.go
├── pkg/
│   └── gcpmock/          # Copied from vaultmux/internal/gcpmock
│       ├── server.go
│       ├── storage.go
│       └── ...
├── Dockerfile
├── go.mod
├── LICENSE               # Apache 2.0
└── README.md
```

**Extraction Checklist**:
1. ✅ Copy `internal/gcpmock` → `pkg/gcpmock`
2. ✅ Copy `cmd/gcp-secret-manager-mock` → `cmd/server`
3. ✅ Copy `Dockerfile.gcpmock` → `Dockerfile`
4. ✅ Update import paths
5. ✅ Create standalone go.mod
6. ✅ Write standalone README with usage examples
7. ✅ Add GitHub Actions CI for standalone repo
8. ✅ Publish Docker image to Docker Hub
9. ✅ Announce to Go/GCP community

**Potential Impact**:
- Other Go projects testing GCP Secret Manager can use it
- Could become de facto standard for GCP Secret Manager testing
- Similar to LocalStack's impact on AWS testing ecosystem

## Dependencies

### Required Runtime Dependencies

Based on the vaultmux GCP backend imports:

```go
// gRPC server framework
google.golang.org/grpc                                  // v1.60.0+

// Protocol Buffer support
google.golang.org/protobuf                              // v1.31.0+

// GCP Secret Manager API definitions (protobuf messages and service)
cloud.google.com/go/secretmanager/apiv1/secretmanagerpb // Contains all message types

// API options for client configuration
google.golang.org/api/option                            // Client options

// gRPC status codes and error handling
google.golang.org/grpc/codes                            // Status code constants
google.golang.org/grpc/status                           // Status error creation

// Google Protobuf well-known types
google.golang.org/protobuf/types/known/timestamppb      // Timestamp messages
google.golang.org/protobuf/types/known/emptypb          // Empty message type

// Standard library (no external deps)
// context, sync, fmt, strings, time, etc.
```

### Development/Testing Dependencies

```go
// GCP SDK client for integration tests
cloud.google.com/go/secretmanager/apiv1                 // Real GCP client
google.golang.org/api/iterator                          // Iterator pattern

// Testing utilities
github.com/google/go-cmp/cmp                            // Deep comparison for tests
```

###Note on Proto Files

**We do NOT need to generate code from .proto files**. The official GCP Go packages already provide:
- `secretmanagerpb.SecretManagerServiceServer` interface (we implement this)
- All request/response message types
- Protobuf serialization/deserialization

We simply implement the `SecretManagerServiceServer` interface using the pre-generated types.

### Zero vaultmux Dependencies

The mock server MUST NOT import:
- ❌ `github.com/blackwell-systems/vaultmux` (no core package imports)
- ❌ Any vaultmux internal packages
- ❌ Any vaultmux types (Backend, Session, Item, etc.)

This ensures clean extraction as standalone project.

## Implementation Estimate

### Time Breakdown

**Phase 1: Core Server (MVP)**
- Storage layer: 1 hour
- gRPC service implementation: 2 hours
- Server binary and configuration: 1 hour
- Unit and integration tests: 1.5 hours
- **Subtotal: 5.5 hours**

**Phase 2: Containerization**
- Dockerfile and build: 30 minutes
- CI integration: 1 hour
- Testing and debugging: 30 minutes
- **Subtotal: 2 hours**

**Phase 3: Documentation**
- Code documentation: 30 minutes
- Usage examples: 30 minutes
- Update TESTING.md: 15 minutes
- Update CHANGELOG.md: 15 minutes
- **Subtotal: 1.5 hours**

**Total Estimate: ~9 hours** (can be done in 2-3 sessions)

### Phased Delivery

**Session 1** (2-3 hours): Core server implementation
- Storage + basic gRPC handlers
- Unit tests
- Runnable locally

**Session 2** (2-3 hours): Complete API + Integration
- All 6 MVP methods
- Integration test with GCP SDK
- vaultmux integration tests passing

**Session 3** (2-3 hours): Docker + CI + Documentation
- Dockerfile
- GitHub Actions integration
- Documentation and polish

## Success Criteria

### MVP Completion

✅ All 6 core methods implemented
✅ vaultmux integration tests pass against mock
✅ GCP backend coverage increases to 75%+
✅ Tests run in <5 seconds
✅ Mock server starts in <1 second
✅ Zero vaultmux imports in mock code

### Production Ready

✅ Docker image builds successfully
✅ Health check endpoint working
✅ GitHub Actions CI job passing
✅ Documentation complete
✅ CHANGELOG updated
✅ Ready for future extraction

## Security Considerations

### Local Development

- Mock server binds to localhost by default
- No authentication required (testing only)
- Secrets stored in memory only (cleared on exit)
- Not intended for production use

### CI Environment

- GitHub Actions service container (isolated)
- No external network access needed
- Ephemeral (destroyed after test run)
- No secrets persistence

### Warnings

⚠️ **Not for Production**: This mock server is for testing only
⚠️ **No Security**: Anyone with network access can read/write secrets
⚠️ **No Persistence**: All data lost on restart
⚠️ **No Audit Trail**: Operations are not logged

## References

### Official GCP Secret Manager Documentation

**API Reference**:
- [Secret Manager RPC Reference](https://cloud.google.com/secret-manager/docs/reference/rpc) - Complete API documentation
- [Proto Definitions (GitHub)](https://github.com/googleapis/googleapis/tree/master/google/cloud/secretmanager/v1) - Official .proto files

**Go Client Library**:
- [Go Client Package Docs](https://pkg.go.dev/cloud.google.com/go/secretmanager/apiv1) - Official Go SDK
- [Proto Message Types](https://pkg.go.dev/cloud.google.com/go/secretmanager/apiv1/secretmanagerpb) - Generated protobuf types
- [Client Options](https://pkg.go.dev/google.golang.org/api/option) - Endpoint configuration

**gRPC Documentation**:
- [gRPC Go Tutorial](https://grpc.io/docs/languages/go/basics/) - gRPC fundamentals
- [gRPC Status Codes](https://grpc.io/docs/guides/status-codes/) - Error handling
- [gRPC Server Guide](https://grpc.io/docs/languages/go/basics/#server) - Server implementation

### Similar Projects (Inspiration)

- **LocalStack** ([github.com/localstack/localstack](https://github.com/localstack/localstack)) - AWS service emulator (architectural inspiration)
- **Google Cloud SDK Emulators** - Official emulators for Datastore, Pub/Sub, Bigtable, Firestore
- **fake-gcs-server** ([github.com/fsouza/fake-gcs-server](https://github.com/fsouza/fake-gcs-server)) - Google Cloud Storage emulator (similar community project)
- **Azurite** ([github.com/Azure/Azurite](https://github.com/Azure/Azurite)) - Azure Storage emulator (Microsoft-maintained)

### Related vaultmux Documentation

- [TESTING.md](TESTING.md) - Overall testing strategy and LocalStack integration
- [ARCHITECTURE.md](ARCHITECTURE.md) - Backend architecture patterns
- [backends/gcpsecrets/gcpsecrets.go](backends/gcpsecrets/gcpsecrets.go) - GCP backend implementation
- [gcp-mock-secret-server.md](gcp-mock-secret-server.md) - This document

---

**Next Steps**: Review this design document, then proceed with implementation following the phased approach outlined above.
