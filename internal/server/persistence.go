package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Persistence is optional and opt-in. When disabled (the default), the emulator
// behaves exactly as before: everything is kept in memory and lost on restart.
//
// When enabled via GCP_MOCK_PERSIST, the full set of secrets is snapshotted to a
// single JSON file. State is loaded on startup and written back atomically after
// every mutation by a single background goroutine, so request latency is never
// blocked by disk I/O and concurrent writes cannot tear the file.
//
// Only secrets are persisted. IAM policies (IAMStorage) are intentionally out of
// scope, consistent with the project roadmap (per-resource policy storage is
// handled by the IAM emulator control plane, not this emulator).

const (
	// persistEnvVar toggles persistence on. It is parsed with strconv.ParseBool,
	// so "1", "t", "T", "TRUE", "true", "True" enable it and "0", "f", "false", …
	// (or unset) keep the in-memory default.
	persistEnvVar = "GCP_MOCK_PERSIST"

	// defaultDataDir is the fixed, documented directory mounted as a volume when
	// persistence is enabled. The path is intentionally not user-configurable so
	// there is a single, well-known mount point to document.
	defaultDataDir = "/data"

	// dataFileName is the snapshot file written inside defaultDataDir.
	dataFileName = "secrets.json"

	// snapshotSchemaVersion is the on-disk format version, bumped if the schema
	// ever changes in a backward-incompatible way.
	snapshotSchemaVersion = 1
)

// loadPersistConfig reads the persistence configuration from the environment and
// returns the snapshot file path. An empty path means persistence is disabled.
func loadPersistConfig() string {
	v := os.Getenv(persistEnvVar)
	if v == "" {
		return ""
	}
	enabled, err := strconv.ParseBool(v)
	if err != nil {
		log.Printf("persistence: ignoring invalid %s=%q (want a boolean)", persistEnvVar, v)
		return ""
	}
	if !enabled {
		return ""
	}
	return filepath.Join(defaultDataDir, dataFileName)
}

// --- On-disk schema (decoupled from the in-memory structs) -------------------

type snapshot struct {
	Version int              `json:"version"`
	Secrets []secretSnapshot `json:"secrets"`
}

type secretSnapshot struct {
	Name           string            `json:"name"`
	CreateTime     string            `json:"createTime"` // RFC3339Nano
	Labels         map[string]string `json:"labels,omitempty"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	Replication    json.RawMessage   `json:"replication,omitempty"` // protojson of secretmanagerpb.Replication
	Etag           string            `json:"etag"`
	VersionAliases map[string]int64  `json:"versionAliases,omitempty"`
	NextVersion    int64             `json:"nextVersion"`
	Versions       []versionSnapshot `json:"versions"`
}

type versionSnapshot struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	CreateTime  string `json:"createTime"` // RFC3339Nano
	State       string `json:"state"`      // enum name, e.g. "ENABLED"
	Etag        string `json:"etag"`
	DataCrc32C  int64  `json:"dataCrc32c"`
	DestroyTime string `json:"destroyTime,omitempty"` // RFC3339Nano, set when DESTROYED
	Payload     []byte `json:"payload,omitempty"`     // base64; nil for DESTROYED versions
}

func formatTime(t *timestamppb.Timestamp) string {
	if t == nil {
		return ""
	}
	return t.AsTime().UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (*timestamppb.Timestamp, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return nil, err
	}
	return timestamppb.New(t), nil
}

// --- Serialization -----------------------------------------------------------

// buildSnapshot captures a consistent serializable view of all secrets.
// It must be called with at least a read lock held.
func (s *Storage) buildSnapshot() (*snapshot, error) {
	snap := &snapshot{
		Version: snapshotSchemaVersion,
		Secrets: make([]secretSnapshot, 0, len(s.secrets)),
	}

	for _, stored := range s.secrets {
		sec := secretSnapshot{
			Name:           stored.Name,
			CreateTime:     formatTime(stored.CreateTime),
			Labels:         stored.Labels,
			Annotations:    stored.Annotations,
			Etag:           stored.Etag,
			VersionAliases: stored.VersionAliases,
			NextVersion:    stored.NextVersion,
			Versions:       make([]versionSnapshot, 0, len(stored.Versions)),
		}

		if stored.Replication != nil {
			raw, err := protojson.Marshal(stored.Replication)
			if err != nil {
				return nil, fmt.Errorf("marshal replication for %q: %w", stored.Name, err)
			}
			sec.Replication = raw
		}

		for id, v := range stored.Versions {
			sec.Versions = append(sec.Versions, versionSnapshot{
				ID:          id,
				Name:        v.Name,
				CreateTime:  formatTime(v.CreateTime),
				State:       v.State.String(),
				Etag:        v.Etag,
				DataCrc32C:  v.DataCrc32C,
				DestroyTime: formatTime(v.DestroyTime),
				Payload:     v.Payload,
			})
		}

		snap.Secrets = append(snap.Secrets, sec)
	}

	return snap, nil
}

// applySnapshot replaces the in-memory state with the contents of snap.
// It is called during construction, before the Storage is shared, so it does not lock.
func (s *Storage) applySnapshot(snap *snapshot) error {
	if snap.Version != snapshotSchemaVersion {
		return fmt.Errorf("unsupported snapshot schema version %d (expected %d)", snap.Version, snapshotSchemaVersion)
	}

	secrets := make(map[string]*StoredSecret, len(snap.Secrets))
	for _, sec := range snap.Secrets {
		createTime, err := parseTime(sec.CreateTime)
		if err != nil {
			return fmt.Errorf("parse createTime for %q: %w", sec.Name, err)
		}

		stored := &StoredSecret{
			Name:           sec.Name,
			CreateTime:     createTime,
			Labels:         sec.Labels,
			Annotations:    sec.Annotations,
			Etag:           sec.Etag,
			VersionAliases: sec.VersionAliases,
			NextVersion:    sec.NextVersion,
			Versions:       make(map[string]*StoredVersion, len(sec.Versions)),
		}
		if stored.VersionAliases == nil {
			stored.VersionAliases = make(map[string]int64)
		}

		if len(sec.Replication) > 0 {
			repl := &secretmanagerpb.Replication{}
			if err := protojson.Unmarshal(sec.Replication, repl); err != nil {
				return fmt.Errorf("unmarshal replication for %q: %w", sec.Name, err)
			}
			stored.Replication = repl
		}

		for _, vs := range sec.Versions {
			vCreate, err := parseTime(vs.CreateTime)
			if err != nil {
				return fmt.Errorf("parse createTime for version %q: %w", vs.Name, err)
			}
			vDestroy, err := parseTime(vs.DestroyTime)
			if err != nil {
				return fmt.Errorf("parse destroyTime for version %q: %w", vs.Name, err)
			}
			state, ok := secretmanagerpb.SecretVersion_State_value[vs.State]
			if !ok {
				return fmt.Errorf("unknown version state %q for %q", vs.State, vs.Name)
			}
			stored.Versions[vs.ID] = &StoredVersion{
				Name:        vs.Name,
				CreateTime:  vCreate,
				State:       secretmanagerpb.SecretVersion_State(state),
				Etag:        vs.Etag,
				DataCrc32C:  vs.DataCrc32C,
				DestroyTime: vDestroy,
				Payload:     vs.Payload,
			}
		}

		secrets[sec.Name] = stored
	}

	s.secrets = secrets
	return nil
}

// --- File I/O ----------------------------------------------------------------

// loadFromFile reads a snapshot file and applies it. A missing file is not an
// error (fresh start); a malformed file IS an error so we never silently discard
// existing data.
func (s *Storage) loadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // fresh start
		}
		return fmt.Errorf("read snapshot %q: %w", path, err)
	}

	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return fmt.Errorf("parse snapshot %q: %w", path, err)
	}
	return s.applySnapshot(&snap)
}

// flush writes the current state to disk atomically (temp file + rename).
// os.Rename replaces the destination atomically on POSIX and on Windows
// (modern Go uses MoveFileEx with MOVEFILE_REPLACE_EXISTING).
func (s *Storage) flush() error {
	s.mu.RLock()
	snap, err := s.buildSnapshot()
	s.mu.RUnlock()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.persistPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create data dir %q: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".secrets-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op if rename succeeded

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, s.persistPath); err != nil {
		return fmt.Errorf("rename snapshot into place: %w", err)
	}
	return nil
}

// --- Async flusher lifecycle -------------------------------------------------

// logPersistError reports a background persistence failure. Errors are logged
// rather than fatal: the emulator keeps serving from memory even if a snapshot
// write fails.
func logPersistError(err error) {
	log.Printf("persistence: failed to write snapshot: %v", err)
}

// markDirty signals that the on-disk snapshot is stale. It is a non-blocking
// no-op when persistence is disabled, so the hot path cost is a single
// comparison. Callers may hold the storage lock; the send never blocks.
func (s *Storage) markDirty() {
	if s.persistPath == "" {
		return
	}
	select {
	case s.dirty <- struct{}{}:
	default: // a flush is already pending; it will pick up the latest state
	}
}

// runFlusher is the sole disk writer. It coalesces bursts (the dirty channel has
// capacity 1) and performs a final flush on shutdown.
func (s *Storage) runFlusher() {
	for {
		select {
		case <-s.quit:
			if err := s.flush(); err != nil {
				logPersistError(err)
			}
			close(s.flushDone)
			return
		case <-s.dirty:
			if err := s.flush(); err != nil {
				logPersistError(err)
			}
		}
	}
}

// Close stops the background flusher after performing a final flush. It is safe
// to call multiple times and is a no-op when persistence is disabled.
func (s *Storage) Close() {
	if s.persistPath == "" {
		return
	}
	s.closeOnce.Do(func() {
		close(s.quit)
		<-s.flushDone
	})
}

// NewStorageWithPersistence creates a storage backed by a JSON snapshot file at
// path. Existing state is loaded on startup; subsequent mutations are flushed
// asynchronously. A malformed file is reported as an error.
func NewStorageWithPersistence(path string) (*Storage, error) {
	s := &Storage{
		secrets:     make(map[string]*StoredSecret),
		persistPath: path,
		dirty:       make(chan struct{}, 1),
		quit:        make(chan struct{}),
		flushDone:   make(chan struct{}),
	}
	if err := s.loadFromFile(path); err != nil {
		return nil, err
	}
	go s.runFlusher()
	return s, nil
}
