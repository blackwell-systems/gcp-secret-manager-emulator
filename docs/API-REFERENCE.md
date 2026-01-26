# API Reference

Complete reference for the GCP Secret Manager Emulator API.

## Overview

The emulator implements the Google Cloud Secret Manager v1 gRPC API. All methods match the official API signature and behavior for common operations.

**Base Service:** `google.cloud.secretmanager.v1.SecretManagerService`

**gRPC Endpoint:** `localhost:9090` (default)

## Connection

### Go Client

```go
import (
    "context"
    secretmanager "cloud.google.com/go/secretmanager/apiv1"
    "google.golang.org/api/option"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

ctx := context.Background()
conn, err := grpc.NewClient(
    "localhost:9090",
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)

client, err := secretmanager.NewClient(ctx, option.WithGRPCConn(conn))
defer client.Close()
```

### Python Client

```python
from google.cloud import secretmanager
import grpc

channel = grpc.insecure_channel('localhost:9090')
client = secretmanager.SecretManagerServiceClient(
    transport=secretmanager.transports.SecretManagerServiceGrpcTransport(
        channel=channel
    )
)
```

## Methods

### CreateSecret

Creates a new secret (metadata only, no versions yet).

**Request:**
```protobuf
message CreateSecretRequest {
  string parent = 1;    // Required: "projects/{project-id}"
  string secret_id = 2; // Required: Secret ID (unique within project)
  Secret secret = 3;    // Required: Secret metadata
}
```

**Response:**
```protobuf
message Secret {
  string name = 1;                         // "projects/{project}/secrets/{secret-id}"
  google.protobuf.Timestamp create_time = 2;
  map<string, string> labels = 3;
  map<string, string> annotations = 4;
  Replication replication = 5;
}
```

**Example (Go):**
```go
secret, err := client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
    Parent:   "projects/test-project",
    SecretId: "my-api-key",
    Secret: &secretmanagerpb.Secret{
        Labels: map[string]string{
            "env": "dev",
        },
        Replication: &secretmanagerpb.Replication{
            Replication: &secretmanagerpb.Replication_Automatic_{
                Automatic: &secretmanagerpb.Replication_Automatic{},
            },
        },
    },
})
```

**Errors:**
- `InvalidArgument` - Missing parent, secret_id, or secret
- `AlreadyExists` - Secret with same ID already exists

---

### GetSecret

Retrieves secret metadata (not version payload).

**Request:**
```protobuf
message GetSecretRequest {
  string name = 1; // Required: "projects/{project}/secrets/{secret-id}"
}
```

**Response:** `Secret` message (see CreateSecret)

**Example (Go):**
```go
secret, err := client.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{
    Name: "projects/test-project/secrets/my-api-key",
})

fmt.Println(secret.Labels["env"]) // "dev"
```

**Errors:**
- `InvalidArgument` - Missing name
- `NotFound` - Secret doesn't exist

---

### ListSecrets

Lists all secrets in a project with pagination support.

**Request:**
```protobuf
message ListSecretsRequest {
  string parent = 1;     // Required: "projects/{project-id}"
  int32 page_size = 2;   // Optional: Max results per page (default: 100)
  string page_token = 3; // Optional: Token from previous response
}
```

**Response:**
```protobuf
message ListSecretsResponse {
  repeated Secret secrets = 1;     // Secrets on this page
  string next_page_token = 2;      // Token for next page (empty if done)
}
```

**Example (Go):**
```go
// List first page
resp, err := client.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
    Parent:   "projects/test-project",
    PageSize: 10,
})

for _, secret := range resp.Secrets {
    fmt.Println(secret.Name)
}

// List next page if available
if resp.NextPageToken != "" {
    resp2, err := client.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
        Parent:    "projects/test-project",
        PageSize:  10,
        PageToken: resp.NextPageToken,
    })
    // ...
}
```

**Example (Iterator Pattern):**
```go
iter := client.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
    Parent:   "projects/test-project",
    PageSize: 10,
})

for {
    secret, err := iter.Next()
    if err == iterator.Done {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(secret.Name)
}
```

**Errors:**
- `InvalidArgument` - Missing parent

---

### DeleteSecret

Deletes a secret and all its versions permanently.

**Request:**
```protobuf
message DeleteSecretRequest {
  string name = 1; // Required: "projects/{project}/secrets/{secret-id}"
}
```

**Response:** `google.protobuf.Empty`

**Example (Go):**
```go
err := client.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{
    Name: "projects/test-project/secrets/my-api-key",
})
```

**Errors:**
- `InvalidArgument` - Missing name
- `NotFound` - Secret doesn't exist

---

### AddSecretVersion

Adds a new version with payload to an existing secret.

**Request:**
```protobuf
message AddSecretVersionRequest {
  string parent = 1;        // Required: "projects/{project}/secrets/{secret-id}"
  SecretPayload payload = 2; // Required: Secret data
}

message SecretPayload {
  bytes data = 1; // The secret content
}
```

**Response:**
```protobuf
message SecretVersion {
  string name = 1;                         // "projects/{project}/secrets/{secret}/versions/{version}"
  google.protobuf.Timestamp create_time = 2;
  State state = 3;                         // ENABLED, DISABLED, DESTROYED
}
```

**Example (Go):**
```go
version, err := client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
    Parent: "projects/test-project/secrets/my-api-key",
    Payload: &secretmanagerpb.SecretPayload{
        Data: []byte("super-secret-value"),
    },
})

fmt.Println(version.Name) // "projects/test-project/secrets/my-api-key/versions/1"
```

**Behavior:**
- Version IDs auto-increment: 1, 2, 3, ...
- All new versions created with `State: ENABLED`
- Previous versions remain accessible

**Errors:**
- `InvalidArgument` - Missing parent or payload
- `NotFound` - Secret doesn't exist

---

### GetSecretVersion

Retrieves version metadata (not payload data).

**Request:**
```protobuf
message GetSecretVersionRequest {
  string name = 1; // Required: "projects/{project}/secrets/{secret}/versions/{version}"
}
```

**Response:** `SecretVersion` message (see AddSecretVersion)

**Example (Go):**
```go
version, err := client.GetSecretVersion(ctx, &secretmanagerpb.GetSecretVersionRequest{
    Name: "projects/test-project/secrets/my-api-key/versions/1",
})

fmt.Println(version.State) // ENABLED
```

**Special Alias:**
- `versions/latest` - Resolves to highest ENABLED version

**Errors:**
- `InvalidArgument` - Missing name or invalid format
- `NotFound` - Secret or version doesn't exist

---

### AccessSecretVersion

Retrieves the payload data for a specific version.

**Request:**
```protobuf
message AccessSecretVersionRequest {
  string name = 1; // Required: "projects/{project}/secrets/{secret}/versions/{version}"
}
```

**Response:**
```protobuf
message AccessSecretVersionResponse {
  string name = 1;          // Full version name
  SecretPayload payload = 2; // The secret data
}
```

**Example (Go):**
```go
// Access latest version
result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
    Name: "projects/test-project/secrets/my-api-key/versions/latest",
})

secretValue := string(result.Payload.Data)
fmt.Println(secretValue) // "super-secret-value"

// Access specific version
result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
    Name: "projects/test-project/secrets/my-api-key/versions/1",
})
```

**Special Alias:**
- `versions/latest` - Returns highest ENABLED version

**Errors:**
- `InvalidArgument` - Missing name or invalid format
- `NotFound` - Secret, version doesn't exist, or no enabled versions
- `FailedPrecondition` - Version exists but is not ENABLED (disabled or destroyed)

---

### ListSecretVersions

Lists all versions of a secret with pagination and filtering support.

**Request:**
```protobuf
message ListSecretVersionsRequest {
  string parent = 1;     // Required: "projects/{project}/secrets/{secret-id}"
  int32 page_size = 2;   // Optional: Max results per page (default: 100)
  string page_token = 3; // Optional: Token from previous response
  string filter = 4;     // Optional: Filter expression (e.g., "state:ENABLED")
}
```

**Response:**
```protobuf
message ListSecretVersionsResponse {
  repeated SecretVersion versions = 1; // Versions on this page
  string next_page_token = 2;          // Token for next page (empty if done)
}
```

**Example (Go):**
```go
// List all versions
resp, err := client.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
    Parent: "projects/test-project/secrets/my-api-key",
})

for _, version := range resp.Versions {
    fmt.Printf("Version: %s, State: %v\n", version.Name, version.State)
}

// List only enabled versions
resp, err := client.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
    Parent: "projects/test-project/secrets/my-api-key",
    Filter: "state:ENABLED",
})

// List only disabled versions
resp, err := client.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
    Parent: "projects/test-project/secrets/my-api-key",
    Filter: "state:DISABLED",
})
```

**Example (Python):**
```python
# List all versions
request = secretmanager.ListSecretVersionsRequest(
    parent="projects/test-project/secrets/my-api-key"
)
response = client.list_secret_versions(request=request)

for version in response.versions:
    print(f"Version: {version.name}, State: {version.state}")

# Filter by state
request = secretmanager.ListSecretVersionsRequest(
    parent="projects/test-project/secrets/my-api-key",
    filter="state:ENABLED"
)
```

**Supported Filters:**
- `state:ENABLED` - Only enabled versions
- `state:DISABLED` - Only disabled versions
- `state:DESTROYED` - Only destroyed versions
- No filter - All versions

**Errors:**
- `InvalidArgument` - Missing parent
- `NotFound` - Secret doesn't exist

---

### DisableSecretVersion

Disables a secret version, preventing access via AccessSecretVersion.

**Request:**
```protobuf
message DisableSecretVersionRequest {
  string name = 1; // Required: "projects/{project}/secrets/{secret}/versions/{version}"
}
```

**Response:**
```protobuf
message SecretVersion {
  string name = 1;
  google.protobuf.Timestamp create_time = 2;
  State state = 3; // Will be DISABLED
}
```

**Example (Go):**
```go
// Disable a specific version
version, err := client.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{
    Name: "projects/test-project/secrets/my-api-key/versions/1",
})

fmt.Println(version.State) // DISABLED

// Attempting to access disabled version will fail
_, err = client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
    Name: "projects/test-project/secrets/my-api-key/versions/1",
})
// Returns: FailedPrecondition error
```

**Example (Python):**
```python
# Disable version
request = secretmanager.DisableSecretVersionRequest(
    name="projects/test-project/secrets/my-api-key/versions/1"
)
version = client.disable_secret_version(request=request)

print(version.state)  # DISABLED
```

**Behavior:**
- Sets version state to DISABLED
- AccessSecretVersion will return `FailedPrecondition` for disabled versions
- Latest alias skips disabled versions (resolves to next highest ENABLED version)
- ListSecretVersions still returns disabled versions (use filter to exclude)

**Use Case - Soft Delete:**
```go
// Soft delete: Disable all versions instead of deleting secret
versions, _ := client.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
    Parent: secretName,
    Filter: "state:ENABLED",
})

for _, v := range versions.Versions {
    client.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{
        Name: v.Name,
    })
}

// Now AccessSecretVersion(latest) returns NotFound (no enabled versions)
```

**Errors:**
- `InvalidArgument` - Missing name or invalid format
- `NotFound` - Secret or version doesn't exist
- `FailedPrecondition` - Version is already DESTROYED

---

### EnableSecretVersion

Re-enables a previously disabled secret version.

**Request:**
```protobuf
message EnableSecretVersionRequest {
  string name = 1; // Required: "projects/{project}/secrets/{secret}/versions/{version}"
}
```

**Response:** `SecretVersion` message with state set to ENABLED

**Example (Go):**
```go
// Re-enable a disabled version
version, err := client.EnableSecretVersion(ctx, &secretmanagerpb.EnableSecretVersionRequest{
    Name: "projects/test-project/secrets/my-api-key/versions/1",
})

fmt.Println(version.State) // ENABLED

// Now AccessSecretVersion works again
result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
    Name: "projects/test-project/secrets/my-api-key/versions/1",
})
```

**Example (Python):**
```python
# Enable version
request = secretmanager.EnableSecretVersionRequest(
    name="projects/test-project/secrets/my-api-key/versions/1"
)
version = client.enable_secret_version(request=request)

print(version.state)  # ENABLED
```

**Behavior:**
- Sets version state to ENABLED
- AccessSecretVersion will succeed for enabled versions
- Latest alias includes enabled versions in resolution

**Errors:**
- `InvalidArgument` - Missing name or invalid format
- `NotFound` - Secret or version doesn't exist
- `FailedPrecondition` - Version is DESTROYED (cannot re-enable destroyed versions)

---

### UpdateSecret

Updates secret metadata (labels, annotations) without modifying versions.

**Request:**
```protobuf
message UpdateSecretRequest {
  Secret secret = 1;              // Required: Secret with updated field values
  FieldMask update_mask = 2;      // Required: Specifies which fields to update
}
```

**Response:** `Secret` message with updated fields

**Example (Go):**
```go
// Update labels only
secret, err := client.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
    Secret: &secretmanagerpb.Secret{
        Name: "projects/test-project/secrets/my-api-key",
        Labels: map[string]string{
            "env":     "production",
            "version": "2.0",
        },
    },
    UpdateMask: &fieldmaskpb.FieldMask{
        Paths: []string{"labels"},
    },
})

fmt.Println(secret.Labels) // map[env:production version:2.0]
```

**Example (Python):**
```python
from google.protobuf import field_mask_pb2

# Update annotations only
request = secretmanager.UpdateSecretRequest(
    secret={
        "name": "projects/test-project/secrets/my-api-key",
        "annotations": {
            "owner": "security-team",
            "rotation": "quarterly"
        }
    },
    update_mask=field_mask_pb2.FieldMask(paths=["annotations"])
)
secret = client.update_secret(request=request)
```

**Supported Update Mask Paths:**
- `labels` - Replace all labels
- `annotations` - Replace all annotations

**Behavior:**
- Only fields in update_mask are modified
- Update replaces entire map (not merged)
- Versions are not affected
- CreateTime and Name are immutable

**Use Cases:**
- Changing environment labels (dev → prod)
- Adding owner information
- Updating cost center tags
- Modifying rotation policies

**Errors:**
- `InvalidArgument` - Missing secret.name or update_mask
- `NotFound` - Secret doesn't exist

---

### DestroySecretVersion

Permanently destroys a secret version, making it irreversibly inaccessible.

**Request:**
```protobuf
message DestroySecretVersionRequest {
  string name = 1;  // Required: "projects/{project}/secrets/{secret}/versions/{version}"
  string etag = 2;  // Optional: Not enforced in emulator
}
```

**Response:** `SecretVersion` message with state set to DESTROYED

**Example (Go):**
```go
// Destroy a version permanently
version, err := client.DestroySecretVersion(ctx, &secretmanagerpb.DestroySecretVersionRequest{
    Name: "projects/test-project/secrets/my-api-key/versions/1",
})

fmt.Println(version.State) // DESTROYED

// Cannot access destroyed version
_, err = client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
    Name: "projects/test-project/secrets/my-api-key/versions/1",
})
// Returns: FailedPrecondition error
```

**Example (Python):**
```python
# Destroy version
request = secretmanager.DestroySecretVersionRequest(
    name="projects/test-project/secrets/my-api-key/versions/1"
)
version = client.destroy_secret_version(request=request)

print(version.state)  # DESTROYED
```

**Behavior:**
- Sets version state to DESTROYED
- Permanently removes the payload data
- Operation is idempotent (destroying again succeeds)
- AccessSecretVersion returns `FailedPrecondition`
- Cannot be reversed (unlike Disable)
- ListSecretVersions still returns destroyed versions
- Latest alias skips destroyed versions

**Destroy vs Disable:**

| Feature | Disable | Destroy |
|---------|---------|---------|
| Reversible | Yes (EnableSecretVersion) | No |
| Payload data | Retained | Permanently deleted |
| Use case | Temporary deactivation | Compliance, permanent removal |
| Recovery | Can re-enable | Cannot recover |

**Use Cases:**
- Compliance requirements (GDPR, data retention policies)
- Permanent removal of leaked credentials
- Cleanup after security incidents
- Reducing storage costs by removing old data

**Errors:**
- `InvalidArgument` - Missing name or invalid format
- `NotFound` - Secret or version doesn't exist

---

## Unimplemented Methods

---

### IAM Methods

SetIamPolicy, GetIamPolicy, TestIamPermissions.

**Status:** Not implemented

**Rationale:** Emulator has no authentication - all requests succeed

**Use Case:** Access control testing

---

## Error Codes

The emulator uses standard gRPC status codes:

| Code | Situation | Example |
|------|-----------|---------|
| `InvalidArgument` | Missing required field | Empty parent, name, or secret_id |
| `NotFound` | Resource doesn't exist | Secret or version not found |
| `AlreadyExists` | Duplicate resource | Creating secret with existing ID |
| `FailedPrecondition` | Invalid state | Accessing disabled version |
| `Unimplemented` | Feature not supported | UpdateSecret, IAM methods |

## Resource Naming Convention

### Project

Format: `projects/{project-id}`

Example: `projects/test-project`

**Note:** Project IDs are not validated. Use any string.

### Secret

Format: `projects/{project-id}/secrets/{secret-id}`

Example: `projects/test-project/secrets/my-api-key`

**Rules:**
- Secret IDs must be unique within a project
- Secret IDs can contain letters, numbers, hyphens, underscores

### Secret Version

Format: `projects/{project-id}/secrets/{secret-id}/versions/{version-id}`

Example: `projects/test-project/secrets/my-api-key/versions/1`

**Version IDs:**
- Auto-incrementing integers: "1", "2", "3", ...
- Special alias: "latest" (resolves to highest ENABLED version)

## Complete Workflow Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    secretmanager "cloud.google.com/go/secretmanager/apiv1"
    "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
    "google.golang.org/api/iterator"
    "google.golang.org/api/option"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

func main() {
    ctx := context.Background()

    // Connect to emulator
    conn, err := grpc.NewClient(
        "localhost:9090",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        log.Fatal(err)
    }

    client, err := secretmanager.NewClient(ctx, option.WithGRPCConn(conn))
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // 1. Create a secret
    secret, err := client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
        Parent:   "projects/test-project",
        SecretId: "database-password",
        Secret: &secretmanagerpb.Secret{
            Labels: map[string]string{
                "app": "myapp",
                "env": "production",
            },
            Replication: &secretmanagerpb.Replication{
                Replication: &secretmanagerpb.Replication_Automatic_{
                    Automatic: &secretmanagerpb.Replication_Automatic{},
                },
            },
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Created secret: %s\n", secret.Name)

    // 2. Add a secret version with payload
    version, err := client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
        Parent: secret.Name,
        Payload: &secretmanagerpb.SecretPayload{
            Data: []byte("my-database-password-123"),
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Added version: %s\n", version.Name)

    // 3. Access the secret value
    accessResp, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
        Name: secret.Name + "/versions/latest",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Secret value: %s\n", string(accessResp.Payload.Data))

    // 4. Add another version
    version2, err := client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
        Parent: secret.Name,
        Payload: &secretmanagerpb.SecretPayload{
            Data: []byte("my-updated-password-456"),
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Added version: %s\n", version2.Name)

    // 5. Access latest (should be version 2)
    accessResp, err = client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
        Name: secret.Name + "/versions/latest",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Latest value: %s\n", string(accessResp.Payload.Data))

    // 6. List all secrets
    iter := client.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
        Parent:   "projects/test-project",
        PageSize: 10,
    })

    fmt.Println("\nAll secrets:")
    for {
        secret, err := iter.Next()
        if err == iterator.Done {
            break
        }
        if err != nil {
            log.Fatal(err)
        }
        fmt.Printf("  - %s (labels: %v)\n", secret.Name, secret.Labels)
    }

    // 7. Delete the secret
    err = client.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{
        Name: secret.Name,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("\nDeleted secret")
}
```

**Output:**
```
Created secret: projects/test-project/secrets/database-password
Added version: projects/test-project/secrets/database-password/versions/1
Secret value: my-database-password-123
Added version: projects/test-project/secrets/database-password/versions/2
Latest value: my-updated-password-456

All secrets:
  - projects/test-project/secrets/database-password (labels: map[app:myapp env:production])

Deleted secret
```

## Pagination

List operations support pagination for large result sets.

**Default Behavior:**
- Page size: 100 (if not specified)
- Returns `next_page_token` if more results available
- Empty token means last page

**Example:**
```go
// Manual pagination
pageToken := ""
for {
    resp, err := client.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
        Parent:    "projects/test-project",
        PageSize:  10,
        PageToken: pageToken,
    })
    if err != nil {
        log.Fatal(err)
    }

    for _, secret := range resp.Secrets {
        fmt.Println(secret.Name)
    }

    if resp.NextPageToken == "" {
        break // No more pages
    }
    pageToken = resp.NextPageToken
}
```

## Version Management

### Version States

```
ENABLED    - Version is active and accessible (default)
DISABLED   - Version exists but cannot be accessed (not supported)
DESTROYED  - Version permanently deleted (not supported)
```

**Emulator Behavior:**
- All versions are created as `ENABLED`
- State changes not supported (Enable/Disable/Destroy methods unimplemented)
- Versions can only be removed by deleting the entire secret

### Latest Version Resolution

The special alias `versions/latest` resolves to the highest-numbered ENABLED version:

```go
// These are equivalent if version 3 is the highest ENABLED version:
AccessSecretVersion("projects/p/secrets/s/versions/latest")
AccessSecretVersion("projects/p/secrets/s/versions/3")
```

**Algorithm:**
1. Get all versions for the secret
2. Filter to only ENABLED versions
3. Find version with highest numeric ID
4. Return that version's payload

**Edge Cases:**
- If no ENABLED versions exist: Returns `NotFound`
- If secret has no versions: Returns `NotFound`

## Metadata

### Labels

Key-value pairs for organizing and filtering secrets.

**Format:**
- Keys: Lowercase alphanumeric + hyphens/underscores
- Values: Any string
- Max 64 labels per secret

**Example:**
```go
secret := &secretmanagerpb.Secret{
    Labels: map[string]string{
        "environment": "production",
        "app":         "web-server",
        "team":        "platform",
        "cost-center": "engineering",
    },
}
```

### Annotations

Similar to labels but not indexed (in real GCP). The emulator treats them identically.

**Use Case:** Store additional metadata that doesn't need filtering.

### Replication

Specifies how secret data is replicated across regions.

**Emulator Behavior:**
- Accepts any replication configuration
- Does not enforce replication (in-memory storage only)
- Defaults to `Automatic` if not specified

**Example:**
```go
replication := &secretmanagerpb.Replication{
    Replication: &secretmanagerpb.Replication_Automatic_{
        Automatic: &secretmanagerpb.Replication_Automatic{},
    },
}
```

## Differences from Real GCP

### Intentional Simplifications

| Feature | Real GCP | Emulator |
|---------|----------|----------|
| Authentication | IAM, service accounts | None (all requests succeed) |
| Authorization | IAM policies | None (no permission checks) |
| Encryption | KMS, customer keys | None (in-memory plaintext) |
| Replication | Multi-region | None (single in-memory store) |
| Persistence | Durable storage | None (data lost on restart) |
| Version lifecycle | Enable/Disable/Destroy | All versions always ENABLED |
| IAM methods | Full support | Not implemented |
| Audit logging | Cloud Logging | None |
| Quotas | API rate limits | None (unlimited) |
| Billing | Per-operation costs | None (free) |

### What's Identical

| Feature | Behavior |
|---------|----------|
| gRPC API signatures | Exactly matches official API |
| Resource naming | Same format (projects/*/secrets/*) |
| Error codes | Same gRPC status codes |
| Pagination | Same token-based pagination |
| Version numbering | Auto-incrementing integers |
| "latest" alias | Resolves to highest ENABLED version |
| Labels/Annotations | Same metadata structure |

## Testing Strategies

### Unit Testing

Test your application logic without network calls:

```go
func TestSecretRetrieval(t *testing.T) {
    // Start emulator in-process for tests
    server := server.NewServer()

    // Create test secret
    server.Storage().CreateSecret(ctx, "projects/test", "my-secret", &secretmanagerpb.Secret{
        Replication: &secretmanagerpb.Replication{
            Replication: &secretmanagerpb.Replication_Automatic_{
                Automatic: &secretmanagerpb.Replication_Automatic{},
            },
        },
    })

    // Add version
    server.Storage().AddSecretVersion(ctx, "projects/test/secrets/my-secret", &secretmanagerpb.SecretPayload{
        Data: []byte("test-value"),
    })

    // Now test your application code against the emulator
}
```

### Integration Testing

Test complete workflows including gRPC communication:

```go
func TestIntegration(t *testing.T) {
    // Connect to running emulator
    ctx := context.Background()
    conn, err := grpc.NewClient("localhost:9090",
        grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        t.Fatal(err)
    }

    client, err := secretmanager.NewClient(ctx, option.WithGRPCConn(conn))
    if err != nil {
        t.Fatal(err)
    }
    defer client.Close()

    // Run full workflow tests
    testSecretLifecycle(t, client)
}
```

### CI/CD Testing

```yaml
- name: Install GCP Secret Manager emulator
  run: go install github.com/blackwell-systems/gcp-secret-manager-emulator/cmd/server@latest

- name: Start emulator
  run: server --port 9090 &

- name: Run integration tests
  env:
    GCP_MOCK_ENDPOINT: localhost:9090
  run: go test -v ./...
```

## Configuration

### Server Options

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--port` | `GCP_MOCK_PORT` | `9090` | gRPC port to listen on |
| `--log-level` | `GCP_MOCK_LOG_LEVEL` | `info` | Log level: debug, info, warn, error |

### Example:

```bash
# Custom port
server --port 8080

# Debug logging
server --log-level debug

# Environment variables
export GCP_MOCK_PORT=8080
export GCP_MOCK_LOG_LEVEL=debug
server
```

## Troubleshooting

### Connection Refused

**Problem:** Client can't connect to emulator

**Check:**
```bash
# Verify emulator is running
ps aux | grep server

# Check port is listening
lsof -i :9090  # Unix
netstat -an | grep 9090  # Windows
```

**Solution:** Start emulator before running tests

---

### NotFound Errors

**Problem:** `AccessSecretVersion` returns NotFound for "latest"

**Cause:** Secret has no versions yet

**Solution:** Call `AddSecretVersion` before accessing:
```go
// 1. Create secret (metadata only)
client.CreateSecret(...)

// 2. Add version with payload (REQUIRED)
client.AddSecretVersion(...)

// 3. Now access works
client.AccessSecretVersion(..., "versions/latest")
```

---

### Unimplemented Errors

**Problem:** Method returns Unimplemented

**Cause:** Method not needed for common testing scenarios

**Solution:** Either:
1. Adjust your code to not use that method
2. Contribute implementation to the emulator
3. Use real GCP for advanced features

---

## Contributing

To add support for unimplemented methods:

1. Add method to `internal/server/server.go`
2. Add storage implementation to `internal/server/storage.go`
3. Add tests to `internal/server/server_test.go`
4. Update this API documentation
5. Submit PR to: https://github.com/blackwell-systems/gcp-secret-manager-emulator

Most needed: `ListSecretVersions`, `UpdateSecret`

## References

- [Official GCP Secret Manager API](https://cloud.google.com/secret-manager/docs/reference/rpc)
- [Protocol Buffer Definitions](https://github.com/googleapis/googleapis/blob/master/google/cloud/secretmanager/v1/service.proto)
- [Go Client Library](https://pkg.go.dev/cloud.google.com/go/secretmanager/apiv1)
- [Python Client Library](https://googleapis.dev/python/secretmanager/latest/)
