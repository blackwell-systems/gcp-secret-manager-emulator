package gcpemulator

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	iampb "cloud.google.com/go/iam/apiv1/iampb"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/blackwell-systems/gcp-secret-manager-emulator/internal/server"
)

// setIAMTestPolicy sets a policy on the IAM emulator granting role to member on resource.
func setIAMTestPolicy(host, resource, role, member string) error {
	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("connect to IAM emulator at %s: %w", host, err)
	}
	defer conn.Close()

	client := iampb.NewIAMPolicyClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.SetIamPolicy(ctx, &iampb.SetIamPolicyRequest{
		Resource: resource,
		Policy: &iampb.Policy{
			Bindings: []*iampb.Binding{
				{Role: role, Members: []string{member}},
			},
		},
	})
	return err
}

// clearIAMTestPolicy removes all bindings on resource.
func clearIAMTestPolicy(host, resource string) error {
	conn, err := grpc.NewClient(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := iampb.NewIAMPolicyClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.SetIamPolicy(ctx, &iampb.SetIamPolicyRequest{
		Resource: resource,
		Policy:   &iampb.Policy{}, // empty = no bindings
	})
	return err
}

func TestIAMIntegration(t *testing.T) {
	if os.Getenv("IAM_EMULATOR_HOST") == "" {
		t.Skip("Skipping IAM integration tests - IAM_EMULATOR_HOST not set")
	}

	iamHost := os.Getenv("IAM_EMULATOR_HOST")

	tests := []struct {
		name          string
		iamMode       string
		principal     string
		clearPolicy   bool // clear projects/test policy before test (ensures clean state)
		setupPolicy   bool // grant user:admin@example.com secretmanager.admin on projects/test
		operation     func(secretmanagerpb.SecretManagerServiceClient, context.Context) error
		expectError   bool
		expectedCode  codes.Code
	}{
		{
			// Permissive mode only fails-open on *connectivity* errors.
			// When IAM is reachable and denies (no policy for empty principal),
			// permissive mode still denies — same as strict.
			name:        "permissive mode - deny without principal (policy denied)",
			iamMode:     "permissive",
			principal:   "",
			clearPolicy: true,
			operation: func(client secretmanagerpb.SecretManagerServiceClient, ctx context.Context) error {
				_, err := client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
					Parent:   "projects/test",
					SecretId: "test-secret-1",
					Secret: &secretmanagerpb.Secret{
						Replication: &secretmanagerpb.Replication{
							Replication: &secretmanagerpb.Replication_Automatic_{
								Automatic: &secretmanagerpb.Replication_Automatic{},
							},
						},
					},
				})
				return err
			},
			expectError:  true,
			expectedCode: codes.PermissionDenied,
		},
		{
			name:        "strict mode - deny without principal",
			iamMode:     "strict",
			principal:   "",
			clearPolicy: true,
			operation: func(client secretmanagerpb.SecretManagerServiceClient, ctx context.Context) error {
				_, err := client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
					Parent:   "projects/test",
					SecretId: "test-secret-2",
					Secret: &secretmanagerpb.Secret{
						Replication: &secretmanagerpb.Replication{
							Replication: &secretmanagerpb.Replication_Automatic_{
								Automatic: &secretmanagerpb.Replication_Automatic{},
							},
						},
					},
				})
				return err
			},
			expectError:  true,
			expectedCode: codes.PermissionDenied,
		},
		{
			name:        "strict mode - allow with authorized principal",
			iamMode:     "strict",
			principal:   "user:admin@example.com",
			setupPolicy: true,
			operation: func(client secretmanagerpb.SecretManagerServiceClient, ctx context.Context) error {
				_, err := client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
					Parent:   "projects/test",
					SecretId: "test-secret-3",
					Secret: &secretmanagerpb.Secret{
						Replication: &secretmanagerpb.Replication{
							Replication: &secretmanagerpb.Replication_Automatic_{
								Automatic: &secretmanagerpb.Replication_Automatic{},
							},
						},
					},
				})
				return err
			},
			expectError: false,
		},
		{
			name:        "strict mode - access secret version requires permission",
			iamMode:     "strict",
			principal:   "user:admin@example.com",
			setupPolicy: true,
			operation: func(client secretmanagerpb.SecretManagerServiceClient, ctx context.Context) error {
				_, err := client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
					Parent:   "projects/test",
					SecretId: "test-secret-4",
					Secret: &secretmanagerpb.Secret{
						Replication: &secretmanagerpb.Replication{
							Replication: &secretmanagerpb.Replication_Automatic_{
								Automatic: &secretmanagerpb.Replication_Automatic{},
							},
						},
					},
				})
				if err != nil {
					return fmt.Errorf("setup failed: %w", err)
				}

				_, err = client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
					Parent: "projects/test/secrets/test-secret-4",
					Payload: &secretmanagerpb.SecretPayload{
						Data: []byte("test-data"),
					},
				})
				if err != nil {
					return fmt.Errorf("setup failed: %w", err)
				}

				_, err = client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
					Name: "projects/test/secrets/test-secret-4/versions/1",
				})
				return err
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.clearPolicy {
				if err := clearIAMTestPolicy(iamHost, "projects/test"); err != nil {
					t.Fatalf("failed to clear IAM test policy: %v", err)
				}
			}
			if tt.setupPolicy {
				if err := setIAMTestPolicy(iamHost, "projects/test", "roles/secretmanager.admin", "user:admin@example.com"); err != nil {
					t.Fatalf("failed to set IAM test policy: %v", err)
				}
				t.Cleanup(func() { _ = clearIAMTestPolicy(iamHost, "projects/test") })
			}

			os.Setenv("IAM_MODE", tt.iamMode)
			defer os.Unsetenv("IAM_MODE")

			_, lis, cleanup := setupTestServerForIAM(t)
			defer cleanup()

			conn, cleanupClient := setupTestClient(t, lis)
			defer cleanupClient()

			client := secretmanagerpb.NewSecretManagerServiceClient(conn)

			ctx := context.Background()
			if tt.principal != "" {
				ctx = metadata.AppendToOutgoingContext(ctx, "x-emulator-principal", tt.principal)
			}

			err := tt.operation(client, ctx)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
					return
				}

				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("Expected gRPC status error, got: %v", err)
					return
				}

				if st.Code() != tt.expectedCode {
					t.Errorf("Expected code %v, got %v", tt.expectedCode, st.Code())
				}
			} else {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
				}
			}
		})
	}
}

func TestIAMModeOff(t *testing.T) {
	os.Setenv("IAM_MODE", "off")
	defer os.Unsetenv("IAM_MODE")

	_, lis, cleanup := setupTestServerForIAM(t)
	defer cleanup()

	conn, cleanupClient := setupTestClient(t, lis)
	defer cleanupClient()

	client := secretmanagerpb.NewSecretManagerServiceClient(conn)
	ctx := context.Background()

	_, err := client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   "projects/test",
		SecretId: "test-secret-off",
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	})

	if err != nil {
		t.Errorf("Expected success with IAM_MODE=off, got error: %v", err)
	}
}

func TestIAMPermissiveVsStrict(t *testing.T) {
	if os.Getenv("IAM_EMULATOR_HOST") == "" {
		t.Skip("Skipping IAM integration tests - IAM_EMULATOR_HOST not set")
	}

	tests := []struct {
		name        string
		iamMode     string
		expectError bool
	}{
		{
			name:        "permissive mode - fail open on connectivity error",
			iamMode:     "permissive",
			expectError: false,
		},
		{
			name:        "strict mode - fail closed on connectivity error",
			iamMode:     "strict",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("IAM_MODE", tt.iamMode)
			defer os.Unsetenv("IAM_MODE")

			originalHost := os.Getenv("IAM_EMULATOR_HOST")
			os.Setenv("IAM_EMULATOR_HOST", "localhost:65535")
			defer os.Setenv("IAM_EMULATOR_HOST", originalHost)

			_, lis, cleanup := setupTestServerForIAM(t)
			defer cleanup()

			conn, cleanupClient := setupTestClient(t, lis)
			defer cleanupClient()

			client := secretmanagerpb.NewSecretManagerServiceClient(conn)
			ctx := context.Background()

			_, err := client.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
				Parent:   "projects/test",
				SecretId: "test-secret",
				Secret: &secretmanagerpb.Secret{
					Replication: &secretmanagerpb.Replication{
						Replication: &secretmanagerpb.Replication_Automatic_{
							Automatic: &secretmanagerpb.Replication_Automatic{},
						},
					},
				},
			})

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error in %s mode, got nil", tt.iamMode)
				}
			} else {
				if err != nil {
					t.Errorf("Expected success in %s mode (fail-open), got error: %v", tt.iamMode, err)
				}
			}
		})
	}
}

func setupTestServerForIAM(t *testing.T) (*grpc.Server, *bufconn.Listener, func()) {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)

	grpcServer := grpc.NewServer()
	smServer, err := server.NewServer()
	if err != nil {
		t.Fatalf("Failed to create Secret Manager server: %v", err)
	}
	secretmanagerpb.RegisterSecretManagerServiceServer(grpcServer, smServer)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Logf("Server exited: %v", err)
		}
	}()

	cleanup := func() {
		grpcServer.Stop()
		lis.Close()
	}

	return grpcServer, lis, cleanup
}

func setupTestClient(t *testing.T, lis *bufconn.Listener) (*grpc.ClientConn, func()) {
	t.Helper()

	conn, err := grpc.NewClient("passthrough://bufconn",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufconn: %v", err)
	}

	cleanup := func() {
		conn.Close()
	}

	return conn, cleanup
}
