// Package gcpmock provides an in-memory mock implementation of the GCP Secret Manager API.
//
// This package is designed to be extraction-ready as a standalone project.
// It has zero dependencies on vaultmux and only uses official GCP protobuf types.
package server

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Storage is the in-memory storage for secrets and versions.
// All operations are thread-safe using sync.RWMutex.
type Storage struct {
	mu      sync.RWMutex
	secrets map[string]*StoredSecret // key: "projects/{project}/secrets/{secret-id}"
}

// StoredSecret represents a secret with all its versions in memory.
type StoredSecret struct {
	// Secret metadata (from secretmanagerpb.Secret)
	Name        string // Full resource name: projects/{project}/secrets/{secret-id}
	CreateTime  *timestamppb.Timestamp
	Labels      map[string]string
	Annotations map[string]string
	Replication *secretmanagerpb.Replication

	// Version management
	Versions    map[string]*StoredVersion // key: "1", "2", "3", etc. (not "latest")
	NextVersion int64                     // Auto-increment version number (1, 2, 3...)
}

// StoredVersion represents a single secret version.
type StoredVersion struct {
	// Version metadata
	Name       string // Full resource name with version
	CreateTime *timestamppb.Timestamp
	State      secretmanagerpb.SecretVersion_State // ENABLED, DISABLED, DESTROYED

	// Actual secret data
	Payload []byte // The secret content
}

// NewStorage creates a new empty storage instance.
func NewStorage() *Storage {
	return &Storage{
		secrets: make(map[string]*StoredSecret),
	}
}

// CreateSecret creates a new secret (metadata only, no versions yet).
// Returns AlreadyExists if secret already exists.
func (s *Storage) CreateSecret(ctx context.Context, parent, secretID string, secret *secretmanagerpb.Secret) (*secretmanagerpb.Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Build full resource name
	secretName := fmt.Sprintf("%s/secrets/%s", parent, secretID)

	// Check if already exists
	if _, exists := s.secrets[secretName]; exists {
		return nil, status.Errorf(codes.AlreadyExists, "Secret [%s] already exists", secretName)
	}

	// Create stored secret
	now := timestamppb.Now()
	stored := &StoredSecret{
		Name:        secretName,
		CreateTime:  now,
		Labels:      secret.GetLabels(),
		Annotations: secret.GetAnnotations(),
		Replication: secret.GetReplication(),
		Versions:    make(map[string]*StoredVersion),
		NextVersion: 1,
	}

	// Default to automatic replication if not specified
	if stored.Replication == nil {
		stored.Replication = &secretmanagerpb.Replication{
			Replication: &secretmanagerpb.Replication_Automatic_{
				Automatic: &secretmanagerpb.Replication_Automatic{},
			},
		}
	}

	s.secrets[secretName] = stored

	// Return secret metadata
	return &secretmanagerpb.Secret{
		Name:        secretName,
		CreateTime:  now,
		Labels:      stored.Labels,
		Annotations: stored.Annotations,
		Replication: stored.Replication,
	}, nil
}

// GetSecret retrieves secret metadata (not version data).
// Returns NotFound if secret doesn't exist.
func (s *Storage) GetSecret(ctx context.Context, secretName string) (*secretmanagerpb.Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stored, exists := s.secrets[secretName]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Secret [%s] not found", secretName)
	}

	return &secretmanagerpb.Secret{
		Name:        stored.Name,
		CreateTime:  stored.CreateTime,
		Labels:      stored.Labels,
		Annotations: stored.Annotations,
		Replication: stored.Replication,
	}, nil
}

// ListSecrets returns all secrets under the parent project.
// Supports pagination via pageSize and pageToken.
func (s *Storage) ListSecrets(ctx context.Context, parent string, pageSize int32, pageToken string) ([]*secretmanagerpb.Secret, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect all secrets matching parent
	var allSecrets []*secretmanagerpb.Secret
	prefix := parent + "/secrets/"

	for name, stored := range s.secrets {
		if strings.HasPrefix(name, prefix) {
			allSecrets = append(allSecrets, &secretmanagerpb.Secret{
				Name:        stored.Name,
				CreateTime:  stored.CreateTime,
				Labels:      stored.Labels,
				Annotations: stored.Annotations,
				Replication: stored.Replication,
			})
		}
	}

	// Simple pagination: start from token index
	startIdx := 0
	if pageToken != "" {
		// Parse token as simple integer index
		_, _ = fmt.Sscanf(pageToken, "%d", &startIdx)
	}

	// Apply page size limit
	if pageSize <= 0 {
		pageSize = 100 // Default page size
	}

	endIdx := startIdx + int(pageSize)
	if endIdx > len(allSecrets) {
		endIdx = len(allSecrets)
	}

	// Paginate results
	var results []*secretmanagerpb.Secret
	if startIdx < len(allSecrets) {
		results = allSecrets[startIdx:endIdx]
	}

	// Generate next page token if there are more results
	if endIdx < len(allSecrets) {
		return results, fmt.Sprintf("%d", endIdx), nil
	}
	return results, "", nil
}

// DeleteSecret deletes a secret and all its versions.
// Returns NotFound if secret doesn't exist.
func (s *Storage) DeleteSecret(ctx context.Context, secretName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.secrets[secretName]; !exists {
		return status.Errorf(codes.NotFound, "Secret [%s] not found", secretName)
	}

	delete(s.secrets, secretName)
	return nil
}

// AddSecretVersion adds a new version to an existing secret.
// Returns NotFound if secret doesn't exist.
func (s *Storage) AddSecretVersion(ctx context.Context, parent string, payload *secretmanagerpb.SecretPayload) (*secretmanagerpb.SecretVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Parent is the secret name
	stored, exists := s.secrets[parent]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Secret [%s] not found", parent)
	}

	// Generate version number
	versionID := fmt.Sprintf("%d", stored.NextVersion)
	stored.NextVersion++

	// Create version
	now := timestamppb.Now()
	versionName := fmt.Sprintf("%s/versions/%s", parent, versionID)
	version := &StoredVersion{
		Name:       versionName,
		CreateTime: now,
		State:      secretmanagerpb.SecretVersion_ENABLED,
		Payload:    payload.GetData(),
	}

	stored.Versions[versionID] = version

	return &secretmanagerpb.SecretVersion{
		Name:       versionName,
		CreateTime: now,
		State:      secretmanagerpb.SecretVersion_ENABLED,
	}, nil
}

// AccessSecretVersion retrieves the payload data for a specific version.
// Supports version aliases: "latest" resolves to highest ENABLED version.
// Returns NotFound if secret or version doesn't exist.
func (s *Storage) AccessSecretVersion(ctx context.Context, versionName string) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Parse resource name: projects/{project}/secrets/{secret}/versions/{version}
	parts := strings.Split(versionName, "/versions/")
	if len(parts) != 2 {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid version name format: %s", versionName)
	}

	secretName := parts[0]
	versionID := parts[1]

	// Get secret
	stored, exists := s.secrets[secretName]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Secret [%s] not found", secretName)
	}

	// Resolve "latest" alias to highest ENABLED version
	if versionID == "latest" {
		latestID, err := s.resolveLatestVersion(stored)
		if err != nil {
			return nil, err
		}
		versionID = latestID
		versionName = fmt.Sprintf("%s/versions/%s", secretName, versionID)
	}

	// Get version
	version, exists := stored.Versions[versionID]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Version [%s] not found", versionName)
	}

	// Check state
	if version.State != secretmanagerpb.SecretVersion_ENABLED {
		return nil, status.Errorf(codes.FailedPrecondition, "Version [%s] is not enabled (state: %s)", versionName, version.State)
	}

	return &secretmanagerpb.AccessSecretVersionResponse{
		Name: versionName,
		Payload: &secretmanagerpb.SecretPayload{
			Data: version.Payload,
		},
	}, nil
}

// resolveLatestVersion finds the highest version number with ENABLED state.
// Must be called with read lock held.
func (s *Storage) resolveLatestVersion(stored *StoredSecret) (string, error) {
	var latestVersionNum int64

	for versionID, version := range stored.Versions {
		if version.State != secretmanagerpb.SecretVersion_ENABLED {
			continue
		}

		var num int64
		_, _ = fmt.Sscanf(versionID, "%d", &num)
		if num > latestVersionNum {
			latestVersionNum = num
		}
	}

	if latestVersionNum == 0 {
		return "", status.Errorf(codes.NotFound, "No enabled versions found for secret [%s]", stored.Name)
	}

	return fmt.Sprintf("%d", latestVersionNum), nil
}

// GetSecretVersion retrieves version metadata (not payload).
// Returns NotFound if secret or version doesn't exist.
func (s *Storage) GetSecretVersion(ctx context.Context, versionName string) (*secretmanagerpb.SecretVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Parse resource name
	parts := strings.Split(versionName, "/versions/")
	if len(parts) != 2 {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid version name format: %s", versionName)
	}

	secretName := parts[0]
	versionID := parts[1]

	// Get secret
	stored, exists := s.secrets[secretName]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Secret [%s] not found", secretName)
	}

	// Resolve "latest" if needed
	if versionID == "latest" {
		latestID, err := s.resolveLatestVersion(stored)
		if err != nil {
			return nil, err
		}
		versionID = latestID
		versionName = fmt.Sprintf("%s/versions/%s", secretName, versionID)
	}

	// Get version
	version, exists := stored.Versions[versionID]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Version [%s] not found", versionName)
	}

	return &secretmanagerpb.SecretVersion{
		Name:       versionName,
		CreateTime: version.CreateTime,
		State:      version.State,
	}, nil
}

// parseStateFilter parses filter string and returns map of states to include.
// Supports filters like "state:ENABLED", "state:DISABLED", "state:DESTROYED".
// Returns empty map if no state filter specified (include all states).
func parseStateFilter(filter string) map[secretmanagerpb.SecretVersion_State]bool {
	if filter == "" {
		return nil // No filter = include all
	}

	includeStates := make(map[secretmanagerpb.SecretVersion_State]bool)

	// Simple filter parser: supports "state:ENABLED", "state:DISABLED", "state:DESTROYED"
	// GCP supports more complex filters, but this covers common testing use cases
	filter = strings.TrimSpace(filter)

	if strings.HasPrefix(filter, "state:") {
		stateName := strings.TrimPrefix(filter, "state:")
		stateName = strings.TrimSpace(stateName)

		switch strings.ToUpper(stateName) {
		case "ENABLED":
			includeStates[secretmanagerpb.SecretVersion_ENABLED] = true
		case "DISABLED":
			includeStates[secretmanagerpb.SecretVersion_DISABLED] = true
		case "DESTROYED":
			includeStates[secretmanagerpb.SecretVersion_DESTROYED] = true
		}
	}

	return includeStates
}

// ListSecretVersions returns all versions of a secret.
// Supports pagination via pageSize and pageToken.
// Supports filtering by state (e.g., "state:ENABLED", "state:DISABLED").
// Returns NotFound if secret doesn't exist.
func (s *Storage) ListSecretVersions(ctx context.Context, parent string, pageSize int32, pageToken, filter string) ([]*secretmanagerpb.SecretVersion, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Parent is the secret name
	stored, exists := s.secrets[parent]
	if !exists {
		return nil, "", status.Errorf(codes.NotFound, "Secret [%s] not found", parent)
	}

	// Parse filter to determine which states to include
	includeStates := parseStateFilter(filter)

	// Collect versions matching filter
	var allVersions []*secretmanagerpb.SecretVersion
	for versionID, version := range stored.Versions {
		// Apply state filter if specified
		if len(includeStates) > 0 && !includeStates[version.State] {
			continue
		}

		versionName := fmt.Sprintf("%s/versions/%s", parent, versionID)
		allVersions = append(allVersions, &secretmanagerpb.SecretVersion{
			Name:       versionName,
			CreateTime: version.CreateTime,
			State:      version.State,
		})
	}

	// Simple pagination: start from token index
	startIdx := 0
	if pageToken != "" {
		_, _ = fmt.Sscanf(pageToken, "%d", &startIdx)
	}

	// Apply page size limit
	if pageSize <= 0 {
		pageSize = 100 // Default page size
	}

	endIdx := startIdx + int(pageSize)
	if endIdx > len(allVersions) {
		endIdx = len(allVersions)
	}

	// Paginate results
	var results []*secretmanagerpb.SecretVersion
	if startIdx < len(allVersions) {
		results = allVersions[startIdx:endIdx]
	}

	// Generate next page token if there are more results
	if endIdx < len(allVersions) {
		return results, fmt.Sprintf("%d", endIdx), nil
	}
	return results, "", nil
}

// DisableSecretVersion disables a version (prevents access).
// Returns NotFound if secret or version doesn't exist.
// Returns FailedPrecondition if version is already DESTROYED.
func (s *Storage) DisableSecretVersion(ctx context.Context, versionName string) (*secretmanagerpb.SecretVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Parse resource name
	parts := strings.Split(versionName, "/versions/")
	if len(parts) != 2 {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid version name format: %s", versionName)
	}

	secretName := parts[0]
	versionID := parts[1]

	// Get secret
	stored, exists := s.secrets[secretName]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Secret [%s] not found", secretName)
	}

	// Get version
	version, exists := stored.Versions[versionID]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Version [%s] not found", versionName)
	}

	// Cannot disable a destroyed version
	if version.State == secretmanagerpb.SecretVersion_DESTROYED {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot disable version [%s]: version is DESTROYED", versionName)
	}

	// Set state to DISABLED
	version.State = secretmanagerpb.SecretVersion_DISABLED

	return &secretmanagerpb.SecretVersion{
		Name:       versionName,
		CreateTime: version.CreateTime,
		State:      secretmanagerpb.SecretVersion_DISABLED,
	}, nil
}

// EnableSecretVersion enables a previously disabled version.
// Returns NotFound if secret or version doesn't exist.
// Returns FailedPrecondition if version is DESTROYED.
func (s *Storage) EnableSecretVersion(ctx context.Context, versionName string) (*secretmanagerpb.SecretVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Parse resource name
	parts := strings.Split(versionName, "/versions/")
	if len(parts) != 2 {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid version name format: %s", versionName)
	}

	secretName := parts[0]
	versionID := parts[1]

	// Get secret
	stored, exists := s.secrets[secretName]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Secret [%s] not found", secretName)
	}

	// Get version
	version, exists := stored.Versions[versionID]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Version [%s] not found", versionName)
	}

	// Cannot enable a destroyed version
	if version.State == secretmanagerpb.SecretVersion_DESTROYED {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot enable version [%s]: version is DESTROYED", versionName)
	}

	// Set state to ENABLED
	version.State = secretmanagerpb.SecretVersion_ENABLED

	return &secretmanagerpb.SecretVersion{
		Name:       versionName,
		CreateTime: version.CreateTime,
		State:      secretmanagerpb.SecretVersion_ENABLED,
	}, nil
}

// Clear removes all secrets from storage (useful for testing).
func (s *Storage) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets = make(map[string]*StoredSecret)
}

// SecretCount returns the number of secrets in storage (useful for testing).
func (s *Storage) SecretCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.secrets)
}
