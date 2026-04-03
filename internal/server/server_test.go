package server

import (
	"context"
	"fmt"
	"hash/crc32"
	"net"
	"strings"
	"testing"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	emulatorauth "github.com/blackwell-systems/gcp-emulator-auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func TestServer_CreateSecret(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

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
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Create a test secret first
	_, err = server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
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
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

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
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Create a test secret first
	_, err = server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
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
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Create a test secret first
	_, err = server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
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
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Create a test secret with version
	_, err = server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
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

func TestServer_UpdateSecret(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Create a secret first
	parent := "projects/test-project"
	secretID := "test-secret"
	secret, err := server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   parent,
		SecretId: secretID,
		Secret: &secretmanagerpb.Secret{
			Labels: map[string]string{
				"env": "dev",
			},
			Annotations: map[string]string{
				"note": "original",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateSecret() failed: %v", err)
	}

	secretName := secret.Name

	t.Run("Success_UpdateLabels", func(t *testing.T) {
		updated, err := server.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
			Secret: &secretmanagerpb.Secret{
				Name: secretName,
				Labels: map[string]string{
					"env":     "prod",
					"version": "1.0",
				},
			},
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{"labels"},
			},
		})
		if err != nil {
			t.Fatalf("UpdateSecret() failed: %v", err)
		}
		if updated.Labels["env"] != "prod" {
			t.Errorf("Labels not updated: got %v", updated.Labels)
		}
		if updated.Labels["version"] != "1.0" {
			t.Errorf("New label not added: got %v", updated.Labels)
		}
		// Annotations should remain unchanged
		if updated.Annotations["note"] != "original" {
			t.Errorf("Annotations changed unexpectedly: got %v", updated.Annotations)
		}
	})

	t.Run("Success_UpdateAnnotations", func(t *testing.T) {
		updated, err := server.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
			Secret: &secretmanagerpb.Secret{
				Name: secretName,
				Annotations: map[string]string{
					"note": "updated",
					"info": "new",
				},
			},
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{"annotations"},
			},
		})
		if err != nil {
			t.Fatalf("UpdateSecret() failed: %v", err)
		}
		if updated.Annotations["note"] != "updated" {
			t.Errorf("Annotations not updated: got %v", updated.Annotations)
		}
		if updated.Annotations["info"] != "new" {
			t.Errorf("New annotation not added: got %v", updated.Annotations)
		}
	})

	t.Run("MissingSecretName", func(t *testing.T) {
		_, err := server.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
			Secret: &secretmanagerpb.Secret{
				Labels: map[string]string{"env": "test"},
			},
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{"labels"},
			},
		})
		if err == nil {
			t.Error("UpdateSecret() should return error for missing secret name")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("UpdateSecret() error = %v, want InvalidArgument", err)
		}
	})

	t.Run("MissingUpdateMask", func(t *testing.T) {
		_, err := server.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
			Secret: &secretmanagerpb.Secret{
				Name:   secretName,
				Labels: map[string]string{"env": "test"},
			},
		})
		if err == nil {
			t.Error("UpdateSecret() should return error for missing update_mask")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("UpdateSecret() error = %v, want InvalidArgument", err)
		}
	})

	t.Run("SecretNotFound", func(t *testing.T) {
		_, err := server.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
			Secret: &secretmanagerpb.Secret{
				Name:   "projects/test-project/secrets/nonexistent",
				Labels: map[string]string{"env": "test"},
			},
			UpdateMask: &fieldmaskpb.FieldMask{
				Paths: []string{"labels"},
			},
		})
		if err == nil {
			t.Error("UpdateSecret() should return error for nonexistent secret")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("UpdateSecret() error = %v, want NotFound", err)
		}
	})
}

func TestServer_DestroySecretVersion(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Create secret and add version
	parent := "projects/test-project"
	secretID := "test-secret"
	_, err = server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   parent,
		SecretId: secretID,
		Secret:   &secretmanagerpb.Secret{},
	})
	if err != nil {
		t.Fatalf("CreateSecret() failed: %v", err)
	}

	secretName := fmt.Sprintf("%s/secrets/%s", parent, secretID)
	version, err := server.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: secretName,
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte("test-data"),
		},
	})
	if err != nil {
		t.Fatalf("AddSecretVersion() failed: %v", err)
	}

	versionName := version.Name

	t.Run("Success", func(t *testing.T) {
		destroyed, err := server.DestroySecretVersion(ctx, &secretmanagerpb.DestroySecretVersionRequest{
			Name: versionName,
		})
		if err != nil {
			t.Fatalf("DestroySecretVersion() failed: %v", err)
		}
		if destroyed.State != secretmanagerpb.SecretVersion_DESTROYED {
			t.Errorf("Version state = %v, want DESTROYED", destroyed.State)
		}

		// Verify payload is destroyed
		_, err = server.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
			Name: versionName,
		})
		if err == nil {
			t.Error("AccessSecretVersion() should fail for destroyed version")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.FailedPrecondition {
			t.Errorf("AccessSecretVersion() error = %v, want FailedPrecondition", err)
		}
	})

	t.Run("Idempotent", func(t *testing.T) {
		// Destroying again should succeed (idempotent)
		destroyed, err := server.DestroySecretVersion(ctx, &secretmanagerpb.DestroySecretVersionRequest{
			Name: versionName,
		})
		if err != nil {
			t.Fatalf("DestroySecretVersion() second call failed: %v", err)
		}
		if destroyed.State != secretmanagerpb.SecretVersion_DESTROYED {
			t.Errorf("Version state = %v, want DESTROYED", destroyed.State)
		}
	})

	t.Run("MissingName", func(t *testing.T) {
		_, err := server.DestroySecretVersion(ctx, &secretmanagerpb.DestroySecretVersionRequest{})
		if err == nil {
			t.Error("DestroySecretVersion() should return error for missing name")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("DestroySecretVersion() error = %v, want InvalidArgument", err)
		}
	})

	t.Run("VersionNotFound", func(t *testing.T) {
		_, err := server.DestroySecretVersion(ctx, &secretmanagerpb.DestroySecretVersionRequest{
			Name: "projects/test-project/secrets/test-secret/versions/999",
		})
		if err == nil {
			t.Error("DestroySecretVersion() should return error for nonexistent version")
			return
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.NotFound {
			t.Errorf("DestroySecretVersion() error = %v, want NotFound", err)
		}
	})
}

func TestServer_GetSecretVersion(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	t.Run("NotFound", func(t *testing.T) {
		// GetSecretVersion is implemented but not commonly used
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
}

func TestServer_Storage(t *testing.T) {
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	storage := server.Storage()

	if storage == nil {
		t.Error("Storage() returned nil")
	}
}

func TestServer_ListSecretVersions(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Create secret and add versions
	parent := "projects/test-project"
	secretID := "test-secret"
	_, err = server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
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

	t.Run("Pagination", func(t *testing.T) {
		// Add more versions for pagination test (total 150 versions)
		for i := 4; i <= 150; i++ {
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

		// Test small page size
		resp, err := server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent:   secretName,
			PageSize: 10,
		})
		if err != nil {
			t.Fatalf("ListSecretVersions() failed: %v", err)
		}
		if len(resp.Versions) != 10 {
			t.Errorf("First page returned %d versions, want 10", len(resp.Versions))
		}
		if resp.NextPageToken == "" {
			t.Error("First page should have NextPageToken")
		}

		// Test second page
		resp2, err := server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent:    secretName,
			PageSize:  10,
			PageToken: resp.NextPageToken,
		})
		if err != nil {
			t.Fatalf("ListSecretVersions() page 2 failed: %v", err)
		}
		if len(resp2.Versions) != 10 {
			t.Errorf("Second page returned %d versions, want 10", len(resp2.Versions))
		}
		if resp2.NextPageToken == "" {
			t.Error("Second page should have NextPageToken (150 total versions)")
		}

		// Test default page size (should be 100)
		respDefault, err := server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent: secretName,
		})
		if err != nil {
			t.Fatalf("ListSecretVersions() with default page size failed: %v", err)
		}
		if len(respDefault.Versions) != 100 {
			t.Errorf("Default page size returned %d versions, want 100", len(respDefault.Versions))
		}
		if respDefault.NextPageToken == "" {
			t.Error("Default page should have NextPageToken (150 total versions)")
		}

		// Get second page with default size
		respDefault2, err := server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent:    secretName,
			PageToken: respDefault.NextPageToken,
		})
		if err != nil {
			t.Fatalf("ListSecretVersions() default page 2 failed: %v", err)
		}
		if len(respDefault2.Versions) != 50 {
			t.Errorf("Second default page returned %d versions, want 50 (remaining)", len(respDefault2.Versions))
		}
		if respDefault2.NextPageToken != "" {
			t.Error("Last page should have empty NextPageToken")
		}

		// Verify all versions are unique and complete
		allVersions := make(map[string]bool)
		pageToken := ""
		totalCount := 0
		for {
			resp, err := server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
				Parent:    secretName,
				PageSize:  25,
				PageToken: pageToken,
			})
			if err != nil {
				t.Fatalf("ListSecretVersions() iteration failed: %v", err)
			}

			for _, v := range resp.Versions {
				if allVersions[v.Name] {
					t.Errorf("Duplicate version in pagination: %s", v.Name)
				}
				allVersions[v.Name] = true
				totalCount++
			}

			if resp.NextPageToken == "" {
				break
			}
			pageToken = resp.NextPageToken
		}

		if totalCount != 150 {
			t.Errorf("Total versions across all pages: %d, want 150", totalCount)
		}
	})

	t.Run("FilterByState", func(t *testing.T) {
		// Note: Pagination test added versions 4-150, so we now have 150 total versions
		// Disable version 2
		_, err := server.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{
			Name: fmt.Sprintf("%s/versions/2", secretName),
		})
		if err != nil {
			t.Fatalf("DisableSecretVersion() failed: %v", err)
		}

		// Filter for ENABLED only (should be 149: versions 1,3-150)
		// Default page size is 100, so we'll get first page
		resp, err := server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent: secretName,
			Filter: "state:ENABLED",
		})
		if err != nil {
			t.Fatalf("ListSecretVersions(filter=ENABLED) failed: %v", err)
		}
		if len(resp.Versions) != 100 {
			t.Errorf("ListSecretVersions(filter=ENABLED) first page returned %d versions, want 100", len(resp.Versions))
		}
		for _, v := range resp.Versions {
			if v.State != secretmanagerpb.SecretVersion_ENABLED {
				t.Errorf("Version %s has state %v, want ENABLED", v.Name, v.State)
			}
		}

		// Count all ENABLED versions across pages
		enabledCount := 0
		pageToken := ""
		for {
			resp, err := server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
				Parent:    secretName,
				Filter:    "state:ENABLED",
				PageSize:  50,
				PageToken: pageToken,
			})
			if err != nil {
				t.Fatalf("ListSecretVersions(filter=ENABLED) failed: %v", err)
			}
			enabledCount += len(resp.Versions)
			if resp.NextPageToken == "" {
				break
			}
			pageToken = resp.NextPageToken
		}
		if enabledCount != 149 {
			t.Errorf("Total ENABLED versions: %d, want 149", enabledCount)
		}

		// Filter for DISABLED only (should be 1: version 2)
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

		// No filter - should return all 150 versions (first page of 100)
		resp, err = server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{
			Parent: secretName,
		})
		if err != nil {
			t.Fatalf("ListSecretVersions(no filter) failed: %v", err)
		}
		if len(resp.Versions) != 100 {
			t.Errorf("ListSecretVersions(no filter) returned %d versions, want 100 (first page)", len(resp.Versions))
		}
	})
}

func TestServer_DisableEnableSecretVersion(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Create secret and add version
	parent := "projects/test-project"
	secretID := "test-secret"
	_, err = server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
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
	server, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	// Create secret and add multiple versions
	parent := "projects/test-project"
	secretID := "test-secret"
	_, err = server.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
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

// denyAllIAMServer is a minimal IAMPolicyServer that denies all permissions.
type denyAllIAMServer struct {
	iampb.UnimplementedIAMPolicyServer
}

func (d *denyAllIAMServer) TestIamPermissions(_ context.Context, _ *iampb.TestIamPermissionsRequest) (*iampb.TestIamPermissionsResponse, error) {
	// Return empty permissions slice — none are granted.
	return &iampb.TestIamPermissionsResponse{Permissions: []string{}}, nil
}

// startDenyAllIAMServer starts a local gRPC IAM server that denies everything.
func startDenyAllIAMServer(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	grpcSrv := grpc.NewServer()
	iampb.RegisterIAMPolicyServer(grpcSrv, &denyAllIAMServer{})
	go func() {
		if serveErr := grpcSrv.Serve(lis); serveErr != nil && serveErr != grpc.ErrServerStopped {
			t.Logf("denyAllIAMServer error: %v", serveErr)
		}
	}()
	t.Cleanup(func() { grpcSrv.Stop() })
	return lis.Addr().String()
}

// TestPermissionDenied_ErrorMessageFormat verifies that when IAM denies a
// request, the error message contains the principal, permission, and resource.
func TestPermissionDenied_ErrorMessageFormat(t *testing.T) {
	addr := startDenyAllIAMServer(t)

	iamClient, err := emulatorauth.NewClient(addr, emulatorauth.AuthModeStrict, "test")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// Create a base server for storage setup (IAM disabled).
	setupServer, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer (setup): %v", err)
	}
	_, createErr := setupServer.storage.CreateSecret(context.Background(), "projects/test-project", "my-secret", nil)
	if createErr != nil {
		t.Fatalf("storage.CreateSecret: %v", createErr)
	}

	// Build a Server directly with IAM enabled, sharing the pre-populated storage.
	s := &Server{
		storage:   setupServer.storage,
		iamClient: iamClient,
		iamMode:   emulatorauth.AuthModeStrict,
	}

	// Inject a principal into the incoming gRPC metadata context.
	md := metadata.Pairs(emulatorauth.PrincipalMetadataKey, "user:alice@example.com")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, getErr := s.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{
		Name: "projects/test-project/secrets/my-secret",
	})
	if getErr == nil {
		t.Fatal("GetSecret() expected PermissionDenied error, got nil")
	}

	st, ok := status.FromError(getErr)
	if !ok {
		t.Fatalf("error is not a gRPC status: %v", getErr)
	}
	if st.Code() != codes.PermissionDenied {
		t.Errorf("error code = %v, want PermissionDenied", st.Code())
	}

	msg := st.Message()
	if !strings.Contains(msg, "user:alice@example.com") {
		t.Errorf("error message %q does not contain principal", msg)
	}
	if !strings.Contains(msg, "secretmanager.secrets.get") {
		t.Errorf("error message %q does not contain permission", msg)
	}
	if !strings.Contains(msg, "projects/test-project/secrets/my-secret") {
		t.Errorf("error message %q does not contain resource", msg)
	}
}

func TestServer_AddSecretVersion_Crc32cValidation(t *testing.T) {
	ctx := context.Background()
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	_, err = srv.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent: "projects/p", SecretId: "crc-test",
		Secret: &secretmanagerpb.Secret{},
	})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}

	data := []byte("payload data")
	table := crc32.MakeTable(crc32.Castagnoli)
	correct := int64(crc32.Checksum(data, table))

	// Correct checksum: expect success
	_, err = srv.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: "projects/p/secrets/crc-test",
		Payload: &secretmanagerpb.SecretPayload{
			Data:       data,
			DataCrc32C: &correct,
		},
	})
	if err != nil {
		t.Fatalf("AddSecretVersion correct crc32c: unexpected error %v", err)
	}

	// Wrong checksum: expect DataLoss
	wrongCrc := correct + 1
	_, err = srv.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: "projects/p/secrets/crc-test",
		Payload: &secretmanagerpb.SecretPayload{
			Data:       data,
			DataCrc32C: &wrongCrc,
		},
	})
	if err == nil {
		t.Fatal("AddSecretVersion wrong crc32c: expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.DataLoss {
		t.Errorf("AddSecretVersion wrong crc32c: got code %v, want DataLoss", st.Code())
	}
}

func TestServer_DeleteSecret_EtagValidation(t *testing.T) {
	ctx := context.Background()
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	created, err := srv.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent: "projects/p", SecretId: "etag-del",
		Secret: &secretmanagerpb.Secret{},
	})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}
	correctEtag := created.GetEtag()
	if correctEtag == "" {
		t.Fatal("CreateSecret returned empty etag")
	}

	// Wrong etag: expect Aborted
	_, err = srv.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{
		Name: created.Name, Etag: "wrong-etag",
	})
	if err == nil {
		t.Fatal("DeleteSecret wrong etag: expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Aborted {
		t.Errorf("DeleteSecret wrong etag: got code %v, want Aborted", st.Code())
	}

	// Correct etag: expect success
	_, err = srv.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{
		Name: created.Name, Etag: correctEtag,
	})
	if err != nil {
		t.Fatalf("DeleteSecret correct etag: unexpected error %v", err)
	}
}

func TestServer_UpdateSecret_EtagValidation(t *testing.T) {
	ctx := context.Background()
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	created, err := srv.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent: "projects/p", SecretId: "etag-upd",
		Secret: &secretmanagerpb.Secret{},
	})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}

	// Wrong etag: expect Aborted
	_, err = srv.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
		Secret: &secretmanagerpb.Secret{Name: created.Name, Etag: "wrong"},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"labels"}},
	})
	if err == nil {
		t.Fatal("UpdateSecret wrong etag: expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Aborted {
		t.Errorf("UpdateSecret wrong etag: got code %v, want Aborted", st.Code())
	}

	// Correct etag: expect success
	_, err = srv.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
		Secret: &secretmanagerpb.Secret{
			Name: created.Name, Etag: created.GetEtag(),
			Labels: map[string]string{"updated": "true"},
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"labels"}},
	})
	if err != nil {
		t.Fatalf("UpdateSecret correct etag: unexpected error %v", err)
	}
}

func TestServer_VersionEtagValidation(t *testing.T) {
	ctx := context.Background()
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	_, err = srv.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent: "projects/p", SecretId: "etag-ver",
		Secret: &secretmanagerpb.Secret{},
	})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}
	parent := "projects/p/secrets/etag-ver"
	sv, err := srv.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent:  parent,
		Payload: &secretmanagerpb.SecretPayload{Data: []byte("data")},
	})
	if err != nil {
		t.Fatalf("AddSecretVersion: %v", err)
	}
	correctEtag := sv.GetEtag()
	versionName := sv.Name

	// DisableSecretVersion with wrong etag: expect Aborted
	_, err = srv.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{
		Name: versionName, Etag: "bad-etag",
	})
	if err == nil {
		t.Fatal("DisableSecretVersion wrong etag: expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Aborted {
		t.Errorf("DisableSecretVersion wrong etag: got %v, want Aborted", st.Code())
	}

	// DisableSecretVersion with correct etag: expect success
	disabled, err := srv.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{
		Name: versionName, Etag: correctEtag,
	})
	if err != nil {
		t.Fatalf("DisableSecretVersion correct etag: %v", err)
	}

	// Re-enable then destroy with correct etag
	_, err = srv.EnableSecretVersion(ctx, &secretmanagerpb.EnableSecretVersionRequest{
		Name: versionName, Etag: disabled.GetEtag(),
	})
	if err != nil {
		t.Fatalf("EnableSecretVersion: %v", err)
	}
}

func TestServer_IAMPolicy(t *testing.T) {
	ctx := context.Background()
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	resource := "projects/p/secrets/iam-test"
	_, err = srv.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent: "projects/p", SecretId: "iam-test",
		Secret: &secretmanagerpb.Secret{},
	})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}

	// GetIamPolicy on new secret: should return empty policy, not Unimplemented
	policy, err := srv.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{Resource: resource})
	if err != nil {
		t.Fatalf("GetIamPolicy: unexpected error %v", err)
	}
	if policy == nil {
		t.Fatal("GetIamPolicy: returned nil policy")
	}

	// SetIamPolicy: store a binding
	setResp, err := srv.SetIamPolicy(ctx, &iampb.SetIamPolicyRequest{
		Resource: resource,
		Policy: &iampb.Policy{
			Bindings: []*iampb.Binding{
				{Role: "roles/secretmanager.viewer", Members: []string{"user:test@example.com"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("SetIamPolicy: unexpected error %v", err)
	}
	if len(setResp.GetBindings()) != 1 {
		t.Errorf("SetIamPolicy: got %d bindings, want 1", len(setResp.GetBindings()))
	}

	// GetIamPolicy should now return the stored binding
	getResp, err := srv.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{Resource: resource})
	if err != nil {
		t.Fatalf("GetIamPolicy after set: %v", err)
	}
	if len(getResp.GetBindings()) != 1 {
		t.Errorf("GetIamPolicy after set: got %d bindings, want 1", len(getResp.GetBindings()))
	}

	// TestIamPermissions: grant all when IAM is disabled
	permResp, err := srv.TestIamPermissions(ctx, &iampb.TestIamPermissionsRequest{
		Resource:    resource,
		Permissions: []string{"secretmanager.versions.access", "secretmanager.secrets.get"},
	})
	if err != nil {
		t.Fatalf("TestIamPermissions: unexpected error %v", err)
	}
	if len(permResp.GetPermissions()) != 2 {
		t.Errorf("TestIamPermissions: got %d permissions, want 2", len(permResp.GetPermissions()))
	}
}

func TestServer_UpdateSecret_VersionAliases(t *testing.T) {
	ctx := context.Background()
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	_, err = srv.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent: "projects/p", SecretId: "alias-test",
		Secret: &secretmanagerpb.Secret{},
	})
	if err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}
	parent := "projects/p/secrets/alias-test"
	_, err = srv.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent:  parent,
		Payload: &secretmanagerpb.SecretPayload{Data: []byte("version1data")},
	})
	if err != nil {
		t.Fatalf("AddSecretVersion: %v", err)
	}

	// Set version alias via UpdateSecret
	_, err = srv.UpdateSecret(ctx, &secretmanagerpb.UpdateSecretRequest{
		Secret: &secretmanagerpb.Secret{
			Name:           parent,
			VersionAliases: map[string]int64{"myalias": 1},
		},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"version_aliases"}},
	})
	if err != nil {
		t.Fatalf("UpdateSecret version_aliases: %v", err)
	}

	// Resolve alias via GetSecretVersion
	sv, err := srv.GetSecretVersion(ctx, &secretmanagerpb.GetSecretVersionRequest{
		Name: parent + "/versions/myalias",
	})
	if err != nil {
		t.Fatalf("GetSecretVersion(myalias): %v", err)
	}
	if sv.GetName() != parent+"/versions/1" {
		t.Errorf("GetSecretVersion alias: name = %q, want %q", sv.GetName(), parent+"/versions/1")
	}
}

func TestServer_ListSecrets_Filter(t *testing.T) {
	ctx := context.Background()
	srv, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	for _, id := range []string{"alpha-1", "alpha-2", "beta-1"} {
		_, err := srv.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
			Parent: "projects/p", SecretId: id,
			Secret: &secretmanagerpb.Secret{},
		})
		if err != nil {
			t.Fatalf("CreateSecret(%s): %v", id, err)
		}
	}

	resp, err := srv.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{
		Parent: "projects/p", Filter: "name:alpha",
	})
	if err != nil {
		t.Fatalf("ListSecrets(filter=name:alpha): %v", err)
	}
	if len(resp.Secrets) != 2 {
		t.Errorf("ListSecrets filter: got %d secrets, want 2", len(resp.Secrets))
	}
}

// TestPermissionDenied_NoPrincipal verifies the "(no principal)" rendering
// when no principal header is present in the context.
func TestPermissionDenied_NoPrincipal(t *testing.T) {
	addr := startDenyAllIAMServer(t)

	iamClient, err := emulatorauth.NewClient(addr, emulatorauth.AuthModeStrict, "test")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	setupServer, err := NewServer()
	if err != nil {
		t.Fatalf("NewServer (setup): %v", err)
	}
	_, createErr := setupServer.storage.CreateSecret(context.Background(), "projects/p", "s", nil)
	if createErr != nil {
		t.Fatalf("storage.CreateSecret: %v", createErr)
	}

	s := &Server{
		storage:   setupServer.storage,
		iamClient: iamClient,
		iamMode:   emulatorauth.AuthModeStrict,
	}

	// Context with no principal injected.
	ctx := context.Background()

	_, getErr := s.GetSecret(ctx, &secretmanagerpb.GetSecretRequest{
		Name: "projects/p/secrets/s",
	})
	if getErr == nil {
		t.Fatal("GetSecret() expected PermissionDenied error, got nil")
	}

	st, ok := status.FromError(getErr)
	if !ok {
		t.Fatalf("error is not a gRPC status: %v", getErr)
	}
	if st.Code() != codes.PermissionDenied {
		t.Errorf("error code = %v, want PermissionDenied", st.Code())
	}

	msg := st.Message()
	if !strings.Contains(msg, "(no principal)") {
		t.Errorf("error message %q does not contain '(no principal)'", msg)
	}
}
