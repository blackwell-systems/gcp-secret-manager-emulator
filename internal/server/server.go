// Package server implements a gRPC emulator for Google Cloud Secret Manager API.
//
// This package provides a complete mock implementation of the Secret Manager v1 API
// for local development and testing. It implements the SecretManagerServiceServer interface
// with in-memory storage, eliminating the need for GCP credentials or network access.
//
// The server supports all core operations including secret creation, version management,
// listing with pagination, and deletion. All operations are thread-safe and can handle
// concurrent requests.
//
// For standalone usage, see cmd/server. For embedded testing, import this package
// directly and create a server with NewServer().
package server

import (
	"context"
	"fmt"
	"hash/crc32"

	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	emulatorauth "github.com/blackwell-systems/gcp-emulator-auth"
	"github.com/blackwell-systems/gcp-secret-manager-emulator/internal/authz"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

// Server implements the SecretManagerServiceServer interface.
// It provides a mock implementation of GCP Secret Manager for testing.
//
// The server maintains in-memory storage of secrets and versions with thread-safe
// access. All gRPC methods are implemented to match GCP Secret Manager behavior
// for common operations.
//
// Usage:
//
//	server := server.NewServer()
//	grpcServer := grpc.NewServer()
//	secretmanagerpb.RegisterSecretManagerServiceServer(grpcServer, server)
type Server struct {
	secretmanagerpb.UnimplementedSecretManagerServiceServer
	storage    *Storage
	iamClient  *emulatorauth.Client
	iamMode    emulatorauth.AuthMode
	iamStorage *IAMStorage
}

// NewServer creates a new mock Secret Manager server.
//
// Storage is in-memory by default. When GCP_MOCK_PERSIST is enabled, secrets are
// persisted to and restored from a JSON snapshot file (see persistence.go); a
// malformed existing snapshot is reported as an error rather than discarded.
func NewServer() (*Server, error) {
	storage := NewStorage()
	if path := loadPersistConfig(); path != "" {
		var err error
		storage, err = NewStorageWithPersistence(path)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize persistent storage: %w", err)
		}
	}

	s := &Server{
		storage:    storage,
		iamStorage: NewIAMStorage(),
	}

	config := emulatorauth.LoadFromEnv()
	s.iamMode = config.Mode

	if config.Mode.IsEnabled() {
		client, err := emulatorauth.NewClient(config.Host, config.Mode)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to IAM emulator: %w", err)
		}
		s.iamClient = client
	}

	return s, nil
}

// checkPermission checks if the principal has permission to perform an operation on a resource.
func (s *Server) checkPermission(ctx context.Context, operation string, resource string) error {
	if s.iamClient == nil {
		return nil // IAM disabled, allow all
	}

	principal := emulatorauth.ExtractPrincipalFromContext(ctx)

	permCheck, ok := authz.GetPermission(operation)
	if !ok {
		return nil // Unknown operation, allow
	}

	allowed, err := s.iamClient.CheckPermission(ctx, principal, resource, permCheck.Permission)
	if err != nil {
		return status.Errorf(codes.Internal, "IAM check failed: %v", err)
	}

	if !allowed {
		displayPrincipal := principal
		if displayPrincipal == "" {
			displayPrincipal = "(no principal)"
		}
		return status.Errorf(codes.PermissionDenied,
			"Permission denied: principal '%s' lacks '%s' on resource '%s'",
			displayPrincipal, permCheck.Permission, resource)
	}

	return nil
}

// ListSecrets lists all secrets within a project.
// Implements google.cloud.secretmanager.v1.SecretManagerService.ListSecrets
func (s *Server) ListSecrets(ctx context.Context, req *secretmanagerpb.ListSecretsRequest) (*secretmanagerpb.ListSecretsResponse, error) {
	if req.GetParent() == "" {
		return nil, status.Error(codes.InvalidArgument, "parent is required")
	}

	if err := s.checkPermission(ctx, "ListSecrets", req.GetParent()); err != nil {
		return nil, err
	}

	secrets, token, totalSize, err := s.storage.ListSecrets(ctx, req.GetParent(), req.GetPageSize(), req.GetPageToken(), req.GetFilter())
	if err != nil {
		return nil, err
	}

	return &secretmanagerpb.ListSecretsResponse{
		Secrets:       secrets,
		NextPageToken: token,
		TotalSize:     totalSize,
	}, nil
}

// CreateSecret creates a new secret (metadata only, no versions).
// Implements google.cloud.secretmanager.v1.SecretManagerService.CreateSecret
func (s *Server) CreateSecret(ctx context.Context, req *secretmanagerpb.CreateSecretRequest) (*secretmanagerpb.Secret, error) {
	if req.GetParent() == "" {
		return nil, status.Error(codes.InvalidArgument, "parent is required")
	}
	if req.GetSecretId() == "" {
		return nil, status.Error(codes.InvalidArgument, "secret_id is required")
	}
	if req.GetSecret() == nil {
		return nil, status.Error(codes.InvalidArgument, "secret is required")
	}

	if err := s.checkPermission(ctx, "CreateSecret", authz.NormalizeParentForCreate(req.GetParent())); err != nil {
		return nil, err
	}

	return s.storage.CreateSecret(ctx, req.GetParent(), req.GetSecretId(), req.GetSecret())
}

// GetSecret retrieves secret metadata (not version data).
// Implements google.cloud.secretmanager.v1.SecretManagerService.GetSecret
func (s *Server) GetSecret(ctx context.Context, req *secretmanagerpb.GetSecretRequest) (*secretmanagerpb.Secret, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	if err := s.checkPermission(ctx, "GetSecret", authz.NormalizeSecretResource(req.GetName())); err != nil {
		return nil, err
	}

	return s.storage.GetSecret(ctx, req.GetName())
}

// UpdateSecret updates secret metadata (labels, annotations).
// Implements google.cloud.secretmanager.v1.SecretManagerService.UpdateSecret
func (s *Server) UpdateSecret(ctx context.Context, req *secretmanagerpb.UpdateSecretRequest) (*secretmanagerpb.Secret, error) {
	if req.GetSecret() == nil || req.GetSecret().GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "secret.name is required")
	}

	if err := s.checkPermission(ctx, "UpdateSecret", authz.NormalizeSecretResource(req.GetSecret().GetName())); err != nil {
		return nil, err
	}

	if req.GetUpdateMask() == nil {
		return nil, status.Error(codes.InvalidArgument, "update_mask is required")
	}

	secretName := req.GetSecret().GetName()
	updateMask := req.GetUpdateMask()

	// Parse update mask to determine which fields to update
	var labels, annotations map[string]string
	var versionAliases map[string]int64

	for _, path := range updateMask.GetPaths() {
		switch path {
		case "labels":
			labels = req.GetSecret().GetLabels()
		case "annotations":
			annotations = req.GetSecret().GetAnnotations()
		case "version_aliases":
			protoAliases := req.GetSecret().GetVersionAliases()
			if protoAliases != nil {
				versionAliases = make(map[string]int64, len(protoAliases))
				for k, v := range protoAliases {
					versionAliases[k] = int64(v)
				}
			}
		default:
			// Ignore unsupported fields (following GCP behavior - silently skip)
		}
	}

	if req.GetSecret().GetEtag() != "" {
		existing, err := s.storage.GetSecret(ctx, secretName)
		if err != nil {
			return nil, err
		}
		if existing.GetEtag() != req.GetSecret().GetEtag() {
			return nil, status.Errorf(codes.Aborted,
				"etag mismatch: provided %q does not match current etag",
				req.GetSecret().GetEtag())
		}
	}

	return s.storage.UpdateSecret(ctx, secretName, labels, annotations, versionAliases)
}

// DeleteSecret deletes a secret and all its versions.
// Implements google.cloud.secretmanager.v1.SecretManagerService.DeleteSecret
func (s *Server) DeleteSecret(ctx context.Context, req *secretmanagerpb.DeleteSecretRequest) (*emptypb.Empty, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if err := s.checkPermission(ctx, "DeleteSecret", authz.NormalizeSecretResource(req.GetName())); err != nil {
		return nil, err
	}
	if req.GetEtag() != "" {
		existing, err := s.storage.GetSecret(ctx, req.GetName())
		if err != nil {
			return nil, err
		}
		if existing.GetEtag() != req.GetEtag() {
			return nil, status.Errorf(codes.Aborted,
				"etag mismatch: provided %q does not match current etag",
				req.GetEtag())
		}
	}
	if err := s.storage.DeleteSecret(ctx, req.GetName()); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// AddSecretVersion adds a new version to an existing secret.
// Implements google.cloud.secretmanager.v1.SecretManagerService.AddSecretVersion
func (s *Server) AddSecretVersion(ctx context.Context, req *secretmanagerpb.AddSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
	if req.GetParent() == "" {
		return nil, status.Error(codes.InvalidArgument, "parent is required")
	}
	if req.GetPayload() == nil {
		return nil, status.Error(codes.InvalidArgument, "payload is required")
	}

	if err := s.checkPermission(ctx, "AddSecretVersion", authz.NormalizeSecretResource(req.GetParent())); err != nil {
		return nil, err
	}

	// Validate crc32c integrity if the client provided a checksum
	if req.GetPayload().DataCrc32C != nil {
		clientCrc := req.GetPayload().GetDataCrc32C()
		computed := int64(crc32.Checksum(req.GetPayload().GetData(), crc32cTable))
		if clientCrc != computed {
			return nil, status.Errorf(codes.DataLoss,
				"data_crc32c mismatch: client provided %d, server computed %d",
				clientCrc, computed)
		}
	}

	return s.storage.AddSecretVersion(ctx, req.GetParent(), req.GetPayload())
}

// GetSecretVersion retrieves version metadata (not payload).
// Implements google.cloud.secretmanager.v1.SecretManagerService.GetSecretVersion
func (s *Server) GetSecretVersion(ctx context.Context, req *secretmanagerpb.GetSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	if err := s.checkPermission(ctx, "GetSecretVersion", authz.NormalizeSecretVersionResource(req.GetName())); err != nil {
		return nil, err
	}

	return s.storage.GetSecretVersion(ctx, req.GetName())
}

// AccessSecretVersion retrieves the payload data for a specific version.
// Supports "latest" version alias.
// Implements google.cloud.secretmanager.v1.SecretManagerService.AccessSecretVersion
func (s *Server) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	if err := s.checkPermission(ctx, "AccessSecretVersion", authz.NormalizeSecretVersionResource(req.GetName())); err != nil {
		return nil, err
	}

	return s.storage.AccessSecretVersion(ctx, req.GetName())
}

// ListSecretVersions lists all versions of a secret.
// Supports pagination via page_size and page_token.
// Supports filtering by state via filter parameter (e.g., "state:ENABLED").
// Implements google.cloud.secretmanager.v1.SecretManagerService.ListSecretVersions
func (s *Server) ListSecretVersions(ctx context.Context, req *secretmanagerpb.ListSecretVersionsRequest) (*secretmanagerpb.ListSecretVersionsResponse, error) {
	if req.GetParent() == "" {
		return nil, status.Error(codes.InvalidArgument, "parent is required")
	}

	if err := s.checkPermission(ctx, "ListSecretVersions", authz.NormalizeSecretResource(req.GetParent())); err != nil {
		return nil, err
	}

	versions, token, totalSize, err := s.storage.ListSecretVersions(ctx, req.GetParent(), req.GetPageSize(), req.GetPageToken(), req.GetFilter())
	if err != nil {
		return nil, err
	}

	return &secretmanagerpb.ListSecretVersionsResponse{
		Versions:      versions,
		NextPageToken: token,
		TotalSize:     totalSize,
	}, nil
}

// EnableSecretVersion enables a previously disabled version.
// Implements google.cloud.secretmanager.v1.SecretManagerService.EnableSecretVersion
func (s *Server) EnableSecretVersion(ctx context.Context, req *secretmanagerpb.EnableSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if err := s.checkPermission(ctx, "EnableSecretVersion", authz.NormalizeSecretVersionResource(req.GetName())); err != nil {
		return nil, err
	}
	if req.GetEtag() != "" {
		existing, err := s.storage.GetSecretVersion(ctx, req.GetName())
		if err != nil {
			return nil, err
		}
		if existing.GetEtag() != req.GetEtag() {
			return nil, status.Errorf(codes.Aborted,
				"etag mismatch: provided %q does not match current etag",
				req.GetEtag())
		}
	}
	return s.storage.EnableSecretVersion(ctx, req.GetName())
}

// DisableSecretVersion disables a version (prevents access).
// AccessSecretVersion will fail for disabled versions.
// Implements google.cloud.secretmanager.v1.SecretManagerService.DisableSecretVersion
func (s *Server) DisableSecretVersion(ctx context.Context, req *secretmanagerpb.DisableSecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if err := s.checkPermission(ctx, "DisableSecretVersion", authz.NormalizeSecretVersionResource(req.GetName())); err != nil {
		return nil, err
	}
	if req.GetEtag() != "" {
		existing, err := s.storage.GetSecretVersion(ctx, req.GetName())
		if err != nil {
			return nil, err
		}
		if existing.GetEtag() != req.GetEtag() {
			return nil, status.Errorf(codes.Aborted,
				"etag mismatch: provided %q does not match current etag",
				req.GetEtag())
		}
	}
	return s.storage.DisableSecretVersion(ctx, req.GetName())
}

// DestroySecretVersion permanently destroys a version.
// Implements google.cloud.secretmanager.v1.SecretManagerService.DestroySecretVersion
func (s *Server) DestroySecretVersion(ctx context.Context, req *secretmanagerpb.DestroySecretVersionRequest) (*secretmanagerpb.SecretVersion, error) {
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if err := s.checkPermission(ctx, "DestroySecretVersion", authz.NormalizeSecretVersionResource(req.GetName())); err != nil {
		return nil, err
	}
	if req.GetEtag() != "" {
		existing, err := s.storage.GetSecretVersion(ctx, req.GetName())
		if err != nil {
			return nil, err
		}
		if existing.GetEtag() != req.GetEtag() {
			return nil, status.Errorf(codes.Aborted,
				"etag mismatch: provided %q does not match current etag",
				req.GetEtag())
		}
	}
	return s.storage.DestroySecretVersion(ctx, req.GetName())
}

// GetIamPolicy returns the IAM policy for a secret resource.
// Implements google.iam.v1.IAMPolicy.GetIamPolicy
func (s *Server) GetIamPolicy(ctx context.Context, req *iampb.GetIamPolicyRequest) (*iampb.Policy, error) {
	if req.GetResource() == "" {
		return nil, status.Error(codes.InvalidArgument, "resource is required")
	}
	if err := s.checkPermission(ctx, "GetIamPolicy", authz.NormalizeSecretResource(req.GetResource())); err != nil {
		return nil, err
	}
	return s.iamStorage.GetPolicy(req.GetResource()), nil
}

// SetIamPolicy sets the IAM policy for a secret resource.
// Implements google.iam.v1.IAMPolicy.SetIamPolicy
func (s *Server) SetIamPolicy(ctx context.Context, req *iampb.SetIamPolicyRequest) (*iampb.Policy, error) {
	if req.GetResource() == "" {
		return nil, status.Error(codes.InvalidArgument, "resource is required")
	}
	if err := s.checkPermission(ctx, "SetIamPolicy", authz.NormalizeSecretResource(req.GetResource())); err != nil {
		return nil, err
	}
	return s.iamStorage.SetPolicy(req.GetResource(), req.GetPolicy()), nil
}

// TestIamPermissions tests whether the caller has the specified permissions on a resource.
// Implements google.iam.v1.IAMPolicy.TestIamPermissions
func (s *Server) TestIamPermissions(ctx context.Context, req *iampb.TestIamPermissionsRequest) (*iampb.TestIamPermissionsResponse, error) {
	if req.GetResource() == "" {
		return nil, status.Error(codes.InvalidArgument, "resource is required")
	}
	if s.iamClient == nil {
		// IAM disabled: grant all requested permissions
		return &iampb.TestIamPermissionsResponse{
			Permissions: req.GetPermissions(),
		}, nil
	}
	// IAM enabled: check each permission individually
	principal := emulatorauth.ExtractPrincipalFromContext(ctx)
	var granted []string
	for _, perm := range req.GetPermissions() {
		allowed, err := s.iamClient.CheckPermission(ctx, principal, req.GetResource(), perm)
		if err == nil && allowed {
			granted = append(granted, perm)
		}
	}
	return &iampb.TestIamPermissionsResponse{Permissions: granted}, nil
}

// Storage returns the underlying storage (useful for testing).
func (s *Server) Storage() *Storage {
	return s.storage
}

// Close releases server resources. When persistence is enabled it performs a
// final snapshot flush and stops the background flusher. It is safe to call
// multiple times and is a no-op when persistence is disabled. Callers should
// invoke it during graceful shutdown.
func (s *Server) Close() {
	s.storage.Close()
}
