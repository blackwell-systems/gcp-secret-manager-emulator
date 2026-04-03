package server

import (
	"context"
	"fmt"
	"hash/crc32"
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
		secrets, nextToken, _, err := storage.ListSecrets(ctx, "projects/test-project", 100, "", "")
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
		secrets, nextToken, _, err := storage.ListSecrets(ctx, "projects/test-project", 2, "", "")
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
		secrets, nextToken, _, err = storage.ListSecrets(ctx, "projects/test-project", 2, nextToken, "")
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
		secrets, nextToken, _, err = storage.ListSecrets(ctx, "projects/test-project", 2, nextToken, "")
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
		secrets, _, _, err := storage.ListSecrets(ctx, "projects/concurrent-test", 1000, "", "")
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
	secrets1, _, _, err := storage.ListSecrets(ctx, "projects/ordering-test", 100, "", "")
	if err != nil {
		t.Fatalf("ListSecrets() first call error = %v", err)
	}

	secrets2, _, _, err := storage.ListSecrets(ctx, "projects/ordering-test", 100, "", "")
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

	// Assert the last-created secret (n-secret) appears first (descending CreateTime order)
	expectedFirst := "projects/ordering-test/secrets/n-secret"
	if secrets1[0].Name != expectedFirst {
		t.Errorf("First secret = %s, want %s (descending create time order)", secrets1[0].Name, expectedFirst)
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

	versions, _, _, err := storage.ListSecretVersions(ctx, secretName, 100, "", "")
	if err != nil {
		t.Fatalf("ListSecretVersions() error = %v", err)
	}

	if len(versions) != 12 {
		t.Fatalf("Expected 12 versions, got %d", len(versions))
	}

	// Assert versions are returned in descending numeric order: 12, 11, 10, ..., 2, 1
	for i, v := range versions {
		expectedVersionNum := 12 - i
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

func TestStorage_EtagGeneration(t *testing.T) {
	s := NewStorage()
	ctx := context.Background()

	secret, err := s.CreateSecret(ctx, "projects/p", "s1", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}
	if secret.GetEtag() == "" {
		t.Error("CreateSecret: etag should be non-empty")
	}

	updated, err := s.UpdateSecret(ctx, secret.Name, map[string]string{"k": "v"}, nil, nil)
	if err != nil {
		t.Fatalf("UpdateSecret: %v", err)
	}
	if updated.GetEtag() == "" {
		t.Error("UpdateSecret: etag should be non-empty")
	}
	// etag changes after mutation
	if updated.GetEtag() == secret.GetEtag() {
		t.Errorf("UpdateSecret: etag should change after mutation, got same etag %q", secret.GetEtag())
	}
}

func TestStorage_DestroyTime(t *testing.T) {
	s := NewStorage()
	ctx := context.Background()

	_, err := s.CreateSecret(ctx, "projects/p", "s-destroy", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}
	parent := "projects/p/secrets/s-destroy"
	_, err = s.AddSecretVersion(ctx, parent, &secretmanagerpb.SecretPayload{Data: []byte("data")})
	if err != nil {
		t.Fatalf("AddSecretVersion: %v", err)
	}
	versionName := parent + "/versions/1"

	destroyed, err := s.DestroySecretVersion(ctx, versionName)
	if err != nil {
		t.Fatalf("DestroySecretVersion: %v", err)
	}
	if destroyed.GetDestroyTime() == nil {
		t.Error("DestroySecretVersion: destroy_time should be set")
	}

	// Idempotent destroy should also return destroy_time
	destroyed2, err := s.DestroySecretVersion(ctx, versionName)
	if err != nil {
		t.Fatalf("Idempotent DestroySecretVersion: %v", err)
	}
	if destroyed2.GetDestroyTime() == nil {
		t.Error("Idempotent DestroySecretVersion: destroy_time should be set")
	}
}

func TestStorage_DataCrc32C(t *testing.T) {
	s := NewStorage()
	ctx := context.Background()

	_, err := s.CreateSecret(ctx, "projects/p", "s-crc", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}
	parent := "projects/p/secrets/s-crc"
	data := []byte("hello world")
	table := crc32.MakeTable(crc32.Castagnoli)
	checksum := int64(crc32.Checksum(data, table))

	_, err = s.AddSecretVersion(ctx, parent, &secretmanagerpb.SecretPayload{
		Data:       data,
		DataCrc32C: &checksum,
	})
	if err != nil {
		t.Fatalf("AddSecretVersion: %v", err)
	}

	resp, err := s.AccessSecretVersion(ctx, parent+"/versions/1")
	if err != nil {
		t.Fatalf("AccessSecretVersion: %v", err)
	}
	if resp.GetPayload().DataCrc32C == nil {
		t.Fatal("AccessSecretVersion: data_crc32c should be non-nil")
	}
	if resp.GetPayload().GetDataCrc32C() != checksum {
		t.Errorf("AccessSecretVersion: data_crc32c = %d, want %d",
			resp.GetPayload().GetDataCrc32C(), checksum)
	}
}

func TestStorage_VersionAliases(t *testing.T) {
	s := NewStorage()
	ctx := context.Background()

	_, err := s.CreateSecret(ctx, "projects/p", "s-alias", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}
	parent := "projects/p/secrets/s-alias"
	_, err = s.AddSecretVersion(ctx, parent, &secretmanagerpb.SecretPayload{Data: []byte("v1data")})
	if err != nil {
		t.Fatalf("AddSecretVersion: %v", err)
	}

	// Set version_aliases via UpdateSecret
	_, err = s.UpdateSecret(ctx, parent, nil, nil, map[string]int64{"myalias": 1})
	if err != nil {
		t.Fatalf("UpdateSecret with aliases: %v", err)
	}

	// GetSecretVersion via alias
	sv, err := s.GetSecretVersion(ctx, parent+"/versions/myalias")
	if err != nil {
		t.Fatalf("GetSecretVersion(myalias): %v", err)
	}
	if sv.GetName() != parent+"/versions/1" {
		t.Errorf("GetSecretVersion alias: name = %q, want %q", sv.GetName(), parent+"/versions/1")
	}

	// AccessSecretVersion via alias
	resp, err := s.AccessSecretVersion(ctx, parent+"/versions/myalias")
	if err != nil {
		t.Fatalf("AccessSecretVersion(myalias): %v", err)
	}
	if string(resp.GetPayload().GetData()) != "v1data" {
		t.Errorf("AccessSecretVersion alias: data = %q, want v1data", resp.GetPayload().GetData())
	}
}

func TestStorage_ListSecretsFilter(t *testing.T) {
	s := NewStorage()
	ctx := context.Background()

	for _, id := range []string{"foo-1", "foo-2", "bar-1"} {
		labels := map[string]string{}
		if id == "foo-1" {
			labels["env"] = "prod"
		}
		_, err := s.CreateSecret(ctx, "projects/p", id, &secretmanagerpb.Secret{Labels: labels})
		if err != nil {
			t.Fatalf("CreateSecret(%s): %v", id, err)
		}
	}

	tests := []struct {
		filter string
		want   int
	}{
		{"name:foo", 2},
		{"name:bar", 1},
		{"name:baz", 0},
		{"labels.env=prod", 1},
		{"", 3},
	}
	for _, tt := range tests {
		secrets, _, _, err := s.ListSecrets(ctx, "projects/p", 100, "", tt.filter)
		if err != nil {
			t.Fatalf("ListSecrets(filter=%q): %v", tt.filter, err)
		}
		if len(secrets) != tt.want {
			t.Errorf("ListSecrets(filter=%q): got %d secrets, want %d", tt.filter, len(secrets), tt.want)
		}
	}
}

func TestStorage_TotalSize(t *testing.T) {
	s := NewStorage()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := s.CreateSecret(ctx, "projects/p", fmt.Sprintf("secret-%d", i), &secretmanagerpb.Secret{})
		if err != nil {
			t.Fatalf("CreateSecret: %v", err)
		}
	}

	// Page size 2, should have total_size = 5
	_, _, total, err := s.ListSecrets(ctx, "projects/p", 2, "", "")
	if err != nil {
		t.Fatalf("ListSecrets: %v", err)
	}
	if total != 5 {
		t.Errorf("ListSecrets TotalSize = %d, want 5", total)
	}

	// ListSecretVersions total_size
	parent := "projects/p/secrets/secret-0"
	for i := 0; i < 4; i++ {
		_, err := s.AddSecretVersion(ctx, parent, &secretmanagerpb.SecretPayload{Data: []byte("x")})
		if err != nil {
			t.Fatalf("AddSecretVersion: %v", err)
		}
	}
	_, _, vTotal, err := s.ListSecretVersions(ctx, parent, 2, "", "")
	if err != nil {
		t.Fatalf("ListSecretVersions: %v", err)
	}
	if vTotal != 4 {
		t.Errorf("ListSecretVersions TotalSize = %d, want 4", vTotal)
	}
}

func TestStorage_VersionEtag(t *testing.T) {
	s := NewStorage()
	ctx := context.Background()

	_, err := s.CreateSecret(ctx, "projects/p", "s-vetag", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}
	parent := "projects/p/secrets/s-vetag"
	sv, err := s.AddSecretVersion(ctx, parent, &secretmanagerpb.SecretPayload{Data: []byte("data")})
	if err != nil {
		t.Fatalf("AddSecretVersion: %v", err)
	}
	if sv.GetEtag() == "" {
		t.Error("AddSecretVersion: etag should be non-empty")
	}
	origEtag := sv.GetEtag()

	disabled, err := s.DisableSecretVersion(ctx, parent+"/versions/1")
	if err != nil {
		t.Fatalf("DisableSecretVersion: %v", err)
	}
	if disabled.GetEtag() == "" {
		t.Error("DisableSecretVersion: etag should be non-empty")
	}
	if disabled.GetEtag() == origEtag {
		t.Errorf("DisableSecretVersion: etag should change after state mutation, still %q", origEtag)
	}
}
