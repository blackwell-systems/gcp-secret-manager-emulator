// Package gcpemulator provides the reference local implementation of the Google Cloud Secret Manager API.
//
// This package delivers production-grade, behaviorally-accurate Secret Manager semantics for
// local development and CI/CD without requiring GCP credentials, network connectivity, or billing.
//
// # Features
//
//   - Full gRPC API implementation compatible with cloud.google.com/go/secretmanager client
//   - No authentication required - works entirely offline
//   - In-memory storage with thread-safe operations
//   - Supports secrets, secret versions, labels, and pagination
//   - Docker container available for CI/CD integration
//
// # Quick Start
//
// Start the emulator server:
//
//	go run github.com/blackwell-systems/gcp-secret-manager-emulator/cmd/server@latest
//
// Connect your application to the emulator:
//
//	ctx := context.Background()
//	conn, err := grpc.NewClient(
//	    "localhost:9090",
//	    grpc.WithTransportCredentials(insecure.NewCredentials()),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	client, err := secretmanager.NewClient(ctx, option.WithGRPCConn(conn))
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Use client exactly as you would with real GCP
//	secret, err := client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
//	    Parent:   "projects/test-project",
//	    SecretId: "my-secret",
//	    Secret: &secretmanagerpb.Secret{
//	        Replication: &secretmanagerpb.Replication{
//	            Replication: &secretmanagerpb.Replication_Automatic_{
//	                Automatic: &secretmanagerpb.Replication_Automatic{},
//	            },
//	        },
//	    },
//	})
//
// # Use Cases
//
//   - Local development without GCP credentials
//   - CI/CD integration testing without cloud costs
//   - Unit testing with deterministic behavior
//   - Offline demos and prototyping
//   - Learning GCP Secret Manager API
//
// # API Coverage
//
// 11 of 12 methods implemented (92% coverage):
//
// Secrets: CreateSecret, GetSecret, UpdateSecret, ListSecrets, DeleteSecret
//
// Versions: AddSecretVersion, GetSecretVersion, AccessSecretVersion, ListSecretVersions,
// EnableSecretVersion, DisableSecretVersion, DestroySecretVersion
//
// Not implemented: IAM methods (SetIamPolicy, GetIamPolicy, TestIamPermissions)
//
// # Architecture
//
// The emulator implements the SecretManagerServiceServer gRPC interface with
// in-memory storage. All operations are thread-safe using sync.RWMutex.
// The server is designed to be embedded in Go tests or run as a standalone
// process for multi-language testing.
//
// See the internal/server package for implementation details.
package gcpemulator
