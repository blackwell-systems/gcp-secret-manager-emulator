package server

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestStorage_CreateSecret(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		secret, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{
			Labels: map[string]string{"env": "test"},
		})

		if err != nil {
			t.Fatalf("CreateSecret() error = %v", err)
		}

		if secret.Name != "projects/test-project/secrets/my-secret" {
			t.Errorf("Secret.Name = %s, want projects/test-project/secrets/my-secret", secret.Name)
		}

		if secret.Labels["env"] != "test" {
			t.Errorf("Secret.Labels[env] = %s, want test", secret.Labels["env"])
		}

		if secret.CreateTime == nil {
			t.Error("Secret.CreateTime is nil, want non-nil")
		}
	})

	t.Run("AlreadyExists", func(t *testing.T) {
		_, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{})

		if err == nil {
			t.Fatal("CreateSecret() duplicate should return error")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.AlreadyExists {
			t.Errorf("CreateSecret() error code = %v, want AlreadyExists", st.Code())
		}
	})
}

func TestStorage_GetSecret(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	// Create a secret first
	_, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{
		Labels: map[string]string{"key": "value"},
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("Success", func(t *testing.T) {
		secret, err := storage.GetSecret(ctx, "projects/test-project/secrets/my-secret")
		if err != nil {
			t.Fatalf("GetSecret() error = %v", err)
		}

		if secret.Name != "projects/test-project/secrets/my-secret" {
			t.Errorf("Secret.Name = %s, want projects/test-project/secrets/my-secret", secret.Name)
		}

		if secret.Labels["key"] != "value" {
			t.Errorf("Secret.Labels[key] = %s, want value", secret.Labels["key"])
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := storage.GetSecret(ctx, "projects/test-project/secrets/nonexistent")
		if err == nil {
			t.Fatal("GetSecret() should return error for nonexistent secret")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("GetSecret() error code = %v, want NotFound", st.Code())
		}
	})
}

func TestStorage_ListSecrets(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	// Create multiple secrets
	for i := 1; i <= 5; i++ {
		secretID := fmt.Sprintf("secret-%d", i)
		_, err := storage.CreateSecret(ctx, "projects/test-project", secretID, &secretmanagerpb.Secret{})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
	}

	t.Run("ListAll", func(t *testing.T) {
		secrets, nextToken, err := storage.ListSecrets(ctx, "projects/test-project", 100, "")
		if err != nil {
			t.Fatalf("ListSecrets() error = %v", err)
		}

		if len(secrets) != 5 {
			t.Errorf("ListSecrets() returned %d secrets, want 5", len(secrets))
		}

		if nextToken != "" {
			t.Errorf("ListSecrets() nextToken = %s, want empty", nextToken)
		}
	})

	t.Run("Pagination", func(t *testing.T) {
		// First page
		secrets, nextToken, err := storage.ListSecrets(ctx, "projects/test-project", 2, "")
		if err != nil {
			t.Fatalf("ListSecrets() error = %v", err)
		}

		if len(secrets) != 2 {
			t.Errorf("ListSecrets() page 1 returned %d secrets, want 2", len(secrets))
		}

		if nextToken == "" {
			t.Error("ListSecrets() page 1 nextToken is empty, want non-empty")
		}

		// Second page
		secrets, nextToken, err = storage.ListSecrets(ctx, "projects/test-project", 2, nextToken)
		if err != nil {
			t.Fatalf("ListSecrets() page 2 error = %v", err)
		}

		if len(secrets) != 2 {
			t.Errorf("ListSecrets() page 2 returned %d secrets, want 2", len(secrets))
		}

		if nextToken == "" {
			t.Error("ListSecrets() page 2 nextToken is empty, want non-empty (more pages available)")
		}

		// Third page (last page with 1 secret)
		secrets, nextToken, err = storage.ListSecrets(ctx, "projects/test-project", 2, nextToken)
		if err != nil {
			t.Fatalf("ListSecrets() page 3 error = %v", err)
		}

		if len(secrets) != 1 {
			t.Errorf("ListSecrets() page 3 returned %d secrets, want 1", len(secrets))
		}

		if nextToken != "" {
			t.Errorf("ListSecrets() page 3 nextToken = %s, want empty (last page)", nextToken)
		}
	})
}

func TestStorage_DeleteSecret(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	_, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("Success", func(t *testing.T) {
		err := storage.DeleteSecret(ctx, "projects/test-project/secrets/my-secret")
		if err != nil {
			t.Fatalf("DeleteSecret() error = %v", err)
		}

		// Verify deleted
		_, err = storage.GetSecret(ctx, "projects/test-project/secrets/my-secret")
		if err == nil {
			t.Error("GetSecret() after delete should return error")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		err := storage.DeleteSecret(ctx, "projects/test-project/secrets/nonexistent")
		if err == nil {
			t.Fatal("DeleteSecret() should return error for nonexistent secret")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("DeleteSecret() error code = %v, want NotFound", st.Code())
		}
	})
}

func TestStorage_AddSecretVersion(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	_, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("Success", func(t *testing.T) {
		payload := &secretmanagerpb.SecretPayload{
			Data: []byte("my-secret-data"),
		}

		version, err := storage.AddSecretVersion(ctx, "projects/test-project/secrets/my-secret", payload)
		if err != nil {
			t.Fatalf("AddSecretVersion() error = %v", err)
		}

		if version.Name != "projects/test-project/secrets/my-secret/versions/1" {
			t.Errorf("Version.Name = %s, want projects/test-project/secrets/my-secret/versions/1", version.Name)
		}

		if version.State != secretmanagerpb.SecretVersion_ENABLED {
			t.Errorf("Version.State = %v, want ENABLED", version.State)
		}
	})

	t.Run("MultipleVersions", func(t *testing.T) {
		// Add second version
		payload2 := &secretmanagerpb.SecretPayload{
			Data: []byte("updated-secret-data"),
		}

		version2, err := storage.AddSecretVersion(ctx, "projects/test-project/secrets/my-secret", payload2)
		if err != nil {
			t.Fatalf("AddSecretVersion() error = %v", err)
		}

		if version2.Name != "projects/test-project/secrets/my-secret/versions/2" {
			t.Errorf("Version2.Name = %s, want version 2", version2.Name)
		}
	})

	t.Run("SecretNotFound", func(t *testing.T) {
		payload := &secretmanagerpb.SecretPayload{Data: []byte("data")}
		_, err := storage.AddSecretVersion(ctx, "projects/test-project/secrets/nonexistent", payload)

		if err == nil {
			t.Fatal("AddSecretVersion() should return error for nonexistent secret")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("AddSecretVersion() error code = %v, want NotFound", st.Code())
		}
	})
}

func TestStorage_AccessSecretVersion(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	// Setup: Create secret with two versions
	_, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	_, err = storage.AddSecretVersion(ctx, "projects/test-project/secrets/my-secret", &secretmanagerpb.SecretPayload{
		Data: []byte("version-1-data"),
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	_, err = storage.AddSecretVersion(ctx, "projects/test-project/secrets/my-secret", &secretmanagerpb.SecretPayload{
		Data: []byte("version-2-data"),
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("SpecificVersion", func(t *testing.T) {
		response, err := storage.AccessSecretVersion(ctx, "projects/test-project/secrets/my-secret/versions/1")
		if err != nil {
			t.Fatalf("AccessSecretVersion() error = %v", err)
		}

		if string(response.Payload.Data) != "version-1-data" {
			t.Errorf("Payload.Data = %s, want version-1-data", string(response.Payload.Data))
		}
	})

	t.Run("LatestVersion", func(t *testing.T) {
		response, err := storage.AccessSecretVersion(ctx, "projects/test-project/secrets/my-secret/versions/latest")
		if err != nil {
			t.Fatalf("AccessSecretVersion() error = %v", err)
		}

		// Should resolve to version 2 (latest)
		if string(response.Payload.Data) != "version-2-data" {
			t.Errorf("Payload.Data = %s, want version-2-data", string(response.Payload.Data))
		}
	})

	t.Run("VersionNotFound", func(t *testing.T) {
		_, err := storage.AccessSecretVersion(ctx, "projects/test-project/secrets/my-secret/versions/999")

		if err == nil {
			t.Fatal("AccessSecretVersion() should return error for nonexistent version")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("AccessSecretVersion() error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("SecretNotFound", func(t *testing.T) {
		_, err := storage.AccessSecretVersion(ctx, "projects/test-project/secrets/nonexistent/versions/1")

		if err == nil {
			t.Fatal("AccessSecretVersion() should return error for nonexistent secret")
		}

		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("AccessSecretVersion() error code = %v, want NotFound", st.Code())
		}
	})
}

func TestStorage_Concurrent(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	// Test concurrent operations (like vaultmux status cache tests)
	const numGoroutines = 100

	t.Run("ConcurrentCreateAndRead", func(t *testing.T) {
		done := make(chan bool, numGoroutines)

		// Concurrent creates
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				secretID := fmt.Sprintf("secret-%d", id)
				_, err := storage.CreateSecret(ctx, "projects/concurrent-test", secretID, &secretmanagerpb.Secret{})
				if err != nil {
					t.Errorf("Concurrent CreateSecret() failed: %v", err)
				}
				done <- true
			}(i)
		}

		// Wait for all creates
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Verify all secrets exist
		secrets, _, err := storage.ListSecrets(ctx, "projects/concurrent-test", 1000, "")
		if err != nil {
			t.Fatalf("ListSecrets() error = %v", err)
		}

		if len(secrets) != numGoroutines {
			t.Errorf("Created %d secrets, but found %d", numGoroutines, len(secrets))
		}
	})
}

func TestListSecrets_DeterministicOrdering(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	// Add secrets with names that would sort differently than insertion order
	secretNames := []string{"z-secret", "a-secret", "m-secret", "b-secret", "n-secret"}
	for _, name := range secretNames {
		_, err := storage.CreateSecret(ctx, "projects/ordering-test", name, &secretmanagerpb.Secret{})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
	}

	// Call ListSecrets twice with no pageToken
	secrets1, _, err := storage.ListSecrets(ctx, "projects/ordering-test", 100, "")
	if err != nil {
		t.Fatalf("ListSecrets() first call error = %v", err)
	}

	secrets2, _, err := storage.ListSecrets(ctx, "projects/ordering-test", 100, "")
	if err != nil {
		t.Fatalf("ListSecrets() second call error = %v", err)
	}

	if len(secrets1) != 5 {
		t.Fatalf("Expected 5 secrets, got %d", len(secrets1))
	}

	// Assert both calls return secrets in the same order
	for i := range secrets1 {
		if secrets1[i].Name != secrets2[i].Name {
			t.Errorf("Call 1 and call 2 differ at index %d: %s vs %s", i, secrets1[i].Name, secrets2[i].Name)
		}
	}

	// Assert first result is lexicographically smallest
	expectedFirst := "projects/ordering-test/secrets/a-secret"
	if secrets1[0].Name != expectedFirst {
		t.Errorf("First secret = %s, want %s", secrets1[0].Name, expectedFirst)
	}
}

func TestListSecretVersions_NumericOrdering(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	_, err := storage.CreateSecret(ctx, "projects/test-project", "versioned-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	secretName := "projects/test-project/secrets/versioned-secret"

	// Add 12 versions to ensure version numbers >= 10 exist
	for i := 0; i < 12; i++ {
		payload := &secretmanagerpb.SecretPayload{
			Data: []byte(fmt.Sprintf("version-%d-data", i+1)),
		}
		_, err := storage.AddSecretVersion(ctx, secretName, payload)
		if err != nil {
			t.Fatalf("AddSecretVersion() failed at iteration %d: %v", i+1, err)
		}
	}

	versions, _, err := storage.ListSecretVersions(ctx, secretName, 100, "", "")
	if err != nil {
		t.Fatalf("ListSecretVersions() error = %v", err)
	}

	if len(versions) != 12 {
		t.Fatalf("Expected 12 versions, got %d", len(versions))
	}

	// Assert versions are returned in numeric order: 1, 2, 3, ..., 10, 11, 12
	for i, v := range versions {
		expectedVersionNum := i + 1
		expectedName := fmt.Sprintf("%s/versions/%d", secretName, expectedVersionNum)
		if v.Name != expectedName {
			t.Errorf("Version at index %d: got %s, want %s", i, v.Name, expectedName)
		}
	}
}

func TestStorage_ClearAndCount(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	// Create some secrets
	for i := 1; i <= 3; i++ {
		_, err := storage.CreateSecret(ctx, "projects/test", fmt.Sprintf("secret-%d", i), &secretmanagerpb.Secret{})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
	}

	if count := storage.SecretCount(); count != 3 {
		t.Errorf("SecretCount() = %d, want 3", count)
	}

	storage.Clear()

	if count := storage.SecretCount(); count != 0 {
		t.Errorf("SecretCount() after Clear() = %d, want 0", count)
	}
}
