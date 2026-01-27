package server

import (
	"context"
	"fmt"
	"net"
	"testing"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestIntegration tests the mock server with the real GCP SDK client.
// This verifies that our mock behaves correctly from a client perspective.
func TestIntegration(t *testing.T) {
	// Start mock server
	server, addr := startTestServer(t)
	defer server.GracefulStop()

	// Create real GCP client pointing to mock
	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx,
		option.WithEndpoint(addr),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Logf("Failed to close client: %v", err)
		}
	}()

	projectID := "test-project"
	parent := fmt.Sprintf("projects/%s", projectID)
	secretID := "integration-test-secret"

	// Test 1: Create secret
	t.Run("CreateSecret", func(t *testing.T) {
		req := &secretmanagerpb.CreateSecretRequest{
			Parent:   parent,
			SecretId: secretID,
			Secret: &secretmanagerpb.Secret{
				Labels: map[string]string{
					"test": "integration",
				},
			},
		}

		secret, err := client.CreateSecret(ctx, req)
		if err != nil {
			t.Fatalf("CreateSecret() error = %v", err)
		}

		expectedName := fmt.Sprintf("%s/secrets/%s", parent, secretID)
		if secret.Name != expectedName {
			t.Errorf("Secret.Name = %s, want %s", secret.Name, expectedName)
		}

		if secret.Labels["test"] != "integration" {
			t.Errorf("Secret.Labels[test] = %s, want integration", secret.Labels["test"])
		}
	})

	// Test 2: Get secret metadata
	t.Run("GetSecret", func(t *testing.T) {
		secretName := fmt.Sprintf("%s/secrets/%s", parent, secretID)
		req := &secretmanagerpb.GetSecretRequest{
			Name: secretName,
		}

		secret, err := client.GetSecret(ctx, req)
		if err != nil {
			t.Fatalf("GetSecret() error = %v", err)
		}

		if secret.Name != secretName {
			t.Errorf("Secret.Name = %s, want %s", secret.Name, secretName)
		}
	})

	// Test 3: Add secret version (first version)
	t.Run("AddSecretVersion", func(t *testing.T) {
		secretName := fmt.Sprintf("%s/secrets/%s", parent, secretID)
		req := &secretmanagerpb.AddSecretVersionRequest{
			Parent: secretName,
			Payload: &secretmanagerpb.SecretPayload{
				Data: []byte("my-secret-data"),
			},
		}

		version, err := client.AddSecretVersion(ctx, req)
		if err != nil {
			t.Fatalf("AddSecretVersion() error = %v", err)
		}

		expectedName := fmt.Sprintf("%s/versions/1", secretName)
		if version.Name != expectedName {
			t.Errorf("Version.Name = %s, want %s", version.Name, expectedName)
		}

		if version.State != secretmanagerpb.SecretVersion_ENABLED {
			t.Errorf("Version.State = %v, want ENABLED", version.State)
		}
	})

	// Test 4: Access secret version (specific version)
	t.Run("AccessSpecificVersion", func(t *testing.T) {
		versionName := fmt.Sprintf("%s/secrets/%s/versions/1", parent, secretID)
		req := &secretmanagerpb.AccessSecretVersionRequest{
			Name: versionName,
		}

		response, err := client.AccessSecretVersion(ctx, req)
		if err != nil {
			t.Fatalf("AccessSecretVersion() error = %v", err)
		}

		if string(response.Payload.Data) != "my-secret-data" {
			t.Errorf("Payload.Data = %s, want my-secret-data", string(response.Payload.Data))
		}
	})

	// Test 5: Add second version
	t.Run("AddSecondVersion", func(t *testing.T) {
		secretName := fmt.Sprintf("%s/secrets/%s", parent, secretID)
		req := &secretmanagerpb.AddSecretVersionRequest{
			Parent: secretName,
			Payload: &secretmanagerpb.SecretPayload{
				Data: []byte("updated-secret-data"),
			},
		}

		version, err := client.AddSecretVersion(ctx, req)
		if err != nil {
			t.Fatalf("AddSecretVersion() error = %v", err)
		}

		expectedName := fmt.Sprintf("%s/versions/2", secretName)
		if version.Name != expectedName {
			t.Errorf("Version.Name = %s, want %s", version.Name, expectedName)
		}
	})

	// Test 6: Access latest version (should be version 2)
	t.Run("AccessLatestVersion", func(t *testing.T) {
		versionName := fmt.Sprintf("%s/secrets/%s/versions/latest", parent, secretID)
		req := &secretmanagerpb.AccessSecretVersionRequest{
			Name: versionName,
		}

		response, err := client.AccessSecretVersion(ctx, req)
		if err != nil {
			t.Fatalf("AccessSecretVersion() error = %v", err)
		}

		if string(response.Payload.Data) != "updated-secret-data" {
			t.Errorf("Payload.Data = %s, want updated-secret-data (version 2)", string(response.Payload.Data))
		}
	})

	// Test 7: List secrets
	t.Run("ListSecrets", func(t *testing.T) {
		req := &secretmanagerpb.ListSecretsRequest{
			Parent: parent,
		}

		iter := client.ListSecrets(ctx, req)
		found := false
		for {
			secret, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				t.Fatalf("ListSecrets() error = %v", err)
			}

			expectedName := fmt.Sprintf("%s/secrets/%s", parent, secretID)
			if secret.Name == expectedName {
				found = true
			}
		}

		if !found {
			t.Error("ListSecrets() did not include our secret")
		}
	})

	// Test 8: Delete secret
	t.Run("DeleteSecret", func(t *testing.T) {
		secretName := fmt.Sprintf("%s/secrets/%s", parent, secretID)
		req := &secretmanagerpb.DeleteSecretRequest{
			Name: secretName,
		}

		err := client.DeleteSecret(ctx, req)
		if err != nil {
			t.Fatalf("DeleteSecret() error = %v", err)
		}

		// Verify deleted
		getReq := &secretmanagerpb.GetSecretRequest{Name: secretName}
		_, err = client.GetSecret(ctx, getReq)
		if err == nil {
			t.Error("GetSecret() after delete should return error")
		}
	})
}

// TestIntegration_Pagination tests pagination with the real client.
func TestIntegration_Pagination(t *testing.T) {
	server, addr := startTestServer(t)
	defer server.GracefulStop()

	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx,
		option.WithEndpoint(addr),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
	)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Logf("Failed to close client: %v", err)
		}
	}()

	// Create 5 secrets
	parent := "projects/pagination-test"
	for i := 1; i <= 5; i++ {
		secretID := fmt.Sprintf("secret-%d", i)
		req := &secretmanagerpb.CreateSecretRequest{
			Parent:   parent,
			SecretId: secretID,
			Secret:   &secretmanagerpb.Secret{},
		}
		_, err := client.CreateSecret(ctx, req)
		if err != nil {
			t.Fatalf("CreateSecret() error = %v", err)
		}
	}

	// List with pagination
	req := &secretmanagerpb.ListSecretsRequest{
		Parent:   parent,
		PageSize: 2, // Small page size to test pagination
	}

	iter := client.ListSecrets(ctx, req)
	count := 0
	for {
		_, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			t.Fatalf("ListSecrets() error = %v", err)
		}
		count++
	}

	if count != 5 {
		t.Errorf("ListSecrets() returned %d secrets, want 5", count)
	}
}

// startTestServer starts a mock gRPC server for testing.
// Returns the server and its address.
func startTestServer(t *testing.T) (*grpc.Server, string) {
	// Create listener on random port
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register mock service
	mockServer, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	secretmanagerpb.RegisterSecretManagerServiceServer(grpcServer, mockServer)

	// Start serving in background
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Logf("Server error: %v", err)
		}
	}()

	return grpcServer, lis.Addr().String()
}
