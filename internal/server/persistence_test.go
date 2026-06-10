package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
)

// newPersistentStorage is a test helper that creates a persistent storage at a
// temp path and registers cleanup.
func newPersistentStorage(t *testing.T) (*Storage, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secrets.json")
	s, err := NewStorageWithPersistence(path)
	if err != nil {
		t.Fatalf("NewStorageWithPersistence() error = %v", err)
	}
	t.Cleanup(s.Close)
	return s, path
}

// TestPersistence_RoundTrip exercises a full save → reload cycle covering every
// field that must survive: metadata, replication (automatic and user-managed),
// labels/annotations, version aliases, and versions in ENABLED/DISABLED/DESTROYED
// states with crc32c, etag and destroy_time.
func TestPersistence_RoundTrip(t *testing.T) {
	ctx := context.Background()
	s, path := newPersistentStorage(t)

	parent := "projects/test-project"

	// Secret A: automatic replication, labels, two versions; v1 disabled, v2 enabled.
	_, err := s.CreateSecret(ctx, parent, "secret-a", &secretmanagerpb.Secret{
		Labels:      map[string]string{"env": "test"},
		Annotations: map[string]string{"team": "core"},
	})
	if err != nil {
		t.Fatalf("CreateSecret(secret-a) error = %v", err)
	}
	nameA := parent + "/secrets/secret-a"
	if _, err := s.AddSecretVersion(ctx, nameA, &secretmanagerpb.SecretPayload{Data: []byte("payload-a-1")}); err != nil {
		t.Fatalf("AddSecretVersion(a/1) error = %v", err)
	}
	if _, err := s.AddSecretVersion(ctx, nameA, &secretmanagerpb.SecretPayload{Data: []byte("payload-a-2")}); err != nil {
		t.Fatalf("AddSecretVersion(a/2) error = %v", err)
	}
	if _, err := s.DisableSecretVersion(ctx, nameA+"/versions/1"); err != nil {
		t.Fatalf("DisableSecretVersion(a/1) error = %v", err)
	}
	// Define a user alias prod -> 2
	if _, err := s.UpdateSecret(ctx, nameA, nil, nil, map[string]int64{"prod": 2}); err != nil {
		t.Fatalf("UpdateSecret(a aliases) error = %v", err)
	}

	// Secret B: user-managed replication, one destroyed version.
	_, err = s.CreateSecret(ctx, parent, "secret-b", &secretmanagerpb.Secret{
		Replication: &secretmanagerpb.Replication{
			Replication: &secretmanagerpb.Replication_UserManaged_{
				UserManaged: &secretmanagerpb.Replication_UserManaged{
					Replicas: []*secretmanagerpb.Replication_UserManaged_Replica{
						{Location: "us-central1"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateSecret(secret-b) error = %v", err)
	}
	nameB := parent + "/secrets/secret-b"
	if _, err := s.AddSecretVersion(ctx, nameB, &secretmanagerpb.SecretPayload{Data: []byte("payload-b-1")}); err != nil {
		t.Fatalf("AddSecretVersion(b/1) error = %v", err)
	}
	if _, err := s.DestroySecretVersion(ctx, nameB+"/versions/1"); err != nil {
		t.Fatalf("DestroySecretVersion(b/1) error = %v", err)
	}

	// Capture pre-reload expectations.
	wantSecretA, err := s.GetSecret(ctx, nameA)
	if err != nil {
		t.Fatalf("GetSecret(a) error = %v", err)
	}

	// Flush and reload into a fresh storage.
	s.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("snapshot file not written: %v", err)
	}

	reloaded, err := NewStorageWithPersistence(path)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	defer reloaded.Close()

	if got := reloaded.SecretCount(); got != 2 {
		t.Fatalf("after reload SecretCount = %d, want 2", got)
	}

	// Secret A metadata, labels, annotations, etag, aliases preserved.
	gotSecretA, err := reloaded.GetSecret(ctx, nameA)
	if err != nil {
		t.Fatalf("reloaded GetSecret(a) error = %v", err)
	}
	if gotSecretA.GetEtag() != wantSecretA.GetEtag() {
		t.Errorf("secret-a etag = %q, want %q", gotSecretA.GetEtag(), wantSecretA.GetEtag())
	}
	if gotSecretA.GetLabels()["env"] != "test" {
		t.Errorf("secret-a label env = %q, want test", gotSecretA.GetLabels()["env"])
	}
	if gotSecretA.GetAnnotations()["team"] != "core" {
		t.Errorf("secret-a annotation team = %q, want core", gotSecretA.GetAnnotations()["team"])
	}
	if formatTime(gotSecretA.GetCreateTime()) != formatTime(wantSecretA.GetCreateTime()) {
		t.Errorf("secret-a createTime not preserved")
	}
	if gotSecretA.GetReplication().GetAutomatic() == nil {
		t.Errorf("secret-a should have automatic replication after reload")
	}

	// Enabled version v2 still accessible with original payload.
	resp, err := reloaded.AccessSecretVersion(ctx, nameA+"/versions/2")
	if err != nil {
		t.Fatalf("reloaded AccessSecretVersion(a/2) error = %v", err)
	}
	if string(resp.GetPayload().GetData()) != "payload-a-2" {
		t.Errorf("a/2 payload = %q, want payload-a-2", resp.GetPayload().GetData())
	}

	// User alias prod -> 2 resolves after reload.
	respAlias, err := reloaded.AccessSecretVersion(ctx, nameA+"/versions/prod")
	if err != nil {
		t.Fatalf("reloaded AccessSecretVersion(a/prod) error = %v", err)
	}
	if string(respAlias.GetPayload().GetData()) != "payload-a-2" {
		t.Errorf("alias prod payload = %q, want payload-a-2", respAlias.GetPayload().GetData())
	}

	// Disabled version v1 cannot be accessed (state preserved).
	if _, err := reloaded.AccessSecretVersion(ctx, nameA+"/versions/1"); err == nil {
		t.Errorf("a/1 should be DISABLED and not accessible after reload")
	}

	// Secret B user-managed replication preserved.
	gotSecretB, err := reloaded.GetSecret(ctx, nameB)
	if err != nil {
		t.Fatalf("reloaded GetSecret(b) error = %v", err)
	}
	um := gotSecretB.GetReplication().GetUserManaged()
	if um == nil || len(um.GetReplicas()) != 1 || um.GetReplicas()[0].GetLocation() != "us-central1" {
		t.Errorf("secret-b user-managed replication not preserved: %+v", um)
	}

	// Destroyed version: state preserved and payload not retained.
	vB1, err := reloaded.GetSecretVersion(ctx, nameB+"/versions/1")
	if err != nil {
		t.Fatalf("reloaded GetSecretVersion(b/1) error = %v", err)
	}
	if vB1.GetState() != secretmanagerpb.SecretVersion_DESTROYED {
		t.Errorf("b/1 state = %v, want DESTROYED", vB1.GetState())
	}
	if vB1.GetDestroyTime() == nil {
		t.Errorf("b/1 destroy_time should be preserved")
	}
	if payload := reloaded.secrets[nameB].Versions["1"].Payload; payload != nil {
		t.Errorf("destroyed version payload should be nil after reload, got %q", payload)
	}
}

// TestPersistence_MissingFile verifies a fresh start when no snapshot exists.
func TestPersistence_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	s, err := NewStorageWithPersistence(path)
	if err != nil {
		t.Fatalf("NewStorageWithPersistence() with missing file error = %v", err)
	}
	defer s.Close()
	if got := s.SecretCount(); got != 0 {
		t.Errorf("SecretCount = %d, want 0", got)
	}
}

// TestPersistence_CorruptFile verifies we fail loudly rather than silently
// discarding (and then overwriting) an unreadable snapshot.
func TestPersistence_CorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("setup write error = %v", err)
	}
	if _, err := NewStorageWithPersistence(path); err == nil {
		t.Fatal("NewStorageWithPersistence() should return error on corrupt file")
	}
}

// TestPersistence_Disabled verifies the in-memory default has no persistence side
// effects: mutations are safe no-ops for the flusher and Close does nothing.
func TestPersistence_Disabled(t *testing.T) {
	ctx := context.Background()
	s := NewStorage()
	if s.persistPath != "" {
		t.Fatalf("in-memory storage persistPath = %q, want empty", s.persistPath)
	}
	// markDirty must be a no-op (nil dirty channel must never be touched).
	if _, err := s.CreateSecret(ctx, "projects/p", "s", &secretmanagerpb.Secret{}); err != nil {
		t.Fatalf("CreateSecret error = %v", err)
	}
	s.Close() // must not panic or block
}

// TestPersistence_ConfigFromEnv verifies the env toggle and the imposed path.
func TestPersistence_ConfigFromEnv(t *testing.T) {
	if got := loadPersistConfig(); got != "" {
		t.Errorf("loadPersistConfig() unset = %q, want empty", got)
	}
	t.Setenv(persistEnvVar, "true")
	want := filepath.Join(defaultDataDir, dataFileName)
	if got := loadPersistConfig(); got != want {
		t.Errorf("loadPersistConfig() enabled = %q, want %q", got, want)
	}
	t.Setenv(persistEnvVar, "false")
	if got := loadPersistConfig(); got != "" {
		t.Errorf("loadPersistConfig() false = %q, want empty", got)
	}
}

// TestPersistence_ConcurrentMutations stresses the flusher under concurrent
// writers; run with -race. The final reloaded state must contain every secret.
func TestPersistence_ConcurrentMutations(t *testing.T) {
	ctx := context.Background()
	s, path := newPersistentStorage(t)

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("secret-%d", i)
			if _, err := s.CreateSecret(ctx, "projects/p", id, &secretmanagerpb.Secret{}); err != nil {
				t.Errorf("CreateSecret(%s) error = %v", id, err)
				return
			}
			if _, err := s.AddSecretVersion(ctx, "projects/p/secrets/"+id, &secretmanagerpb.SecretPayload{Data: []byte("v")}); err != nil {
				t.Errorf("AddSecretVersion(%s) error = %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	s.Close() // final flush

	reloaded, err := NewStorageWithPersistence(path)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	defer reloaded.Close()
	if got := reloaded.SecretCount(); got != n {
		t.Errorf("after reload SecretCount = %d, want %d", got, n)
	}
}
