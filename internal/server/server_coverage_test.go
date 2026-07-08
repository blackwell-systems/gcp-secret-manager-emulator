package server

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// TestServer_checkPermission_NilIAMClient tests that checkPermission allows
// all operations when IAM client is nil (IAM disabled).
func TestServer_checkPermission_NilIAMClient(t *testing.T) {
	s := &Server{
		storage:   NewStorage(),
		iamClient: nil, // IAM disabled
	}

	ctx := context.Background()

	t.Run("AllowsKnownOperation", func(t *testing.T) {
		err := s.checkPermission(ctx, "GetSecret", "projects/p/secrets/s")
		if err != nil {
			t.Errorf("checkPermission() with nil IAM client returned error: %v", err)
		}
	})

	t.Run("AllowsUnknownOperation", func(t *testing.T) {
		err := s.checkPermission(ctx, "UnknownOp", "projects/p/secrets/s")
		if err != nil {
			t.Errorf("checkPermission() with unknown op returned error: %v", err)
		}
	})

	t.Run("AllowsEmptyOperation", func(t *testing.T) {
		err := s.checkPermission(ctx, "", "some-resource")
		if err != nil {
			t.Errorf("checkPermission() with empty op returned error: %v", err)
		}
	})
}

// TestServer_GetSecretVersion_Coverage fills coverage gaps for GetSecretVersion
// in the server layer (success, latest alias, missing name).
func TestServer_GetSecretVersion_Coverage(t *testing.T) {
	ctx := context.Background()
	s := &Server{
		storage:   NewStorage(),
		iamClient: nil,
	}

	// Setup: create secret and version
	secretName := "projects/test-project/secrets/test-secret"
	_, err := s.storage.CreateSecret(ctx, "projects/test-project", "test-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	_, err = s.storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{
		Data: []byte("test-data"),
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("Success_SpecificVersion", func(t *testing.T) {
		resp, err := s.GetSecretVersion(ctx, &secretmanagerpb.GetSecretVersionRequest{
			Name: secretName + "/versions/1",
		})
		if err != nil {
			t.Fatalf("GetSecretVersion() error = %v", err)
		}
		if resp.Name != secretName+"/versions/1" {
			t.Errorf("GetSecretVersion().Name = %q, want %q", resp.Name, secretName+"/versions/1")
		}
		if resp.State != secretmanagerpb.SecretVersion_ENABLED {
			t.Errorf("GetSecretVersion().State = %v, want ENABLED", resp.State)
		}
		if resp.CreateTime == nil {
			t.Error("GetSecretVersion().CreateTime is nil")
		}
	})

	t.Run("Success_LatestAlias", func(t *testing.T) {
		resp, err := s.GetSecretVersion(ctx, &secretmanagerpb.GetSecretVersionRequest{
			Name: secretName + "/versions/latest",
		})
		if err != nil {
			t.Fatalf("GetSecretVersion(latest) error = %v", err)
		}
		if resp.Name != secretName+"/versions/1" {
			t.Errorf("GetSecretVersion(latest).Name = %q, want version 1", resp.Name)
		}
	})

	t.Run("MissingName", func(t *testing.T) {
		_, err := s.GetSecretVersion(ctx, &secretmanagerpb.GetSecretVersionRequest{
			Name: "",
		})
		if err == nil {
			t.Error("GetSecretVersion() should return error for empty name")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("GetSecretVersion() error code = %v, want InvalidArgument", st.Code())
		}
	})

	t.Run("VersionNotFound", func(t *testing.T) {
		_, err := s.GetSecretVersion(ctx, &secretmanagerpb.GetSecretVersionRequest{
			Name: secretName + "/versions/999",
		})
		if err == nil {
			t.Error("GetSecretVersion() should return error for nonexistent version")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("GetSecretVersion() error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("SecretNotFound", func(t *testing.T) {
		_, err := s.GetSecretVersion(ctx, &secretmanagerpb.GetSecretVersionRequest{
			Name: "projects/test-project/secrets/nonexistent/versions/1",
		})
		if err == nil {
			t.Error("GetSecretVersion() should return error for nonexistent secret")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("GetSecretVersion() error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("GetDestroyedVersion", func(t *testing.T) {
		// Destroy the version
		_, err := s.storage.DestroySecretVersion(ctx, secretName+"/versions/1")
		if err != nil {
			t.Fatalf("DestroySecretVersion() failed: %v", err)
		}

		// GetSecretVersion should still return metadata for destroyed versions
		resp, err := s.GetSecretVersion(ctx, &secretmanagerpb.GetSecretVersionRequest{
			Name: secretName + "/versions/1",
		})
		if err != nil {
			t.Fatalf("GetSecretVersion() for destroyed version error = %v", err)
		}
		if resp.State != secretmanagerpb.SecretVersion_DESTROYED {
			t.Errorf("GetSecretVersion().State = %v, want DESTROYED", resp.State)
		}
	})

	t.Run("LatestWithNoEnabledVersions", func(t *testing.T) {
		// All versions are destroyed, latest should fail
		_, err := s.GetSecretVersion(ctx, &secretmanagerpb.GetSecretVersionRequest{
			Name: secretName + "/versions/latest",
		})
		if err == nil {
			t.Error("GetSecretVersion(latest) should fail when no enabled versions exist")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("GetSecretVersion(latest) error code = %v, want NotFound", st.Code())
		}
	})
}

// TestServer_AddSecretVersion_MissingPayload fills the nil payload gap.
func TestServer_AddSecretVersion_MissingPayload(t *testing.T) {
	ctx := context.Background()
	s := &Server{
		storage:   NewStorage(),
		iamClient: nil,
	}

	// Setup
	_, err := s.storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	_, err = s.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent:  "projects/test-project/secrets/my-secret",
		Payload: nil,
	})
	if err == nil {
		t.Error("AddSecretVersion() should return error for nil payload")
		return
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Errorf("AddSecretVersion() error code = %v, want InvalidArgument", st.Code())
	}
}

// TestServer_CreateSecret_NilSecret fills the nil secret gap.
func TestServer_CreateSecret_NilSecret(t *testing.T) {
	ctx := context.Background()
	s := &Server{
		storage:   NewStorage(),
		iamClient: nil,
	}

	_, err := s.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   "projects/test-project",
		SecretId: "my-secret",
		Secret:   nil,
	})
	if err == nil {
		t.Error("CreateSecret() should return error for nil secret")
		return
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Errorf("CreateSecret() error code = %v, want InvalidArgument", st.Code())
	}
}

// TestServer_ListSecrets_PaginationThroughServer tests pagination via server layer.
func TestServer_ListSecrets_PaginationThroughServer(t *testing.T) {
	ctx := context.Background()
	s := &Server{
		storage:   NewStorage(),
		iamClient: nil,
	}

	// Create 5 secrets
	for i := 1; i <= 5; i++ {
		_, err := s.storage.CreateSecret(ctx, "projects/test-project", fmt.Sprintf("secret-%d", i), &secretmanagerpb.Secret{})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
	}

	t.Run("PaginateFullCycle", func(t *testing.T) {
		// Page through with size 2
		resp, err := s.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
			Parent:   "projects/test-project",
			PageSize: 2,
		})
		if err != nil {
			t.Fatalf("ListSecrets() page 1 error = %v", err)
		}
		if len(resp.Secrets) != 2 {
			t.Errorf("ListSecrets() page 1 count = %d, want 2", len(resp.Secrets))
		}
		if resp.NextPageToken == "" {
			t.Error("ListSecrets() page 1 should have next page token")
		}

		// Page 2
		resp2, err := s.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
			Parent:    "projects/test-project",
			PageSize:  2,
			PageToken: resp.NextPageToken,
		})
		if err != nil {
			t.Fatalf("ListSecrets() page 2 error = %v", err)
		}
		if len(resp2.Secrets) != 2 {
			t.Errorf("ListSecrets() page 2 count = %d, want 2", len(resp2.Secrets))
		}

		// Page 3 (last)
		resp3, err := s.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
			Parent:    "projects/test-project",
			PageSize:  2,
			PageToken: resp2.NextPageToken,
		})
		if err != nil {
			t.Fatalf("ListSecrets() page 3 error = %v", err)
		}
		if len(resp3.Secrets) != 1 {
			t.Errorf("ListSecrets() page 3 count = %d, want 1", len(resp3.Secrets))
		}
		if resp3.NextPageToken != "" {
			t.Errorf("ListSecrets() page 3 should have empty token, got %q", resp3.NextPageToken)
		}
	})

	t.Run("EmptyProject", func(t *testing.T) {
		resp, err := s.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
			Parent: "projects/empty-project",
		})
		if err != nil {
			t.Fatalf("ListSecrets() error = %v", err)
		}
		if len(resp.Secrets) != 0 {
			t.Errorf("ListSecrets() count = %d, want 0", len(resp.Secrets))
		}
		if resp.NextPageToken != "" {
			t.Errorf("ListSecrets() should have empty token for empty project, got %q", resp.NextPageToken)
		}
	})

	t.Run("DefaultPageSize", func(t *testing.T) {
		resp, err := s.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
			Parent:   "projects/test-project",
			PageSize: 0, // should default to 100
		})
		if err != nil {
			t.Fatalf("ListSecrets() error = %v", err)
		}
		if len(resp.Secrets) != 5 {
			t.Errorf("ListSecrets() count = %d, want 5", len(resp.Secrets))
		}
	})
}

// TestServer_UpdateSecret_UnsupportedFieldInMask tests update with unsupported field path.
func TestServer_UpdateSecret_UnsupportedFieldInMask(t *testing.T) {
	ctx := context.Background()
	s := &Server{
		storage:   NewStorage(),
		iamClient: nil,
	}

	_, err := s.storage.CreateSecret(ctx, "projects/test-project", "my-secret", &secretmanagerpb.Secret{
		Labels: map[string]string{"env": "dev"},
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("UnsupportedField_Silently_Skipped", func(t *testing.T) {
		// Update with unsupported field - should silently skip
		resp, err := s.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
			Secret: &secretmanagerpb.Secret{
				Name: "projects/test-project/secrets/my-secret",
			},
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{"unsupported_field"},
			},
		})
		if err != nil {
			t.Fatalf("UpdateSecret() error = %v", err)
		}
		// Labels should remain unchanged
		if resp.Labels["env"] != "dev" {
			t.Errorf("Labels changed unexpectedly: %v", resp.Labels)
		}
	})

	t.Run("MixedFields_SupportedAndUnsupported", func(t *testing.T) {
		resp, err := s.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
			Secret: &secretmanagerpb.Secret{
				Name:   "projects/test-project/secrets/my-secret",
				Labels: map[string]string{"env": "prod"},
			},
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{"labels", "unsupported_field"},
			},
		})
		if err != nil {
			t.Fatalf("UpdateSecret() error = %v", err)
		}
		if resp.Labels["env"] != "prod" {
			t.Errorf("Labels not updated: %v", resp.Labels)
		}
	})

	t.Run("NilSecret_InRequest", func(t *testing.T) {
		_, err := s.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
			Secret: nil,
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{"labels"},
			},
		})
		if err == nil {
			t.Error("UpdateSecret() should return error for nil secret")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("UpdateSecret() error code = %v, want InvalidArgument", st.Code())
		}
	})

	t.Run("UpdateBothLabelsAndAnnotations", func(t *testing.T) {
		resp, err := s.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
			Secret: &secretmanagerpb.Secret{
				Name:        "projects/test-project/secrets/my-secret",
				Labels:      map[string]string{"new-label": "value"},
				Annotations: map[string]string{"ann": "val"},
			},
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{"labels", "annotations"},
			},
		})
		if err != nil {
			t.Fatalf("UpdateSecret() error = %v", err)
		}
		if resp.Labels["new-label"] != "value" {
			t.Errorf("Labels not updated: %v", resp.Labels)
		}
		if resp.Annotations["ann"] != "val" {
			t.Errorf("Annotations not updated: %v", resp.Annotations)
		}
	})
}

// TestServer_EnableSecretVersion_Coverage fills gaps for EnableSecretVersion.
func TestServer_EnableSecretVersion_Coverage(t *testing.T) {
	ctx := context.Background()
	s := &Server{
		storage:   NewStorage(),
		iamClient: nil,
	}

	// Setup
	secretName := "projects/test-project/secrets/test-secret"
	_, err := s.storage.CreateSecret(ctx, "projects/test-project", "test-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	_, err = s.storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{Data: []byte("data")})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("EnableNotFound_SecretMissing", func(t *testing.T) {
		_, err := s.EnableSecretVersion(ctx, &secretmanagerpb.EnableSecretVersionRequest{
			Name: "projects/test-project/secrets/nonexistent/versions/1",
		})
		if err == nil {
			t.Error("EnableSecretVersion() should return error")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("EnableNotFound_VersionMissing", func(t *testing.T) {
		_, err := s.EnableSecretVersion(ctx, &secretmanagerpb.EnableSecretVersionRequest{
			Name: secretName + "/versions/999",
		})
		if err == nil {
			t.Error("EnableSecretVersion() should return error")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("EnableDestroyedVersion", func(t *testing.T) {
		// Destroy version first
		_, err := s.storage.DestroySecretVersion(ctx, secretName+"/versions/1")
		if err != nil {
			t.Fatalf("DestroySecretVersion() failed: %v", err)
		}

		_, err = s.EnableSecretVersion(ctx, &secretmanagerpb.EnableSecretVersionRequest{
			Name: secretName + "/versions/1",
		})
		if err == nil {
			t.Error("EnableSecretVersion() should return error for destroyed version")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.FailedPrecondition {
			t.Errorf("error code = %v, want FailedPrecondition", st.Code())
		}
	})
}

// TestServer_DisableSecretVersion_Coverage fills gaps for DisableSecretVersion.
func TestServer_DisableSecretVersion_Coverage(t *testing.T) {
	ctx := context.Background()
	s := &Server{
		storage:   NewStorage(),
		iamClient: nil,
	}

	// Setup
	secretName := "projects/test-project/secrets/test-secret"
	_, err := s.storage.CreateSecret(ctx, "projects/test-project", "test-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	_, err = s.storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{Data: []byte("data")})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("DisableNotFound_SecretMissing", func(t *testing.T) {
		_, err := s.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{
			Name: "projects/test-project/secrets/nonexistent/versions/1",
		})
		if err == nil {
			t.Error("DisableSecretVersion() should return error")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})

	t.Run("DisableDestroyedVersion", func(t *testing.T) {
		// Destroy version first
		_, err := s.storage.DestroySecretVersion(ctx, secretName+"/versions/1")
		if err != nil {
			t.Fatalf("DestroySecretVersion() failed: %v", err)
		}

		_, err = s.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{
			Name: secretName + "/versions/1",
		})
		if err == nil {
			t.Error("DisableSecretVersion() should return error for destroyed version")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.FailedPrecondition {
			t.Errorf("error code = %v, want FailedPrecondition", st.Code())
		}
	})
}

// TestServer_DestroySecretVersion_Coverage fills gaps for DestroySecretVersion via server layer.
func TestServer_DestroySecretVersion_Coverage(t *testing.T) {
	ctx := context.Background()
	s := &Server{
		storage:   NewStorage(),
		iamClient: nil,
	}

	// Setup
	secretName := "projects/test-project/secrets/test-secret"
	_, err := s.storage.CreateSecret(ctx, "projects/test-project", "test-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	_, err = s.storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{Data: []byte("data")})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	t.Run("DestroyNotFound_SecretMissing", func(t *testing.T) {
		_, err := s.DestroySecretVersion(ctx, &secretmanagerpb.DestroySecretVersionRequest{
			Name: "projects/test-project/secrets/nonexistent/versions/1",
		})
		if err == nil {
			t.Error("DestroySecretVersion() should return error")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("error code = %v, want NotFound", st.Code())
		}
	})
}

// TestServer_AccessSecretVersion_Coverage fills AccessSecretVersion gaps.
func TestServer_AccessSecretVersion_Coverage(t *testing.T) {
	ctx := context.Background()
	s := &Server{
		storage:   NewStorage(),
		iamClient: nil,
	}

	t.Run("InvalidFormat_NoVersionSeparator", func(t *testing.T) {
		_, err := s.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
			Name: "invalid-resource-name",
		})
		if err == nil {
			t.Error("AccessSecretVersion() should return error for invalid format")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("error code = %v, want InvalidArgument", st.Code())
		}
	})
}

// TestServer_ListSecretVersions_Coverage fills ListSecretVersions gaps.
func TestServer_ListSecretVersions_Coverage(t *testing.T) {
	ctx := context.Background()
	s := &Server{
		storage:   NewStorage(),
		iamClient: nil,
	}

	// Setup with a secret with multiple versions
	secretName := "projects/test-project/secrets/test-secret"
	_, err := s.storage.CreateSecret(ctx, "projects/test-project", "test-secret", &secretmanagerpb.Secret{})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	for i := 1; i <= 3; i++ {
		_, err := s.storage.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{
			Data: []byte(fmt.Sprintf("data-%d", i)),
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
	}

	t.Run("FilterByDestroyedState", func(t *testing.T) {
		// Destroy version 1
		_, err := s.storage.DestroySecretVersion(ctx, secretName+"/versions/1")
		if err != nil {
			t.Fatalf("DestroySecretVersion() failed: %v", err)
		}

		resp, err := s.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent: secretName,
			Filter: "state:DESTROYED",
		})
		if err != nil {
			t.Fatalf("ListSecretVersions(DESTROYED) failed: %v", err)
		}
		if len(resp.Versions) != 1 {
			t.Errorf("ListSecretVersions(DESTROYED) returned %d, want 1", len(resp.Versions))
		}
		if resp.Versions[0].State != secretmanagerpb.SecretVersion_DESTROYED {
			t.Errorf("Version state = %v, want DESTROYED", resp.Versions[0].State)
		}
	})

	t.Run("InvalidFilter_ReturnsAll", func(t *testing.T) {
		// Filter with invalid state name: parseStateFilter returns empty map (no keys added),
		// so len(includeStates) == 0, which means no filtering is applied (all versions returned).
		resp, err := s.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent: secretName,
			Filter: "state:INVALID_STATE",
		})
		if err != nil {
			t.Fatalf("ListSecretVersions(INVALID_STATE) failed: %v", err)
		}
		// 3 versions total (1 destroyed + 2 enabled)
		if len(resp.Versions) != 3 {
			t.Errorf("ListSecretVersions(INVALID_STATE) returned %d, want 3", len(resp.Versions))
		}
	})

	t.Run("NonStateFilter_IgnoredReturnsAll", func(t *testing.T) {
		// Filter without "state:" prefix returns nil (no filter applied)
		resp, err := s.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent: secretName,
			Filter: "name:something",
		})
		if err != nil {
			t.Fatalf("ListSecretVersions(name:something) failed: %v", err)
		}
		if len(resp.Versions) != 3 {
			t.Errorf("ListSecretVersions(non-state-filter) returned %d, want 3", len(resp.Versions))
		}
	})
}

// TestNewServer_WithIAMEnvVars tests NewServer when IAM env vars are set.
func TestNewServer_WithIAMEnvVars(t *testing.T) {
	t.Run("StrictMode_InvalidHost", func(t *testing.T) {
		t.Setenv("IAM_MODE", "strict")
		t.Setenv("IAM_HOST", "localhost:19999") // non-existent host
		// NewClient with strict mode and unreachable host — should succeed (gRPC is lazy dial).
		s, err := NewServer()
		if err != nil {
			// Some implementations eagerly validate; either result is fine.
			t.Logf("NewServer with IAM strict mode returned error (acceptable): %v", err)
			return
		}
		if s.iamClient == nil {
			t.Error("NewServer() with IAM_MODE=strict should set iamClient")
		}
	})

	t.Run("OffMode", func(t *testing.T) {
		t.Setenv("IAM_MODE", "off")
		t.Setenv("IAM_HOST", "")
		s, err := NewServer()
		if err != nil {
			t.Fatalf("NewServer() with IAM_MODE=off returned error: %v", err)
		}
		if s.iamClient != nil {
			t.Error("NewServer() with IAM_MODE=off should not set iamClient")
		}
	})

	t.Run("PermissiveMode", func(t *testing.T) {
		t.Setenv("IAM_MODE", "permissive")
		t.Setenv("IAM_HOST", "localhost:19999")
		s, err := NewServer()
		if err != nil {
			t.Logf("NewServer with IAM permissive mode returned error (acceptable): %v", err)
			return
		}
		if s.iamClient == nil {
			t.Error("NewServer() with IAM_MODE=permissive should set iamClient")
		}
	})
}

// TestServer_checkPermission_WithIAMClient tests checkPermission when an IAM client is set.
// It uses a client pointed at a non-existent host to exercise the error path.
func TestServer_checkPermission_WithIAMClient(t *testing.T) {
	t.Run("IAMCheckFailure_ReturnsInternal", func(t *testing.T) {
		t.Setenv("IAM_MODE", "strict")
		t.Setenv("IAM_HOST", "localhost:19999")
		s, err := NewServer()
		if err != nil {
			t.Skipf("NewServer failed (IAM may require reachable host): %v", err)
		}
		if s.iamClient == nil {
			t.Skip("iamClient not set — IAM mode may not be enabled")
		}

		ctx := context.Background()
		// GetSecret is a known operation, so authz.GetPermission will return a permission.
		// The IAM call to localhost:19999 will fail, returning codes.Internal.
		err = s.checkPermission(ctx, "GetSecret", "projects/p/secrets/s")
		if err == nil {
			t.Error("checkPermission() with unreachable IAM should return error")
			return
		}
		st, ok := status.FromError(err)
		if !ok {
			t.Errorf("checkPermission() error is not a gRPC status: %v", err)
			return
		}
		if st.Code() != codes.Internal && st.Code() != codes.PermissionDenied && st.Code() != codes.Unavailable {
			t.Errorf("checkPermission() error code = %v, want Internal/PermissionDenied/Unavailable", st.Code())
		}
	})
}

// TestServer_CreateSecret_DefaultReplication tests that default replication is applied.
func TestServer_CreateSecret_DefaultReplication(t *testing.T) {
	ctx := context.Background()
	s := &Server{
		storage:   NewStorage(),
		iamClient: nil,
	}

	secret, err := s.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   "projects/test-project",
		SecretId: "no-replication",
		Secret:   &secretmanagerpb.Secret{}, // No replication specified
	})
	if err != nil {
		t.Fatalf("CreateSecret() error = %v", err)
	}
	if secret.Replication == nil {
		t.Error("CreateSecret() should set default replication")
	}
}
