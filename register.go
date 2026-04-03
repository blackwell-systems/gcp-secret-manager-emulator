// Package gcpemulator provides the composition entry point for the GCP Secret Manager Emulator.
//
// Register wires the Secret Manager gRPC service onto an existing grpc.Server,
// enabling use within the unified gcp-emulator or any custom composition layer.
// For standalone use, see cmd/server, cmd/server-rest, or cmd/server-dual.
package gcpemulator

import (
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc"

	"github.com/blackwell-systems/gcp-secret-manager-emulator/internal/server"
)

// Option configures the Secret Manager server at registration time.
type Option func(*options)

type options struct{}

// Register adds the Secret Manager gRPC service to grpcSrv.
// IAM enforcement is configured via the IAM_MODE and IAM_EMULATOR_HOST
// environment variables (same as the standalone binary).
// It does not start a listener — the caller owns the grpc.Server lifecycle.
func Register(grpcSrv *grpc.Server, opts ...Option) error {
	srv, err := server.NewServer()
	if err != nil {
		return err
	}
	secretmanagerpb.RegisterSecretManagerServiceServer(grpcSrv, srv)
	return nil
}
