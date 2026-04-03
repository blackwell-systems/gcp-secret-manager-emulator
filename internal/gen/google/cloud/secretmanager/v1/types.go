// Package secretmanagerv1 provides grpc-gateway handler registration for the
// Google Cloud Secret Manager v1 API.
//
// This file re-exports gRPC service interfaces and request/response types
// from cloud.google.com/go/secretmanager/apiv1/secretmanagerpb so that the
// generated gateway file (service.pb.gw.go) compiles without also generating pb.go.
package secretmanagerv1

import (
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc"
)

// gRPC service interfaces and constructors.
type SecretManagerServiceClient = secretmanagerpb.SecretManagerServiceClient
type SecretManagerServiceServer = secretmanagerpb.SecretManagerServiceServer

func NewSecretManagerServiceClient(cc grpc.ClientConnInterface) SecretManagerServiceClient {
	return secretmanagerpb.NewSecretManagerServiceClient(cc)
}

// Request types used by the gateway handlers.
type AccessSecretVersionRequest = secretmanagerpb.AccessSecretVersionRequest
type AddSecretVersionRequest = secretmanagerpb.AddSecretVersionRequest
type CreateSecretRequest = secretmanagerpb.CreateSecretRequest
type DeleteSecretRequest = secretmanagerpb.DeleteSecretRequest
type DestroySecretVersionRequest = secretmanagerpb.DestroySecretVersionRequest
type DisableSecretVersionRequest = secretmanagerpb.DisableSecretVersionRequest
type EnableSecretVersionRequest = secretmanagerpb.EnableSecretVersionRequest
type GetSecretRequest = secretmanagerpb.GetSecretRequest
type GetSecretVersionRequest = secretmanagerpb.GetSecretVersionRequest
type ListSecretsRequest = secretmanagerpb.ListSecretsRequest
type ListSecretVersionsRequest = secretmanagerpb.ListSecretVersionsRequest
type UpdateSecretRequest = secretmanagerpb.UpdateSecretRequest
