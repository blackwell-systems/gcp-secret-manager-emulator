package gcpemulator_test

import (
	"context"
	"fmt"
	"log"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Example demonstrates basic usage of the GCP Secret Manager emulator.
// This example shows how to connect to the emulator and perform common operations.
func Example() {
	ctx := context.Background()

	// Connect to emulator (assumes server running on localhost:9090)
	conn, err := grpc.NewClient(
		"localhost:9090",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Create Secret Manager client pointing to emulator
	client, err := secretmanager.NewClient(ctx, option.WithGRPCConn(conn))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Create a secret
	secret, err := client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   "projects/test-project",
		SecretId: "my-api-key",
		Secret: &secretmanagerpb.Secret{
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

	// Add a secret version with payload
	version, err := client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: secret.Name,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte("super-secret-value"),
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Added version: %s\n", version.Name)

	// Access the secret value
	accessResp, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: version.Name,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Secret value: %s\n", string(accessResp.Payload.Data))
}

// Example_listSecrets demonstrates listing secrets with pagination.
func Example_listSecrets() {
	ctx := context.Background()

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

	// List all secrets in project
	it := client.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
		Parent:   "projects/test-project",
		PageSize: 10,
	})

	for {
		secret, err := it.Next()
		if err != nil {
			break
		}
		fmt.Printf("Secret: %s\n", secret.Name)
	}
}

// Example_cicd demonstrates typical CI/CD usage pattern.
func Example_cicd() {
	// This example shows how to use the emulator in CI/CD pipelines
	// where you need to test GCP Secret Manager integration without credentials.

	ctx := context.Background()

	// In your CI/CD environment:
	// 1. Start emulator: docker run -d -p 9090:9090 gcp-secret-manager-emulator
	// 2. Configure your app to use emulator endpoint
	// 3. Run integration tests

	conn, _ := grpc.NewClient(
		"localhost:9090",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	client, _ := secretmanager.NewClient(ctx, option.WithGRPCConn(conn))
	defer client.Close()

	// Your integration tests run against emulator
	// No GCP_PROJECT, GOOGLE_APPLICATION_CREDENTIALS, or billing required
	_, _ = client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   "projects/ci-test-project",
		SecretId: "test-secret",
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	})

	fmt.Println("CI/CD test completed successfully")
}
