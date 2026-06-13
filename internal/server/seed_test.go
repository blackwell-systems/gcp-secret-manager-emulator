package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// writeInitFile writes seed content to a temp file and returns its path.
func writeInitFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "init.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write init file: %v", err)
	}
	return path
}

// TestApplyInitFile checks that a seed file is replayed correctly: single value,
// multiple versions, and secret-level labels/annotations.
func TestApplyInitFile(t *testing.T) {
	ctx := context.Background()
	path := writeInitFile(t, `{
	  "secrets": [
	    { "project": "p1", "id": "db-password", "value": "s3cr3t",
	      "labels": {"env": "dev"}, "annotations": {"team": "core"} },
	    { "project": "p1", "id": "api-key", "versions": ["v1", "v2"] }
	  ]
	}`)

	s := NewStorage()
	if err := s.applyInitFile(ctx, path); err != nil {
		t.Fatalf("applyInitFile() error = %v", err)
	}

	if got := s.SecretCount(); got != 2 {
		t.Fatalf("SecretCount = %d, want 2", got)
	}

	// Single value secret + secret-level labels/annotations.
	sec, err := s.GetSecret(ctx, "projects/p1/secrets/db-password")
	if err != nil {
		t.Fatalf("GetSecret(db-password) error = %v", err)
	}
	if sec.GetLabels()["env"] != "dev" || sec.GetAnnotations()["team"] != "core" {
		t.Errorf("labels/annotations not applied at secret level: %+v / %+v", sec.GetLabels(), sec.GetAnnotations())
	}
	resp, err := s.AccessSecretVersion(ctx, "projects/p1/secrets/db-password/versions/latest")
	if err != nil {
		t.Fatalf("Access(db-password latest) error = %v", err)
	}
	if string(resp.GetPayload().GetData()) != "s3cr3t" {
		t.Errorf("db-password payload = %q, want s3cr3t", resp.GetPayload().GetData())
	}

	// Multiple versions: latest resolves to v2; v1 is reachable by number.
	versions, _, total, err := s.ListSecretVersions(ctx, "projects/p1/secrets/api-key", 0, "", "")
	if err != nil {
		t.Fatalf("ListSecretVersions(api-key) error = %v", err)
	}
	if total != 2 {
		t.Errorf("api-key versions = %d, want 2", total)
	}
	_ = versions
	v2, err := s.AccessSecretVersion(ctx, "projects/p1/secrets/api-key/versions/latest")
	if err != nil {
		t.Fatalf("Access(api-key latest) error = %v", err)
	}
	if string(v2.GetPayload().GetData()) != "v2" {
		t.Errorf("api-key latest = %q, want v2", v2.GetPayload().GetData())
	}
}

func TestApplyInitFile_Validation(t *testing.T) {
	ctx := context.Background()

	cases := map[string]string{
		"value and versions together": `{"secrets":[{"project":"p","id":"s","value":"x","versions":["y"]}]}`,
		"missing project":             `{"secrets":[{"id":"s","value":"x"}]}`,
		"missing id":                  `{"secrets":[{"project":"p","value":"x"}]}`,
		"malformed json":              `{"secrets":[`,
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewStorage()
			if err := s.applyInitFile(ctx, writeInitFile(t, content)); err == nil {
				t.Errorf("applyInitFile() should fail for %s", name)
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		s := NewStorage()
		if err := s.applyInitFile(ctx, filepath.Join(t.TempDir(), "nope.json")); err == nil {
			t.Error("applyInitFile() should fail when the configured file is missing")
		}
	})
}

// TestSeedIfFresh_InMemory: an in-memory store always seeds.
func TestSeedIfFresh_InMemory(t *testing.T) {
	ctx := context.Background()
	path := writeInitFile(t, `{"secrets":[{"project":"p","id":"s","value":"x"}]}`)

	s := NewStorage()
	if err := s.seedIfFresh(ctx, path); err != nil {
		t.Fatalf("seedIfFresh() error = %v", err)
	}
	if got := s.SecretCount(); got != 1 {
		t.Errorf("SecretCount = %d, want 1", got)
	}
}

// TestSeedIfFresh_DisabledWhenNoPath: empty path is a no-op.
func TestSeedIfFresh_DisabledWhenNoPath(t *testing.T) {
	s := NewStorage()
	if err := s.seedIfFresh(context.Background(), ""); err != nil {
		t.Fatalf("seedIfFresh(\"\") error = %v", err)
	}
	if got := s.SecretCount(); got != 0 {
		t.Errorf("SecretCount = %d, want 0", got)
	}
}

// TestSeedIfFresh_PersistFirstBoot: with persistence and no snapshot yet, the
// seed is applied and then persisted; on reload the snapshot wins and the seed
// is NOT re-applied (even after the user mutates state).
func TestSeedIfFresh_PersistFirstBoot(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	snapPath := filepath.Join(dir, "secrets.json")
	initPath := writeInitFile(t, `{"secrets":[{"project":"p","id":"seeded","value":"x"}]}`)

	// First boot: fresh persistent store, no snapshot -> seed applies.
	s1, err := NewStorageWithPersistence(snapPath)
	if err != nil {
		t.Fatalf("NewStorageWithPersistence() error = %v", err)
	}
	if s1.loadedFromDisk {
		t.Fatal("loadedFromDisk should be false on first boot (no snapshot)")
	}
	if err := s1.seedIfFresh(ctx, initPath); err != nil {
		t.Fatalf("seedIfFresh() error = %v", err)
	}
	// Simulate the user deleting the seeded secret at runtime.
	if err := s1.DeleteSecret(ctx, "projects/p/secrets/seeded"); err != nil {
		t.Fatalf("DeleteSecret() error = %v", err)
	}
	s1.Close() // flush final state (empty)

	// Second boot: snapshot exists -> loaded, seed must NOT re-add the secret.
	s2, err := NewStorageWithPersistence(snapPath)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	defer s2.Close()
	if !s2.loadedFromDisk {
		t.Fatal("loadedFromDisk should be true after reloading a snapshot")
	}
	if err := s2.seedIfFresh(ctx, initPath); err != nil {
		t.Fatalf("seedIfFresh() on reload error = %v", err)
	}
	if got := s2.SecretCount(); got != 0 {
		t.Errorf("after reload SecretCount = %d, want 0 (seed must not re-apply over runtime state)", got)
	}
}

// TestSeedIfFresh_EmptySnapshotSuppressesSeed: an existing but empty snapshot
// still counts as runtime state and suppresses seeding.
func TestSeedIfFresh_EmptySnapshotSuppressesSeed(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	snapPath := filepath.Join(dir, "secrets.json")
	if err := os.WriteFile(snapPath, []byte(`{"version":1,"secrets":[]}`), 0o600); err != nil {
		t.Fatalf("write empty snapshot: %v", err)
	}
	initPath := writeInitFile(t, `{"secrets":[{"project":"p","id":"s","value":"x"}]}`)

	s, err := NewStorageWithPersistence(snapPath)
	if err != nil {
		t.Fatalf("NewStorageWithPersistence() error = %v", err)
	}
	defer s.Close()
	if !s.loadedFromDisk {
		t.Fatal("loadedFromDisk should be true for an existing (empty) snapshot")
	}
	if err := s.seedIfFresh(ctx, initPath); err != nil {
		t.Fatalf("seedIfFresh() error = %v", err)
	}
	if got := s.SecretCount(); got != 0 {
		t.Errorf("SecretCount = %d, want 0 (empty snapshot suppresses seed)", got)
	}
}

func TestLoadInitConfig(t *testing.T) {
	t.Setenv(initEnvVar, "")
	if got := loadInitConfig(); got != "" {
		t.Errorf("loadInitConfig() empty = %q, want empty", got)
	}
	t.Setenv(initEnvVar, "/some/init.json")
	if got := loadInitConfig(); got != "/some/init.json" {
		t.Errorf("loadInitConfig() = %q, want /some/init.json", got)
	}
}
