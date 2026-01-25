package server

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestServer_CreateSecret(t *testing.T) {
	ctx := context.Background()
	server := NewServer()

	tests := []struct {
		name    string
		req     *secretmanagerpb.CreateSecretRequest
		wantErr codes.Code
	}{
		{
			name: "Success",
			req: &secretmanagerpb.CreateSecretRequest{
				Parent:   "projects/test-project",
				SecretId: "test-secret",
				Secret: &secretmanagerpb.Secret{
					Replication: &secretmanagerpb.Replication{
						Replication: &secretmanagerpb.Replication_Automatic_{
							Automatic: &secretmanagerpb.Replication_Automatic{},
						},
					},
				},
			},
			wantErr: codes.OK,
		},
		{
			name: "MissingParent",
			req: &secretmanagerpb.CreateSecretRequest{
				SecretId: "test-secret",
				Secret:   &secretmanagerpb.Secret{},
			},
			wantErr: codes.InvalidArgument,
		},
		{
			name: "MissingSecretId",
			req: &secretmanagerpb.CreateSecretRequest{
				Parent: "projects/test-project",
				Secret: &secretmanagerpb.Secret{},
			},
			wantErr: codes.InvalidArgument,
		},
		{
			name: "AlreadyExists",
			req: &secretmanagerpb.CreateSecretRequest{
				Parent:   "projects/test-project",
				SecretId: "test-secret",
				Secret:   &secretmanagerpb.Secret{},
			},
			wantErr: codes.AlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret, err := server.CreateSecret(ctx, tt.req)

			if tt.wantErr != codes.OK {
				if err == nil {
					t.Errorf("CreateSecret() error = nil, wantErr %v", tt.wantErr)
					return
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("CreateSecret() error is not a status error: %v", err)
					return
				}
				if st.Code() != tt.wantErr {
					t.Errorf("CreateSecret() error code = %v, wantErr %v", st.Code(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("CreateSecret() unexpected error = %v", err)
				return
			}
			if secret == nil {
				t.Error("CreateSecret() returned nil secret")
			}
		})
	}
}

func TestServer_GetSecret(t *testing.T) {
	ctx := context.Background()
	server := NewServer()

	// Create a test secret first
	_, err := server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   "projects/test-project",
		SecretId: "test-secret",
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	tests := []struct {
		name    string
		req     *secretmanagerpb.GetSecretRequest
		wantErr codes.Code
	}{
		{
			name: "Success",
			req: &secretmanagerpb.GetSecretRequest{
				Name: "projects/test-project/secrets/test-secret",
			},
			wantErr: codes.OK,
		},
		{
			name: "MissingName",
			req: &secretmanagerpb.GetSecretRequest{
				Name: "",
			},
			wantErr: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req: &secretmanagerpb.GetSecretRequest{
				Name: "projects/test-project/secrets/nonexistent",
			},
			wantErr: codes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret, err := server.GetSecret(ctx, tt.req)

			if tt.wantErr != codes.OK {
				if err == nil {
					t.Errorf("GetSecret() error = nil, wantErr %v", tt.wantErr)
					return
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("GetSecret() error is not a status error: %v", err)
					return
				}
				if st.Code() != tt.wantErr {
					t.Errorf("GetSecret() error code = %v, wantErr %v", st.Code(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("GetSecret() unexpected error = %v", err)
				return
			}
			if secret == nil {
				t.Error("GetSecret() returned nil secret")
			}
		})
	}
}

func TestServer_ListSecrets(t *testing.T) {
	ctx := context.Background()
	server := NewServer()

	// Clear any existing data
	server.Storage().Clear()

	// Create test secrets
	for i := 1; i <= 5; i++ {
		_, err := server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
			Parent:   "projects/test-project",
			SecretId: "test-secret-" + string(rune('0'+i)),
			Secret: &secretmanagerpb.Secret{
				Replication: &secretmanagerpb.Replication{
					Replication: &secretmanagerpb.Replication_Automatic_{
						Automatic: &secretmanagerpb.Replication_Automatic{},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
	}

	tests := []struct {
		name      string
		req       *secretmanagerpb.ListSecretsRequest
		wantErr   codes.Code
		wantCount int
	}{
		{
			name: "Success",
			req: &secretmanagerpb.ListSecretsRequest{
				Parent: "projects/test-project",
			},
			wantErr:   codes.OK,
			wantCount: 5,
		},
		{
			name: "MissingParent",
			req: &secretmanagerpb.ListSecretsRequest{
				Parent: "",
			},
			wantErr: codes.InvalidArgument,
		},
		{
			name: "WithPageSize",
			req: &secretmanagerpb.ListSecretsRequest{
				Parent:   "projects/test-project",
				PageSize: 2,
			},
			wantErr:   codes.OK,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.ListSecrets(ctx, tt.req)

			if tt.wantErr != codes.OK {
				if err == nil {
					t.Errorf("ListSecrets() error = nil, wantErr %v", tt.wantErr)
					return
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("ListSecrets() error is not a status error: %v", err)
					return
				}
				if st.Code() != tt.wantErr {
					t.Errorf("ListSecrets() error code = %v, wantErr %v", st.Code(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("ListSecrets() unexpected error = %v", err)
				return
			}
			if resp == nil {
				t.Error("ListSecrets() returned nil response")
				return
			}
			if len(resp.Secrets) != tt.wantCount {
				t.Errorf("ListSecrets() returned %d secrets, want %d", len(resp.Secrets), tt.wantCount)
			}
		})
	}
}

func TestServer_DeleteSecret(t *testing.T) {
	ctx := context.Background()
	server := NewServer()

	// Create a test secret first
	_, err := server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   "projects/test-project",
		SecretId: "test-secret-delete",
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	tests := []struct {
		name    string
		req     *secretmanagerpb.DeleteSecretRequest
		wantErr codes.Code
	}{
		{
			name: "Success",
			req: &secretmanagerpb.DeleteSecretRequest{
				Name: "projects/test-project/secrets/test-secret-delete",
			},
			wantErr: codes.OK,
		},
		{
			name: "MissingName",
			req: &secretmanagerpb.DeleteSecretRequest{
				Name: "",
			},
			wantErr: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req: &secretmanagerpb.DeleteSecretRequest{
				Name: "projects/test-project/secrets/nonexistent",
			},
			wantErr: codes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.DeleteSecret(ctx, tt.req)

			if tt.wantErr != codes.OK {
				if err == nil {
					t.Errorf("DeleteSecret() error = nil, wantErr %v", tt.wantErr)
					return
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("DeleteSecret() error is not a status error: %v", err)
					return
				}
				if st.Code() != tt.wantErr {
					t.Errorf("DeleteSecret() error code = %v, wantErr %v", st.Code(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("DeleteSecret() unexpected error = %v", err)
			}
		})
	}
}

func TestServer_AddSecretVersion(t *testing.T) {
	ctx := context.Background()
	server := NewServer()

	// Create a test secret first
	_, err := server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   "projects/test-project",
		SecretId: "test-secret-version",
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	tests := []struct {
		name    string
		req     *secretmanagerpb.AddSecretVersionRequest
		wantErr codes.Code
	}{
		{
			name: "Success",
			req: &secretmanagerpb.AddSecretVersionRequest{
				Parent: "projects/test-project/secrets/test-secret-version",
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte("test-data"),
				},
			},
			wantErr: codes.OK,
		},
		{
			name: "MissingParent",
			req: &secretmanagerpb.AddSecretVersionRequest{
				Parent: "",
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte("test-data"),
				},
			},
			wantErr: codes.InvalidArgument,
		},
		{
			name: "SecretNotFound",
			req: &secretmanagerpb.AddSecretVersionRequest{
				Parent: "projects/test-project/secrets/nonexistent",
				Payload: &secretmanagerpb.SecretPayload{
					Data: []byte("test-data"),
				},
			},
			wantErr: codes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := server.AddSecretVersion(ctx, tt.req)

			if tt.wantErr != codes.OK {
				if err == nil {
					t.Errorf("AddSecretVersion() error = nil, wantErr %v", tt.wantErr)
					return
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("AddSecretVersion() error is not a status error: %v", err)
					return
				}
				if st.Code() != tt.wantErr {
					t.Errorf("AddSecretVersion() error code = %v, wantErr %v", st.Code(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("AddSecretVersion() unexpected error = %v", err)
				return
			}
			if version == nil {
				t.Error("AddSecretVersion() returned nil version")
			}
		})
	}
}

func TestServer_AccessSecretVersion(t *testing.T) {
	ctx := context.Background()
	server := NewServer()

	// Create a test secret with version
	_, err := server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   "projects/test-project",
		SecretId: "test-secret-access",
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	_, err = server.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: "projects/test-project/secrets/test-secret-access",
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte("test-data"),
		},
	})
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	tests := []struct {
		name    string
		req     *secretmanagerpb.AccessSecretVersionRequest
		wantErr codes.Code
	}{
		{
			name: "Success",
			req: &secretmanagerpb.AccessSecretVersionRequest{
				Name: "projects/test-project/secrets/test-secret-access/versions/1",
			},
			wantErr: codes.OK,
		},
		{
			name: "SuccessLatest",
			req: &secretmanagerpb.AccessSecretVersionRequest{
				Name: "projects/test-project/secrets/test-secret-access/versions/latest",
			},
			wantErr: codes.OK,
		},
		{
			name: "MissingName",
			req: &secretmanagerpb.AccessSecretVersionRequest{
				Name: "",
			},
			wantErr: codes.InvalidArgument,
		},
		{
			name: "NotFound",
			req: &secretmanagerpb.AccessSecretVersionRequest{
				Name: "projects/test-project/secrets/nonexistent/versions/1",
			},
			wantErr: codes.NotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.AccessSecretVersion(ctx, tt.req)

			if tt.wantErr != codes.OK {
				if err == nil {
					t.Errorf("AccessSecretVersion() error = nil, wantErr %v", tt.wantErr)
					return
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("AccessSecretVersion() error is not a status error: %v", err)
					return
				}
				if st.Code() != tt.wantErr {
					t.Errorf("AccessSecretVersion() error code = %v, wantErr %v", st.Code(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("AccessSecretVersion() unexpected error = %v", err)
				return
			}
			if resp == nil {
				t.Error("AccessSecretVersion() returned nil response")
				return
			}
			if resp.Payload == nil || string(resp.Payload.Data) != "test-data" {
				t.Error("AccessSecretVersion() returned wrong payload data")
			}
		})
	}
}

func TestServer_UnimplementedMethods(t *testing.T) {
	ctx := context.Background()
	server := NewServer()

	t.Run("UpdateSecret", func(t *testing.T) {
		_, err := server.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{})
		if err == nil {
			t.Error("UpdateSecret() should return Unimplemented error")
			return
		}
		st, ok := status.FromError(err)
		if !ok {
			t.Errorf("UpdateSecret() error is not a status error: %v", err)
			return
		}
		if st.Code() != codes.Unimplemented {
			t.Errorf("UpdateSecret() error code = %v, want Unimplemented", st.Code())
		}
	})

	t.Run("GetSecretVersion", func(t *testing.T) {
		// GetSecretVersion is implemented but not used by vaultmux
		// Test that it returns NotFound for non-existent versions
		_, err := server.GetSecretVersion(ctx, &secretmanagerpb.GetSecretVersionRequest{
			Name: "projects/test-project/secrets/nonexistent/versions/1",
		})
		if err == nil {
			t.Error("GetSecretVersion() should return error for non-existent version")
			return
		}
		st, ok := status.FromError(err)
		if !ok {
			t.Errorf("GetSecretVersion() error is not a status error: %v", err)
			return
		}
		if st.Code() != codes.NotFound {
			t.Errorf("GetSecretVersion() error code = %v, want NotFound", st.Code())
		}
	})


	t.Run("DestroySecretVersion", func(t *testing.T) {
		_, err := server.DestroySecretVersion(ctx, &secretmanagerpb.DestroySecretVersionRequest{})
		if err == nil {
			t.Error("DestroySecretVersion() should return Unimplemented error")
			return
		}
		st, ok := status.FromError(err)
		if !ok {
			t.Errorf("DestroySecretVersion() error is not a status error: %v", err)
			return
		}
		if st.Code() != codes.Unimplemented {
			t.Errorf("DestroySecretVersion() error code = %v, want Unimplemented", st.Code())
		}
	})
}

func TestServer_Storage(t *testing.T) {
	server := NewServer()
	storage := server.Storage()

	if storage == nil {
		t.Error("Storage() returned nil")
	}
}

func TestServer_ListSecretVersions(t *testing.T) {
	ctx := context.Background()
	server := NewServer()

	// Create secret and add versions
	parent := "projects/test-project"
	secretID := "test-secret"
	_, err := server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   parent,
		SecretId: secretID,
		Secret:   &secretmanagerpb.Secret{},
	})
	if err != nil {
		t.Fatalf("CreateSecret() failed: %v", err)
	}

	secretName := fmt.Sprintf("%s/secrets/%s", parent, secretID)

	// Add 3 versions
	for i := 1; i <= 3; i++ {
		_, err := server.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
			Parent: secretName,
			Payload: &secretmanagerpb.SecretPayload{
				Data: []byte(fmt.Sprintf("secret-data-%d", i)),
			},
		})
		if err != nil {
			t.Fatalf("AddSecretVersion() failed: %v", err)
		}
	}

	t.Run("Success", func(t *testing.T) {
		resp, err := server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent: secretName,
		})
		if err != nil {
			t.Fatalf("ListSecretVersions() failed: %v", err)
		}
		if len(resp.Versions) != 3 {
			t.Errorf("ListSecretVersions() returned %d versions, want 3", len(resp.Versions))
		}
		for _, v := range resp.Versions {
			if v.State != secretmanagerpb.SecretVersion_ENABLED {
				t.Errorf("Version %s has state %v, want ENABLED", v.Name, v.State)
			}
		}
	})

	t.Run("MissingParent", func(t *testing.T) {
		_, err := server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{})
		if err == nil {
			t.Error("ListSecretVersions() should return error for missing parent")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("ListSecretVersions() error = %v, want InvalidArgument", err)
		}
	})

	t.Run("SecretNotFound", func(t *testing.T) {
		_, err := server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent: "projects/test-project/secrets/nonexistent",
		})
		if err == nil {
			t.Error("ListSecretVersions() should return error for nonexistent secret")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("ListSecretVersions() error = %v, want NotFound", err)
		}
	})

	t.Run("FilterByState", func(t *testing.T) {
		// Disable version 2
		_, err := server.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{
			Name: fmt.Sprintf("%s/versions/2", secretName),
		})
		if err != nil {
			t.Fatalf("DisableSecretVersion() failed: %v", err)
		}

		// Filter for ENABLED only
		resp, err := server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent: secretName,
			Filter: "state:ENABLED",
		})
		if err != nil {
			t.Fatalf("ListSecretVersions(filter=ENABLED) failed: %v", err)
		}
		if len(resp.Versions) != 2 {
			t.Errorf("ListSecretVersions(filter=ENABLED) returned %d versions, want 2", len(resp.Versions))
		}
		for _, v := range resp.Versions {
			if v.State != secretmanagerpb.SecretVersion_ENABLED {
				t.Errorf("Version %s has state %v, want ENABLED", v.Name, v.State)
			}
		}

		// Filter for DISABLED only
		resp, err = server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent: secretName,
			Filter: "state:DISABLED",
		})
		if err != nil {
			t.Fatalf("ListSecretVersions(filter=DISABLED) failed: %v", err)
		}
		if len(resp.Versions) != 1 {
			t.Errorf("ListSecretVersions(filter=DISABLED) returned %d versions, want 1", len(resp.Versions))
		}
		if resp.Versions[0].State != secretmanagerpb.SecretVersion_DISABLED {
			t.Errorf("Version has state %v, want DISABLED", resp.Versions[0].State)
		}

		// No filter - should return all 3
		resp, err = server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent: secretName,
		})
		if err != nil {
			t.Fatalf("ListSecretVersions(no filter) failed: %v", err)
		}
		if len(resp.Versions) != 3 {
			t.Errorf("ListSecretVersions(no filter) returned %d versions, want 3", len(resp.Versions))
		}
	})
}

func TestServer_DisableEnableSecretVersion(t *testing.T) {
	ctx := context.Background()
	server := NewServer()

	// Create secret and add version
	parent := "projects/test-project"
	secretID := "test-secret"
	_, err := server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   parent,
		SecretId: secretID,
		Secret:   &secretmanagerpb.Secret{},
	})
	if err != nil {
		t.Fatalf("CreateSecret() failed: %v", err)
	}

	secretName := fmt.Sprintf("%s/secrets/%s", parent, secretID)
	addResp, err := server.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretName,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte("secret-data"),
		},
	})
	if err != nil {
		t.Fatalf("AddSecretVersion() failed: %v", err)
	}
	versionName := addResp.Name

	t.Run("DisableVersion", func(t *testing.T) {
		resp, err := server.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{
			Name: versionName,
		})
		if err != nil {
			t.Fatalf("DisableSecretVersion() failed: %v", err)
		}
		if resp.State != secretmanagerpb.SecretVersion_DISABLED {
			t.Errorf("DisableSecretVersion() state = %v, want DISABLED", resp.State)
		}

		// Verify AccessSecretVersion fails for disabled version
		_, err = server.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
			Name: versionName,
		})
		if err == nil {
			t.Error("AccessSecretVersion() should fail for disabled version")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.FailedPrecondition {
			t.Errorf("AccessSecretVersion() error = %v, want FailedPrecondition", err)
		}
	})

	t.Run("EnableVersion", func(t *testing.T) {
		resp, err := server.EnableSecretVersion(ctx, &secretmanagerpb.EnableSecretVersionRequest{
			Name: versionName,
		})
		if err != nil {
			t.Fatalf("EnableSecretVersion() failed: %v", err)
		}
		if resp.State != secretmanagerpb.SecretVersion_ENABLED {
			t.Errorf("EnableSecretVersion() state = %v, want ENABLED", resp.State)
		}

		// Verify AccessSecretVersion works again
		accessResp, err := server.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
			Name: versionName,
		})
		if err != nil {
			t.Fatalf("AccessSecretVersion() failed after re-enabling: %v", err)
		}
		if string(accessResp.Payload.Data) != "secret-data" {
			t.Errorf("AccessSecretVersion() payload = %s, want secret-data", accessResp.Payload.Data)
		}
	})

	t.Run("DisableMissingName", func(t *testing.T) {
		_, err := server.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{})
		if err == nil {
			t.Error("DisableSecretVersion() should return error for missing name")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("DisableSecretVersion() error = %v, want InvalidArgument", err)
		}
	})

	t.Run("EnableMissingName", func(t *testing.T) {
		_, err := server.EnableSecretVersion(ctx, &secretmanagerpb.EnableSecretVersionRequest{})
		if err == nil {
			t.Error("EnableSecretVersion() should return error for missing name")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("EnableSecretVersion() error = %v, want InvalidArgument", err)
		}
	})

	t.Run("VersionNotFound", func(t *testing.T) {
		_, err := server.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{
			Name: secretName + "/versions/999",
		})
		if err == nil {
			t.Error("DisableSecretVersion() should return error for nonexistent version")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("DisableSecretVersion() error = %v, want NotFound", err)
		}
	})
}

func TestServer_VersionStateManagement(t *testing.T) {
	ctx := context.Background()
	server := NewServer()

	// Create secret and add multiple versions
	parent := "projects/test-project"
	secretID := "test-secret"
	_, err := server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   parent,
		SecretId: secretID,
		Secret:   &secretmanagerpb.Secret{},
	})
	if err != nil {
		t.Fatalf("CreateSecret() failed: %v", err)
	}

	secretName := fmt.Sprintf("%s/secrets/%s", parent, secretID)

	// Add 3 versions
	version1, _ := server.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretName,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte("data-1"),
		},
	})
	version2, _ := server.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretName,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte("data-2"),
		},
	})
	version3, _ := server.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretName,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte("data-3"),
		},
	})

	t.Run("LatestResolvesToHighestEnabled", func(t *testing.T) {
		// Latest should resolve to version 3 (highest)
		resp, err := server.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
			Name: secretName + "/versions/latest",
		})
		if err != nil {
			t.Fatalf("AccessSecretVersion(latest) failed: %v", err)
		}
		if string(resp.Payload.Data) != "data-3" {
			t.Errorf("AccessSecretVersion(latest) = %s, want data-3", resp.Payload.Data)
		}

		// Disable version 3
		_, err = server.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{
			Name: version3.Name,
		})
		if err != nil {
			t.Fatalf("DisableSecretVersion() failed: %v", err)
		}

		// Latest should now resolve to version 2
		resp, err = server.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
			Name: secretName + "/versions/latest",
		})
		if err != nil {
			t.Fatalf("AccessSecretVersion(latest) failed after disabling v3: %v", err)
		}
		if string(resp.Payload.Data) != "data-2" {
			t.Errorf("AccessSecretVersion(latest) = %s, want data-2", resp.Payload.Data)
		}
	})

	t.Run("SoftDeletePattern", func(t *testing.T) {
		// Disable all versions (soft delete pattern)
		_, err := server.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{
			Name: version1.Name,
		})
		if err != nil {
			t.Fatalf("DisableSecretVersion(v1) failed: %v", err)
		}
		_, err = server.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{
			Name: version2.Name,
		})
		if err != nil {
			t.Fatalf("DisableSecretVersion(v2) failed: %v", err)
		}

		// Accessing latest should fail (no enabled versions)
		_, err = server.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
			Name: secretName + "/versions/latest",
		})
		if err == nil {
			t.Error("AccessSecretVersion(latest) should fail when all versions disabled")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("AccessSecretVersion(latest) error = %v, want NotFound", err)
		}

		// ListSecretVersions should still show all versions
		listResp, err := server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent: secretName,
		})
		if err != nil {
			t.Fatalf("ListSecretVersions() failed: %v", err)
		}
		if len(listResp.Versions) != 3 {
			t.Errorf("ListSecretVersions() = %d versions, want 3", len(listResp.Versions))
		}
		for _, v := range listResp.Versions {
			if v.State != secretmanagerpb.SecretVersion_DISABLED {
				t.Errorf("Version %s state = %v, want DISABLED", v.Name, v.State)
			}
		}
	})
}
