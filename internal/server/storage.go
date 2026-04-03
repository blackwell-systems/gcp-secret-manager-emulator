// Package gcpmock provides an in-memory mock implementation of the GCP Secret Manager API.
//
// This package is designed to be extraction-ready as a standalone project.
// It has zero dependencies on vaultmux and only uses official GCP protobuf types.
package server

import (
	"context"
	"encoding/base64"
	"fmt"
	"sort"
	"strconv"
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
	Name           string // Full resource name: projects/{project}/secrets/{secret-id}
	CreateTime     *timestamppb.Timestamp
	Labels         map[string]string
	Annotations    map[string]string
	Replication    *secretmanagerpb.Replication
	Etag           string           // generated on create, regenerated on update
	VersionAliases map[string]int64 // user-defined alias -> version number

	// Version management
	Versions    map[string]*StoredVersion // key: "1", "2", "3", etc. (not "latest")
	NextVersion int64                     // Auto-increment version number (1, 2, 3...)
}

// StoredVersion represents a single secret version.
type StoredVersion struct {
	// Version metadata
	Name        string // Full resource name with version
	CreateTime  *timestamppb.Timestamp
	State       secretmanagerpb.SecretVersion_State // ENABLED, DISABLED, DESTROYED
	Etag        string                              // generated on AddSecretVersion + state mutation
	DataCrc32C  int64                               // stored checksum (0 if client did not supply)
	DestroyTime *timestamppb.Timestamp              // set when state becomes DESTROYED

	// Actual secret data
	Payload []byte // The secret content
}

// NewStorage creates a new empty storage instance.
func NewStorage() *Storage {
	return &Storage{
		secrets: make(map[string]*StoredSecret),
	}
}

// generateEtag generates a deterministic etag from a name, timestamp, and optional extras.
func generateEtag(name string, t *timestamppb.Timestamp, extra ...string) string {
	s := fmt.Sprintf("%s:%d:%s", name, t.AsTime().UnixNano(), strings.Join(extra, ":"))
	return base64.URLEncoding.EncodeToString([]byte(s))
}

// buildSecretProto constructs a secretmanagerpb.Secret from a StoredSecret.
func buildSecretProto(stored *StoredSecret) *secretmanagerpb.Secret {
	// VersionAliases proto field is map[string]int64, same as our internal representation.
	var aliases map[string]int64
	if len(stored.VersionAliases) > 0 {
		aliases = make(map[string]int64, len(stored.VersionAliases))
		for k, v := range stored.VersionAliases {
			aliases[k] = v
		}
	}
	return &secretmanagerpb.Secret{
		Name:           stored.Name,
		CreateTime:     stored.CreateTime,
		Labels:         stored.Labels,
		Annotations:    stored.Annotations,
		Replication:    stored.Replication,
		Etag:           stored.Etag,
		VersionAliases: aliases,
	}
}

// buildVersionProto constructs a secretmanagerpb.SecretVersion from a StoredVersion.
func buildVersionProto(versionName string, version *StoredVersion) *secretmanagerpb.SecretVersion {
	sv := &secretmanagerpb.SecretVersion{
		Name:       versionName,
		CreateTime: version.CreateTime,
		State:      version.State,
		Etag:       version.Etag,
	}
	if version.State == secretmanagerpb.SecretVersion_DESTROYED && version.DestroyTime != nil {
		sv.DestroyTime = version.DestroyTime
	}
	return sv
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

	stored.Etag = generateEtag(secretName, now)
	stored.VersionAliases = make(map[string]int64)

	s.secrets[secretName] = stored

	return buildSecretProto(stored), nil
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

	return buildSecretProto(stored), nil
}

// parseSecretFilter parses a filter string and returns a predicate function.
// Supports "name:<prefix>" and "labels.<key>=<value>" filter expressions.
// Returns nil if no filter or unknown expression (include all).
func parseSecretFilter(filter string) func(*secretmanagerpb.Secret) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return nil // include all
	}
	if strings.HasPrefix(filter, "name:") {
		prefix := strings.TrimPrefix(filter, "name:")
		return func(s *secretmanagerpb.Secret) bool {
			parts := strings.Split(s.Name, "/secrets/")
			if len(parts) != 2 {
				return false
			}
			return strings.HasPrefix(strings.ToLower(parts[1]), strings.ToLower(prefix))
		}
	}
	if strings.HasPrefix(filter, "labels.") {
		// labels.KEY=VALUE
		rest := strings.TrimPrefix(filter, "labels.")
		kv := strings.SplitN(rest, "=", 2)
		if len(kv) == 2 {
			key, val := kv[0], kv[1]
			return func(s *secretmanagerpb.Secret) bool {
				return s.Labels[key] == val
			}
		}
	}
	return nil // unknown filter expression = include all
}

// ListSecrets returns all secrets under the parent project.
// Supports pagination via pageSize and pageToken.
// Supports filter expressions for name prefix and label matching.
// Returns: secrets page, next page token, total_size (count before pagination), error.
func (s *Storage) ListSecrets(ctx context.Context, parent string, pageSize int32, pageToken string, filter string) ([]*secretmanagerpb.Secret, string, int32, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect all secrets matching parent
	var allSecrets []*secretmanagerpb.Secret
	prefix := parent + "/secrets/"

	for name, stored := range s.secrets {
		if strings.HasPrefix(name, prefix) {
			allSecrets = append(allSecrets, buildSecretProto(stored))
		}
	}

	// Apply filter
	matchFn := parseSecretFilter(filter)
	if matchFn != nil {
		var filtered []*secretmanagerpb.Secret
		for _, sec := range allSecrets {
			if matchFn(sec) {
				filtered = append(filtered, sec)
			}
		}
		allSecrets = filtered
	}

	// Sort descending by CreateTime (newest first)
	sort.Slice(allSecrets, func(i, j int) bool {
		ti := allSecrets[i].CreateTime.AsTime()
		tj := allSecrets[j].CreateTime.AsTime()
		return ti.After(tj)
	})

	// Record total size after filter, before pagination
	totalSize := int32(len(allSecrets))

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
		return results, fmt.Sprintf("%d", endIdx), totalSize, nil
	}
	return results, "", totalSize, nil
}

// UpdateSecret updates mutable fields of a secret (labels, annotations, versionAliases).
// Returns NotFound if secret doesn't exist.
func (s *Storage) UpdateSecret(ctx context.Context, secretName string, labels, annotations map[string]string, versionAliases map[string]int64) (*secretmanagerpb.Secret, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, exists := s.secrets[secretName]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Secret [%s] not found", secretName)
	}

	// Update mutable fields
	if labels != nil {
		stored.Labels = labels
	}
	if annotations != nil {
		stored.Annotations = annotations
	}
	if versionAliases != nil {
		stored.VersionAliases = versionAliases
	}

	// Regenerate etag on any mutation
	stored.Etag = generateEtag(stored.Name, stored.CreateTime, fmt.Sprintf("%d", stored.NextVersion))

	return buildSecretProto(stored), nil
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

	// Store crc32c if client provided it (DataCrc32C is *int64; GetDataCrc32C returns 0 if nil)
	version.DataCrc32C = payload.GetDataCrc32C()
	version.Etag = generateEtag(versionName, now, versionID)

	stored.Versions[versionID] = version

	return buildVersionProto(versionName, version), nil
}

// AccessSecretVersion retrieves the payload data for a specific version.
// Supports version aliases: "latest" resolves to highest ENABLED version.
// Also supports user-defined aliases from VersionAliases.
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

	// Resolve user-defined aliases
	if versionID != "latest" {
		if num, ok := stored.VersionAliases[versionID]; ok {
			versionID = fmt.Sprintf("%d", num)
			versionName = fmt.Sprintf("%s/versions/%s", secretName, versionID)
		}
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

	crc := version.DataCrc32C
	return &secretmanagerpb.AccessSecretVersionResponse{
		Name: versionName,
		Payload: &secretmanagerpb.SecretPayload{
			Data:       version.Payload,
			DataCrc32C: &crc,
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

	// Resolve user-defined aliases
	if versionID != "latest" {
		if num, ok := stored.VersionAliases[versionID]; ok {
			versionID = fmt.Sprintf("%d", num)
			versionName = fmt.Sprintf("%s/versions/%s", secretName, versionID)
		}
	}

	// Get version
	version, exists := stored.Versions[versionID]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Version [%s] not found", versionName)
	}

	return buildVersionProto(versionName, version), nil
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
// Returns: versions page, next page token, total_size (count before pagination), error.
func (s *Storage) ListSecretVersions(ctx context.Context, parent string, pageSize int32, pageToken, filter string) ([]*secretmanagerpb.SecretVersion, string, int32, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Parent is the secret name
	stored, exists := s.secrets[parent]
	if !exists {
		return nil, "", 0, status.Errorf(codes.NotFound, "Secret [%s] not found", parent)
	}

	// Parse filter to determine which states to include
	includeStates := parseStateFilter(filter)

	// Collect version IDs and sort descending (highest version first)
	var versionIDs []string
	for versionID := range stored.Versions {
		versionIDs = append(versionIDs, versionID)
	}
	sort.Slice(versionIDs, func(i, j int) bool {
		a, _ := strconv.Atoi(versionIDs[i])
		b, _ := strconv.Atoi(versionIDs[j])
		return a > b
	})

	// Collect versions matching filter in sorted order
	var allVersions []*secretmanagerpb.SecretVersion
	for _, versionID := range versionIDs {
		version := stored.Versions[versionID]

		// Apply state filter if specified
		if len(includeStates) > 0 && !includeStates[version.State] {
			continue
		}

		versionName := fmt.Sprintf("%s/versions/%s", parent, versionID)
		allVersions = append(allVersions, buildVersionProto(versionName, version))
	}

	// Record total size after filter, before pagination
	totalSize := int32(len(allVersions))

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
		return results, fmt.Sprintf("%d", endIdx), totalSize, nil
	}
	return results, "", totalSize, nil
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

	// Set state to DISABLED and regenerate etag
	version.State = secretmanagerpb.SecretVersion_DISABLED
	version.Etag = generateEtag(version.Name, timestamppb.Now(), version.State.String())

	return buildVersionProto(versionName, version), nil
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

	// Set state to ENABLED and regenerate etag
	version.State = secretmanagerpb.SecretVersion_ENABLED
	version.Etag = generateEtag(version.Name, timestamppb.Now(), version.State.String())

	return buildVersionProto(versionName, version), nil
}

// DestroySecretVersion permanently destroys a version (irreversible).
// Returns NotFound if secret or version doesn't exist.
// Returns FailedPrecondition if version is already DESTROYED (idempotent).
func (s *Storage) DestroySecretVersion(ctx context.Context, versionName string) (*secretmanagerpb.SecretVersion, error) {
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

	// Already destroyed - idempotent operation
	if version.State == secretmanagerpb.SecretVersion_DESTROYED {
		return buildVersionProto(versionName, version), nil
	}

	// Set state to DESTROYED, clear payload, record destroy time, regenerate etag
	version.State = secretmanagerpb.SecretVersion_DESTROYED
	version.Payload = nil // Permanently remove the payload data
	version.DestroyTime = timestamppb.Now()
	version.Etag = generateEtag(version.Name, version.DestroyTime, "DESTROYED")

	return buildVersionProto(versionName, version), nil
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
