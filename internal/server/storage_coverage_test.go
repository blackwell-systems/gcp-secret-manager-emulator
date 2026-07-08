package server

import (
	"context"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestStorage_GetSecretVersion_Coverage fills coverage gaps for GetSecretVersion in storage.
func TestStorage_GetSecretVersion_Coverage(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	// Setup: create secret with two versions
	secretName := "projects/test-project/secrets/my-secret"
	_, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	_, err = storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{Data: []byte("v1")})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	_, err = storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{Data: []byte("v2")})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("SpecificVersion", func(t *testing.T) {
		resp, err := storage.GetSecretVersion(ctx, secretName+"/versions/1")
		if err != nil {
			t.Fatalf("GetSecretVersion() error = %v", err)
		}
		if resp.Name != secretName+"/versions/1" {
			t.Errorf("Name = %q, want %q", resp.Name, secretName+"/versions/1")
		}
		if resp.State != secretmanagerpb.SecretVersion_ENABLED {
			t.Errorf("State = %v, want ENABLED", resp.State)
		}
		if resp.CreateTime == nil {
			t.Error("CreateTime is nil")
		}
	})

	t.Run("LatestAlias", func(t *testing.T) {
		resp, err := storage.GetSecretVersion(ctx, secretName+"/versions/latest")
		if err != nil {
			t.Fatalf("GetSecretVersion(latest) error = %v", err)
		}
		if resp.Name != secretName+"/versions/2" {
			t.Errorf("latest resolved to %q, want version 2", resp.Name)
		}
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		_, err := storage.GetSecretVersion(ctx, "invalid-no-versions-separator")
		if err == nil {
			t.Error("GetSecretVersion() should return error for invalid format")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("error code = %v, want InvalidArgument", st.Code())
		}
	})

	t.Run("SecretNotFound", func(t *testing.T) {
		_, err := storage.GetSecretVersion(ctx, "projects/test-project/secrets/nonexistent/versions/1")
		if err == nil {
			t.Error("GetSecretVersion() should return error for nonexistent secret")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("VersionNotFound", func(t *testing.T) {
		_, err := storage.GetSecretVersion(ctx, secretName+"/versions/999")
		if err == nil {
			t.Error("GetSecretVersion() should return error for nonexistent version")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("DestroyedVersionMetadata", func(t *testing.T) {
		_, err := storage.DestroySecretVersion(ctx, secretName+"/versions/1")
		if err != nil {
			t.Fatalf("DestroySecretVersion() failed: %v", err)
		}
		resp, err := storage.GetSecretVersion(ctx, secretName+"/versions/1")
		if err != nil {
			t.Fatalf("GetSecretVersion() for destroyed version error = %v", err)
		}
		if resp.State != secretmanagerpb.SecretVersion_DESTROYED {
			t.Errorf("State = %v, want DESTROYED", resp.State)
		}
	})

	t.Run("LatestSkipsDisabledVersions", func(t *testing.T) {
		// Version 1 is destroyed, version 2 is enabled; disable version 2
		_, err := storage.DisableSecretVersion(ctx, secretName+"/versions/2")
		if err != nil {
			t.Fatalf("DisableSecretVersion() failed: %v", err)
		}

		// Latest should fail - no enabled versions
		_, err = storage.GetSecretVersion(ctx, secretName+"/versions/latest")
		if err == nil {
			t.Error("GetSecretVersion(latest) should fail when no enabled versions")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})
}

// TestStorage_EnableSecretVersion_Coverage fills coverage gaps for EnableSecretVersion.
func TestStorage_EnableSecretVersion_Coverage(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	secretName := "projects/test-project/secrets/my-secret"
	_, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	_, err = storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{Data: []byte("data")})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("InvalidFormat", func(t *testing.T) {
		_, err := storage.EnableSecretVersion(ctx, "invalid-format")
		if err == nil {
			t.Error("EnableSecretVersion() should return error for invalid format")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("error code = %v, want InvalidArgument", st.Code())
		}
	})

	t.Run("SecretNotFound", func(t *testing.T) {
		_, err := storage.EnableSecretVersion(ctx, "projects/test-project/secrets/nonexistent/versions/1")
		if err == nil {
			t.Error("EnableSecretVersion() should return error for nonexistent secret")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("VersionNotFound", func(t *testing.T) {
		_, err := storage.EnableSecretVersion(ctx, secretName+"/versions/999")
		if err == nil {
			t.Error("EnableSecretVersion() should return error for nonexistent version")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("EnableDestroyedVersion", func(t *testing.T) {
		_, err := storage.DestroySecretVersion(ctx, secretName+"/versions/1")
		if err != nil {
			t.Fatalf("DestroySecretVersion() failed: %v", err)
		}
		_, err = storage.EnableSecretVersion(ctx, secretName+"/versions/1")
		if err == nil {
			t.Error("EnableSecretVersion() should return error for destroyed version")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.FailedPrecondition {
			t.Errorf("error code = %v, want FailedPrecondition", st.Code())
		}
	})

	t.Run("EnableAlreadyEnabledVersion", func(t *testing.T) {
		// Create fresh version
		_, err := storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{Data: []byte("v2")})
		if err != nil {
			t.Fatalf("AddSecretVersion() failed: %v", err)
		}

		// Enable an already enabled version (should succeed as no-op)
		resp, err := storage.EnableSecretVersion(ctx, secretName+"/versions/2")
		if err != nil {
			t.Fatalf("EnableSecretVersion() failed: %v", err)
		}
		if resp.State != secretmanagerpb.SecretVersion_ENABLED {
			t.Errorf("State = %v, want ENABLED", resp.State)
		}
	})
}

// TestStorage_DisableSecretVersion_Coverage fills coverage gaps for DisableSecretVersion.
func TestStorage_DisableSecretVersion_Coverage(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	secretName := "projects/test-project/secrets/my-secret"
	_, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	_, err = storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{Data: []byte("data")})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("InvalidFormat", func(t *testing.T) {
		_, err := storage.DisableSecretVersion(ctx, "no-versions-separator")
		if err == nil {
			t.Error("DisableSecretVersion() should return error for invalid format")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("error code = %v, want InvalidArgument", st.Code())
		}
	})

	t.Run("SecretNotFound", func(t *testing.T) {
		_, err := storage.DisableSecretVersion(ctx, "projects/test-project/secrets/nonexistent/versions/1")
		if err == nil {
			t.Error("DisableSecretVersion() should return error for nonexistent secret")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("VersionNotFound", func(t *testing.T) {
		_, err := storage.DisableSecretVersion(ctx, secretName+"/versions/999")
		if err == nil {
			t.Error("DisableSecretVersion() should return error for nonexistent version")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("DisableDestroyedVersion", func(t *testing.T) {
		_, err := storage.DestroySecretVersion(ctx, secretName+"/versions/1")
		if err != nil {
			t.Fatalf("DestroySecretVersion() failed: %v", err)
		}
		_, err = storage.DisableSecretVersion(ctx, secretName+"/versions/1")
		if err == nil {
			t.Error("DisableSecretVersion() should return error for destroyed version")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.FailedPrecondition {
			t.Errorf("error code = %v, want FailedPrecondition", st.Code())
		}
	})

	t.Run("DisableAlreadyDisabledVersion", func(t *testing.T) {
		// Add new version and disable it twice
		_, err := storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{Data: []byte("v2")})
		if err != nil {
			t.Fatalf("AddSecretVersion() failed: %v", err)
		}

		_, err = storage.DisableSecretVersion(ctx, secretName+"/versions/2")
		if err != nil {
			t.Fatalf("first DisableSecretVersion() failed: %v", err)
		}

		// Disabling again should succeed
		resp, err := storage.DisableSecretVersion(ctx, secretName+"/versions/2")
		if err != nil {
			t.Fatalf("second DisableSecretVersion() failed: %v", err)
		}
		if resp.State != secretmanagerpb.SecretVersion_DISABLED {
			t.Errorf("State = %v, want DISABLED", resp.State)
		}
	})
}

// TestStorage_AccessSecretVersion_InvalidFormat fills the remaining gap.
func TestStorage_AccessSecretVersion_InvalidFormat(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	_, err := storage.AccessSecretVersion(ctx, "no-versions-separator")
	if err == nil {
		t.Error("AccessSecretVersion() should return error for invalid format")
		return
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Errorf("error code = %v, want InvalidArgument", st.Code())
	}
}

// TestStorage_DestroySecretVersion_Coverage fills gaps for DestroySecretVersion.
func TestStorage_DestroySecretVersion_Coverage(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	secretName := "projects/test-project/secrets/my-secret"
	_, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	_, err = storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{Data: []byte("data")})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("InvalidFormat", func(t *testing.T) {
		_, err := storage.DestroySecretVersion(ctx, "no-versions-separator")
		if err == nil {
			t.Error("DestroySecretVersion() should return error for invalid format")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("error code = %v, want InvalidArgument", st.Code())
		}
	})

	t.Run("SecretNotFound", func(t *testing.T) {
		_, err := storage.DestroySecretVersion(ctx, "projects/test-project/secrets/nonexistent/versions/1")
		if err == nil {
			t.Error("DestroySecretVersion() should return error for nonexistent secret")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("VersionNotFound", func(t *testing.T) {
		_, err := storage.DestroySecretVersion(ctx, secretName+"/versions/999")
		if err == nil {
			t.Error("DestroySecretVersion() should return error for nonexistent version")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("Idempotent", func(t *testing.T) {
		// Destroy once
		resp, err := storage.DestroySecretVersion(ctx, secretName+"/versions/1")
		if err != nil {
			t.Fatalf("first DestroySecretVersion() failed: %v", err)
		}
		if resp.State != secretmanagerpb.SecretVersion_DESTROYED {
			t.Errorf("State = %v, want DESTROYED", resp.State)
		}

		// Destroy again (idempotent)
		resp, err = storage.DestroySecretVersion(ctx, secretName+"/versions/1")
		if err != nil {
			t.Fatalf("second DestroySecretVersion() failed: %v", err)
		}
		if resp.State != secretmanagerpb.SecretVersion_DESTROYED {
			t.Errorf("State = %v, want DESTROYED", resp.State)
		}
	})

	t.Run("DestroyDisabledVersion", func(t *testing.T) {
		// Add new version, disable it, then destroy it
		_, err := storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{Data: []byte("v2")})
		if err != nil {
			t.Fatalf("AddSecretVersion() failed: %v", err)
		}

		_, err = storage.DisableSecretVersion(ctx, secretName+"/versions/2")
		if err != nil {
			t.Fatalf("DisableSecretVersion() failed: %v", err)
		}

		resp, err := storage.DestroySecretVersion(ctx, secretName+"/versions/2")
		if err != nil {
			t.Fatalf("DestroySecretVersion() on disabled version failed: %v", err)
		}
		if resp.State != secretmanagerpb.SecretVersion_DESTROYED {
			t.Errorf("State = %v, want DESTROYED", resp.State)
		}
	})
}

// TestStorage_UpdateSecret_Coverage fills coverage gaps for UpdateSecret.
func TestStorage_UpdateSecret_Coverage(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	secretName := "projects/test-project/secrets/my-secret"
	_, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{
		Labels:      map[string]string{"key": "val"},
		Annotations: map[string]string{"ann": "val"},
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("NotFound", func(t *testing.T) {
		_, err := storage.UpdateSecret(ctx, "projects/test-project/secrets/nonexistent", nil, nil, nil)
		if err == nil {
			t.Error("UpdateSecret() should return error for nonexistent secret")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("NilLabels_Preserves", func(t *testing.T) {
		resp, err := storage.UpdateSecret(ctx, secretName, nil, map[string]string{"new-ann": "new"}, nil)
		if err != nil {
			t.Fatalf("UpdateSecret() failed: %v", err)
		}
		if resp.Labels["key"] != "val" {
			t.Errorf("Labels should be preserved, got %v", resp.Labels)
		}
		if resp.Annotations["new-ann"] != "new" {
			t.Errorf("Annotations not updated: %v", resp.Annotations)
		}
	})

	t.Run("NilAnnotations_Preserves", func(t *testing.T) {
		resp, err := storage.UpdateSecret(ctx, secretName, map[string]string{"new-key": "new"}, nil, nil)
		if err != nil {
			t.Fatalf("UpdateSecret() failed: %v", err)
		}
		if resp.Labels["new-key"] != "new" {
			t.Errorf("Labels not updated: %v", resp.Labels)
		}
		// Annotations should be preserved from previous update
		if resp.Annotations["new-ann"] != "new" {
			t.Errorf("Annotations should be preserved, got %v", resp.Annotations)
		}
	})
}

// TestStorage_ListSecretVersions_Coverage fills coverage gaps for ListSecretVersions.
func TestStorage_ListSecretVersions_Coverage(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	secretName := "projects/test-project/secrets/my-secret"
	_, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Add 3 versions
	for i := 1; i <= 3; i++ {
		_, err := storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{
			Data: []byte("data"),
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
	}

	t.Run("NotFound", func(t *testing.T) {
		_, _, _, err := storage.ListSecretVersions(ctx, "projects/test-project/secrets/nonexistent", 10, "", "")
		if err == nil {
			t.Error("ListSecretVersions() should return error for nonexistent secret")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("EmptyFilter", func(t *testing.T) {
		versions, token, _, err := storage.ListSecretVersions(ctx, secretName, 100, "", "")
		if err != nil {
			t.Fatalf("ListSecretVersions() failed: %v", err)
		}
		if len(versions) != 3 {
			t.Errorf("count = %d, want 3", len(versions))
		}
		if token != "" {
			t.Errorf("token = %q, want empty", token)
		}
	})

	t.Run("FilterEnabled", func(t *testing.T) {
		// All versions are ENABLED
		versions, _, _, err := storage.ListSecretVersions(ctx, secretName, 100, "", "state:ENABLED")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if len(versions) != 3 {
			t.Errorf("count = %d, want 3", len(versions))
		}
	})

	t.Run("FilterDisabled_Empty", func(t *testing.T) {
		versions, _, _, err := storage.ListSecretVersions(ctx, secretName, 100, "", "state:DISABLED")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if len(versions) != 0 {
			t.Errorf("count = %d, want 0", len(versions))
		}
	})

	t.Run("Pagination_WithFilter", func(t *testing.T) {
		versions, token, _, err := storage.ListSecretVersions(ctx, secretName, 2, "", "state:ENABLED")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if len(versions) != 2 {
			t.Errorf("page 1 count = %d, want 2", len(versions))
		}
		if token == "" {
			t.Error("page 1 should have next token")
		}

		versions2, token2, _, err := storage.ListSecretVersions(ctx, secretName, 2, token, "state:ENABLED")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if len(versions2) != 1 {
			t.Errorf("page 2 count = %d, want 1", len(versions2))
		}
		if token2 != "" {
			t.Errorf("page 2 token = %q, want empty", token2)
		}
	})

	t.Run("FilterCaseInsensitive_LowerCase", func(t *testing.T) {
		// Filter with lowercase state - parseStateFilter uses strings.ToUpper
		versions, _, _, err := storage.ListSecretVersions(ctx, secretName, 100, "", "state:enabled")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if len(versions) != 3 {
			t.Errorf("count = %d, want 3", len(versions))
		}
	})

	t.Run("FilterWithSpaces", func(t *testing.T) {
		// Filter with extra whitespace
		versions, _, _, err := storage.ListSecretVersions(ctx, secretName, 100, "", "  state:ENABLED  ")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		// After TrimSpace, the filter becomes "state:ENABLED" which should work
		if len(versions) != 3 {
			t.Errorf("count = %d, want 3", len(versions))
		}
	})

	t.Run("PageTokenBeyondRange", func(t *testing.T) {
		// Token pointing beyond available versions
		versions, token, _, err := storage.ListSecretVersions(ctx, secretName, 100, "999", "")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if len(versions) != 0 {
			t.Errorf("count = %d, want 0", len(versions))
		}
		if token != "" {
			t.Errorf("token = %q, want empty", token)
		}
	})
}

// TestStorage_ListSecrets_Coverage fills coverage gaps for ListSecrets.
func TestStorage_ListSecrets_Coverage(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	t.Run("EmptyStorage", func(t *testing.T) {
		secrets, token, _, err := storage.ListSecrets(ctx, "projects/test-project", 10, "", "")
		if err != nil {
			t.Fatalf("ListSecrets() failed: %v", err)
		}
		if len(secrets) != 0 {
			t.Errorf("count = %d, want 0", len(secrets))
		}
		if token != "" {
			t.Errorf("token = %q, want empty", token)
		}
	})

	t.Run("PageTokenBeyondRange", func(t *testing.T) {
		// Create 2 secrets
		_, err := storage.CreateSecret(ctx, "projects/test-project", "s1", &secretmanagerpb.Secret{})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
		_, err = storage.CreateSecret(ctx, "projects/test-project", "s2", &secretmanagerpb.Secret{})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Token beyond range
		secrets, token, _, err := storage.ListSecrets(ctx, "projects/test-project", 10, "999", "")
		if err != nil {
			t.Fatalf("ListSecrets() failed: %v", err)
		}
		if len(secrets) != 0 {
			t.Errorf("count = %d, want 0", len(secrets))
		}
		if token != "" {
			t.Errorf("token = %q, want empty", token)
		}
	})

	t.Run("NegativePageSize_DefaultsTo100", func(t *testing.T) {
		secrets, _, _, err := storage.ListSecrets(ctx, "projects/test-project", -1, "", "")
		if err != nil {
			t.Fatalf("ListSecrets() failed: %v", err)
		}
		if len(secrets) != 2 {
			t.Errorf("count = %d, want 2", len(secrets))
		}
	})

	t.Run("DifferentProject_NoResults", func(t *testing.T) {
		secrets, _, _, err := storage.ListSecrets(ctx, "projects/other-project", 10, "", "")
		if err != nil {
			t.Fatalf("ListSecrets() failed: %v", err)
		}
		if len(secrets) != 0 {
			t.Errorf("count = %d, want 0 for different project", len(secrets))
		}
	})
}

// TestStorage_CreateSecret_WithAnnotationsAndReplication tests additional create paths.
func TestStorage_CreateSecret_WithAnnotationsAndReplication(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	t.Run("WithAnnotations", func(t *testing.T) {
		secret, err := storage.CreateSecret(ctx, "projects/test-project", "annotated-secret", &secretmanagerpb.Secret{
			Annotations: map[string]string{"managed-by": "terraform"},
		})
		if err != nil {
			t.Fatalf("CreateSecret() error = %v", err)
		}
		if secret.Annotations["managed-by"] != "terraform" {
			t.Errorf("Annotations not set: %v", secret.Annotations)
		}
	})

	t.Run("WithCustomReplication", func(t *testing.T) {
		replication := &secretmanagerpb.Replication{
			Replication: &secretmanagerpb.Replication_Automatic_{
				Automatic: &secretmanagerpb.Replication_Automatic{},
			},
		}
		secret, err := storage.CreateSecret(ctx, "projects/test-project", "custom-repl", &secretmanagerpb.Secret{
			Replication: replication,
		})
		if err != nil {
			t.Fatalf("CreateSecret() error = %v", err)
		}
		if secret.Replication == nil {
			t.Error("Replication should be set")
		}
	})

	t.Run("WithoutReplication_DefaultApplied", func(t *testing.T) {
		secret, err := storage.CreateSecret(ctx, "projects/test-project", "no-repl", &secretmanagerpb.Secret{})
		if err != nil {
			t.Fatalf("CreateSecret() error = %v", err)
		}
		if secret.Replication == nil {
			t.Error("Default replication should be applied")
		}
	})
}

// TestStorage_AccessSecretVersion_DisabledVersion tests accessing a disabled version.
func TestStorage_AccessSecretVersion_DisabledVersion(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	secretName := "projects/test-project/secrets/my-secret"
	_, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	_, err = storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{Data: []byte("data")})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Disable the version
	_, err = storage.DisableSecretVersion(ctx, secretName+"/versions/1")
	if err != nil {
		t.Fatalf("DisableSecretVersion() failed: %v", err)
	}

	// Access should fail with FailedPrecondition
	_, err = storage.AccessSecretVersion(ctx, secretName+"/versions/1")
	if err == nil {
		t.Error("AccessSecretVersion() should fail for disabled version")
		return
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.FailedPrecondition {
		t.Errorf("error code = %v, want FailedPrecondition", st.Code())
	}
}

// TestStorage_AccessSecretVersion_DestroyedVersion tests accessing a destroyed version.
func TestStorage_AccessSecretVersion_DestroyedVersion(t *testing.T) {
	storage := NewStorage()
	ctx := context.Background()

	secretName := "projects/test-project/secrets/my-secret"
	_, err := storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	_, err = storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{Data: []byte("data")})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Destroy the version
	_, err = storage.DestroySecretVersion(ctx, secretName+"/versions/1")
	if err != nil {
		t.Fatalf("DestroySecretVersion() failed: %v", err)
	}

	// Access should fail with FailedPrecondition
	_, err = storage.AccessSecretVersion(ctx, secretName+"/versions/1")
	if err == nil {
		t.Error("AccessSecretVersion() should fail for destroyed version")
		return
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.FailedPrecondition {
		t.Errorf("error code = %v, want FailedPrecondition", st.Code())
	}
}

// TestParseStateFilter tests the parseStateFilter function directly.
func TestParseStateFilter(t *testing.T) {
	tests := []struct {
		name       string
		filter     string
		wantNil    bool
		wantStates []secretmanagerpb.SecretVersion_State
	}{
		{
			name:    "EmptyFilter",
			filter:  "",
			wantNil: true,
		},
		{
			name:       "ENABLED",
			filter:     "state:ENABLED",
			wantStates: []secretmanagerpb.SecretVersion_State{secretmanagerpb.SecretVersion_ENABLED},
		},
		{
			name:       "DISABLED",
			filter:     "state:DISABLED",
			wantStates: []secretmanagerpb.SecretVersion_State{secretmanagerpb.SecretVersion_DISABLED},
		},
		{
			name:       "DESTROYED",
			filter:     "state:DESTROYED",
			wantStates: []secretmanagerpb.SecretVersion_State{secretmanagerpb.SecretVersion_DESTROYED},
		},
		{
			name:       "lowercase",
			filter:     "state:enabled",
			wantStates: []secretmanagerpb.SecretVersion_State{secretmanagerpb.SecretVersion_ENABLED},
		},
		{
			name:       "MixedCase",
			filter:     "state:Enabled",
			wantStates: []secretmanagerpb.SecretVersion_State{secretmanagerpb.SecretVersion_ENABLED},
		},
		{
			name:    "NonStatePrefix",
			filter:  "name:something",
			wantNil: false, // Returns empty map (not nil), no state: prefix match
		},
		{
			name:    "InvalidState",
			filter:  "state:UNKNOWN",
			wantNil: false, // Returns empty map (no keys added for unknown state)
		},
		{
			name:       "WithWhitespace",
			filter:     "  state:ENABLED  ",
			wantStates: []secretmanagerpb.SecretVersion_State{secretmanagerpb.SecretVersion_ENABLED},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseStateFilter(tt.filter)

			if tt.wantNil {
				if result != nil {
					t.Errorf("parseStateFilter(%q) = %v, want nil", tt.filter, result)
				}
				return
			}

			if len(tt.wantStates) > 0 {
				for _, state := range tt.wantStates {
					if !result[state] {
						t.Errorf("parseStateFilter(%q) missing state %v", tt.filter, state)
					}
				}
			}
		})
	}
}
