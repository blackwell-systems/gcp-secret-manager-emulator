package server

import (
	"context"
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

	t.Run("ListSecretVersions", func(t *testing.T) {
		_, err := server.ListSecretVersions(ctx, &secretmanagerpb.ListSecretVersionsRequest{})
		if err == nil {
			t.Error("ListSecretVersions() should return Unimplemented error")
			return
		}
		st, ok := status.FromError(err)
		if !ok {
			t.Errorf("ListSecretVersions() error is not a status error: %v", err)
			return
		}
		if st.Code() != codes.Unimplemented {
			t.Errorf("ListSecretVersions() error code = %v, want Unimplemented", st.Code())
		}
	})

	t.Run("EnableSecretVersion", func(t *testing.T) {
		_, err := server.EnableSecretVersion(ctx, &secretmanagerpb.EnableSecretVersionRequest{})
		if err == nil {
			t.Error("EnableSecretVersion() should return Unimplemented error")
			return
		}
		st, ok := status.FromError(err)
		if !ok {
			t.Errorf("EnableSecretVersion() error is not a status error: %v", err)
			return
		}
		if st.Code() != codes.Unimplemented {
			t.Errorf("EnableSecretVersion() error code = %v, want Unimplemented", st.Code())
		}
	})

	t.Run("DisableSecretVersion", func(t *testing.T) {
		_, err := server.DisableSecretVersion(ctx, &secretmanagerpb.DisableSecretVersionRequest{})
		if err == nil {
			t.Error("DisableSecretVersion() should return Unimplemented error")
			return
		}
		st, ok := status.FromError(err)
		if !ok {
			t.Errorf("DisableSecretVersion() error is not a status error: %v", err)
			return
		}
		if st.Code() != codes.Unimplemented {
			t.Errorf("DisableSecretVersion() error code = %v, want Unimplemented", st.Code())
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
