package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// Init/seed file support (opt-in, separate from persistence).
//
// When GCP_MOCK_INIT_FILE points at a JSON file, its secrets are loaded into the
// store at startup — but only when no persisted snapshot was loaded (see
// Storage.loadedFromDisk). This is the classic seed/fixtures pattern (cf.
// Postgres' docker-entrypoint-initdb.d): the init file provides starting data on
// a fresh store, and is ignored once runtime state exists, so user changes are
// never overwritten.
//
// The file is replayed through the normal CreateSecret/AddSecretVersion paths, so
// etags, create times and crc32c are generated exactly as for live requests.

// initEnvVar names the file to seed from. Unset means seeding is disabled.
const initEnvVar = "GCP_MOCK_INIT_FILE"

// loadInitConfig returns the configured init file path, or "" when seeding is
// disabled.
func loadInitConfig() string {
	return os.Getenv(initEnvVar)
}

// initSeed is the on-disk schema for the init file.
type initSeed struct {
	Secrets []initSecret `json:"secrets"`
}

// initSecret describes one secret to create. labels/annotations are attributes
// of the secret (not of individual versions, matching the GCP API). Use value
// for a single version, or versions for several; the two are mutually exclusive.
type initSecret struct {
	Project     string            `json:"project"`
	ID          string            `json:"id"`
	Value       string            `json:"value,omitempty"`
	Versions    []string          `json:"versions,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// payloads returns the ordered list of version payloads for this entry, or an
// error if the entry is invalid.
func (e initSecret) payloads() ([]string, error) {
	if e.Project == "" {
		return nil, fmt.Errorf("missing project")
	}
	if e.ID == "" {
		return nil, fmt.Errorf("missing id")
	}
	if e.Value != "" && len(e.Versions) > 0 {
		return nil, fmt.Errorf("secret %q: set either value or versions, not both", e.ID)
	}
	if e.Value != "" {
		return []string{e.Value}, nil
	}
	return e.Versions, nil
}

// seedIfFresh applies the init file when one is configured and no persisted
// snapshot was loaded. It is a no-op otherwise, so existing runtime state always
// wins over the seed.
func (s *Storage) seedIfFresh(ctx context.Context, initPath string) error {
	if initPath == "" || s.loadedFromDisk {
		return nil
	}
	return s.applyInitFile(ctx, initPath)
}

// applyInitFile reads, validates and replays the init file into the store.
// A missing file, malformed JSON, or a duplicate secret is reported as an error.
func (s *Storage) applyInitFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read init file %q: %w", path, err)
	}

	var seed initSeed
	if err := json.Unmarshal(data, &seed); err != nil {
		return fmt.Errorf("parse init file %q: %w", path, err)
	}

	for _, e := range seed.Secrets {
		payloads, err := e.payloads()
		if err != nil {
			return fmt.Errorf("init file %q: %w", path, err)
		}

		parent := "projects/" + e.Project
		if _, err := s.CreateSecret(ctx, parent, e.ID, &secretmanagerpb.Secret{
			Labels:      e.Labels,
			Annotations: e.Annotations,
		}); err != nil {
			return fmt.Errorf("init file %q: create secret %s/%s: %w", path, e.Project, e.ID, err)
		}

		secretName := fmt.Sprintf("%s/secrets/%s", parent, e.ID)
		for _, v := range payloads {
			if _, err := s.AddSecretVersion(ctx, secretName, &secretmanagerpb.SecretPayload{
				Data: []byte(v),
			}); err != nil {
				return fmt.Errorf("init file %q: add version to %s/%s: %w", path, e.Project, e.ID, err)
			}
		}
	}

	return nil
}
