package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/config"
	"github.com/Homiakus/WebGate/server/pkg/domain"
)

const (
	controlSchemaVersion = 1
	controlRecordVersion = 1
)

type SQLiteControlStore struct {
	registry *SQLiteRegistryStore
	db       *sql.DB
	path     string
}

func OpenSQLiteControlStore(registry *SQLiteRegistryStore) (*SQLiteControlStore, error) {
	if registry == nil || registry.db == nil {
		return nil, errors.New("registry store is required")
	}
	store := &SQLiteControlStore{registry: registry, db: registry.db, path: registry.path}
	if err := store.initialize(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *SQLiteControlStore) initialize() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin control-state migration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS webgate_control_schema (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    version INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS control_config (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    schema_version INTEGER NOT NULL,
    payload BLOB NOT NULL,
    checksum TEXT NOT NULL,
    updated_at_unix INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS audit_events (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL UNIQUE,
    schema_version INTEGER NOT NULL,
    payload BLOB NOT NULL,
    checksum TEXT NOT NULL,
    created_at_unix INTEGER NOT NULL
);
CREATE TRIGGER IF NOT EXISTS audit_events_no_update
BEFORE UPDATE ON audit_events
BEGIN
    SELECT RAISE(ABORT, 'audit events are append-only');
END;
CREATE TRIGGER IF NOT EXISTS audit_events_no_delete
BEFORE DELETE ON audit_events
BEGIN
    SELECT RAISE(ABORT, 'audit events are append-only');
END;`); err != nil {
		return fmt.Errorf("create control-state schema: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO webgate_control_schema(singleton, version) VALUES(1, ?) ON CONFLICT(singleton) DO NOTHING`,
		controlSchemaVersion,
	); err != nil {
		return fmt.Errorf("initialize control-state schema version: %w", err)
	}
	var version int
	if err := tx.QueryRowContext(ctx, `SELECT version FROM webgate_control_schema WHERE singleton = 1`).Scan(&version); err != nil {
		return fmt.Errorf("read control-state schema version: %w", err)
	}
	if version != controlSchemaVersion {
		return fmt.Errorf("%w: control database=%d supported=%d", ErrUnsupportedSchema, version, controlSchemaVersion)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit control-state migration: %w", err)
	}
	return nil
}

func (s *SQLiteControlStore) SaveControlConfig(cfg *config.DurableServerConfig) error {
	if cfg == nil {
		return errors.New("durable control config is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin durable control config write: %w", err)
	}
	defer tx.Rollback()
	if err := saveControlConfigTx(ctx, tx, cfg); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit durable control config: %w", err)
	}
	return nil
}

func (s *SQLiteControlStore) SaveControlConfigWithAudit(cfg *config.DurableServerConfig, event domain.AuditEvent) error {
	if cfg == nil {
		return errors.New("durable control config is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin durable control config transaction: %w", err)
	}
	defer tx.Rollback()
	if err := saveControlConfigTx(ctx, tx, cfg); err != nil {
		return err
	}
	if err := appendAuditTx(ctx, tx, event); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit durable control config transaction: %w", err)
	}
	return nil
}

func saveControlConfigTx(ctx context.Context, tx *sql.Tx, cfg *config.DurableServerConfig) error {
	payload, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal durable control config: %w", err)
	}
	checksum := checksumFor("control_config", "singleton", payload)
	if _, err := tx.ExecContext(ctx, `
INSERT INTO control_config(singleton, schema_version, payload, checksum, updated_at_unix)
VALUES(1, ?, ?, ?, ?)
ON CONFLICT(singleton) DO UPDATE SET
    schema_version = excluded.schema_version,
    payload = excluded.payload,
    checksum = excluded.checksum,
    updated_at_unix = excluded.updated_at_unix`,
		controlRecordVersion, payload, checksum, time.Now().UTC().UnixNano(),
	); err != nil {
		return fmt.Errorf("write durable control config: %w", err)
	}
	return nil
}

func (s *SQLiteControlStore) LoadControlConfig() (*config.DurableServerConfig, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var schemaVersion int
	var payload []byte
	var checksum string
	err := s.db.QueryRowContext(ctx,
		`SELECT schema_version, payload, checksum FROM control_config WHERE singleton = 1`,
	).Scan(&schemaVersion, &payload, &checksum)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load durable control config: %w", err)
	}
	if schemaVersion != controlRecordVersion {
		return nil, fmt.Errorf("%w: control config record=%d supported=%d", ErrUnsupportedSchema, schemaVersion, controlRecordVersion)
	}
	if expected := checksumFor("control_config", "singleton", payload); !strings.EqualFold(checksum, expected) {
		return nil, fmt.Errorf("%w: control config checksum mismatch", ErrCorruptState)
	}
	var cfg config.DurableServerConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return nil, fmt.Errorf("%w: decode control config: %v", ErrCorruptState, err)
	}
	cfg.TelegramAdminChatIDs = append([]int64(nil), cfg.TelegramAdminChatIDs...)
	cfg.RelayNodes = append([]config.RelayNodeEntry(nil), cfg.RelayNodes...)
	return &cfg, nil
}

func (s *SQLiteControlStore) AppendAuditBatch(events []domain.AuditEvent) error {
	if len(events) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin durable audit append: %w", err)
	}
	defer tx.Rollback()
	for _, event := range events {
		if err := appendAuditTx(ctx, tx, event); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit durable audit append: %w", err)
	}
	return nil
}

func appendAuditTx(ctx context.Context, tx *sql.Tx, event domain.AuditEvent) error {
	if strings.TrimSpace(event.ID) == "" || strings.TrimSpace(string(event.Action)) == "" || event.Timestamp.IsZero() {
		return errors.New("audit event ID, action and timestamp are required")
	}
	event.Timestamp = event.Timestamp.UTC()
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event %q: %w", event.ID, err)
	}
	checksum := checksumFor("audit", event.ID, payload)
	result, err := tx.ExecContext(ctx, `
INSERT INTO audit_events(event_id, schema_version, payload, checksum, created_at_unix)
VALUES(?, ?, ?, ?, ?)
ON CONFLICT(event_id) DO NOTHING`,
		event.ID, controlRecordVersion, payload, checksum, event.Timestamp.UnixNano(),
	)
	if err != nil {
		return fmt.Errorf("append durable audit event %q: %w", event.ID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect durable audit append %q: %w", event.ID, err)
	}
	if rows == 1 {
		return nil
	}
	var existingChecksum string
	if err := tx.QueryRowContext(ctx, `SELECT checksum FROM audit_events WHERE event_id = ?`, event.ID).Scan(&existingChecksum); err != nil {
		return fmt.Errorf("read existing audit event %q: %w", event.ID, err)
	}
	if !strings.EqualFold(existingChecksum, checksum) {
		return fmt.Errorf("%w: conflicting audit event ID %q", ErrCorruptState, event.ID)
	}
	return nil
}

func (s *SQLiteControlStore) LoadAudit() ([]domain.AuditEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `
SELECT event_id, schema_version, payload, checksum
FROM audit_events
ORDER BY seq`)
	if err != nil {
		return nil, fmt.Errorf("load durable audit log: %w", err)
	}
	defer rows.Close()

	var events []domain.AuditEvent
	for rows.Next() {
		var eventID, checksum string
		var schemaVersion int
		var payload []byte
		if err := rows.Scan(&eventID, &schemaVersion, &payload, &checksum); err != nil {
			return nil, fmt.Errorf("scan durable audit event: %w", err)
		}
		if schemaVersion != controlRecordVersion {
			return nil, fmt.Errorf("%w: audit %q record=%d supported=%d", ErrUnsupportedSchema, eventID, schemaVersion, controlRecordVersion)
		}
		if expected := checksumFor("audit", eventID, payload); !strings.EqualFold(checksum, expected) {
			return nil, fmt.Errorf("%w: audit checksum mismatch for %q", ErrCorruptState, eventID)
		}
		var event domain.AuditEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, fmt.Errorf("%w: decode audit %q: %v", ErrCorruptState, eventID, err)
		}
		if event.ID != eventID {
			return nil, fmt.Errorf("%w: audit key mismatch %q != %q", ErrCorruptState, event.ID, eventID)
		}
		event.Timestamp = event.Timestamp.UTC()
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate durable audit log: %w", err)
	}
	return events, nil
}

func (s *SQLiteControlStore) Validate() error {
	if _, err := s.registry.LoadServices(); err != nil {
		return err
	}
	if _, err := s.registry.LoadDevices(); err != nil {
		return err
	}
	if _, err := s.registry.LoadReleases(); err != nil {
		return err
	}
	if _, err := s.LoadControlConfig(); err != nil {
		return err
	}
	if _, err := s.LoadAudit(); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteControlStore) BackupTo(destination string) error {
	destination = strings.TrimSpace(destination)
	if destination == "" {
		return errors.New("backup destination is required")
	}
	sourceAbs, err := filepath.Abs(s.path)
	if err != nil {
		return err
	}
	destinationAbs, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	if sourceAbs == destinationAbs {
		return errors.New("backup destination must differ from live state database")
	}
	if _, err := os.Stat(destinationAbs); err == nil {
		return fmt.Errorf("backup destination already exists: %s", destinationAbs)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect backup destination: %w", err)
	}
	if err := s.Validate(); err != nil {
		return fmt.Errorf("refuse backup of invalid state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destinationAbs), 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	statement := "VACUUM INTO '" + strings.ReplaceAll(destinationAbs, "'", "''") + "'"
	if _, err := s.db.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("create SQLite snapshot: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(destinationAbs, 0o600); err != nil {
			return fmt.Errorf("restrict backup permissions: %w", err)
		}
	}
	if err := syncFile(destinationAbs); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(destinationAbs))
}

// RestoreSQLiteBackup validates a backup through the normal registry/control
// loaders before atomically installing it. For safety it refuses to overwrite an
// existing target; operators must stop WebGate and move the old database aside.
func RestoreSQLiteBackup(source, target string) error {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	if source == "" || target == "" {
		return errors.New("backup source and restore target are required")
	}
	sourceAbs, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if sourceAbs == targetAbs {
		return errors.New("restore target must differ from backup source")
	}
	if _, err := os.Stat(targetAbs); err == nil {
		return fmt.Errorf("restore target already exists: %s", targetAbs)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect restore target: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetAbs), 0o700); err != nil {
		return fmt.Errorf("create restore directory: %w", err)
	}

	sourceFile, err := os.Open(sourceAbs)
	if err != nil {
		return fmt.Errorf("open backup source: %w", err)
	}
	defer sourceFile.Close()
	tempFile, err := os.CreateTemp(filepath.Dir(targetAbs), ".webgate-restore-*.db")
	if err != nil {
		return fmt.Errorf("create restore staging file: %w", err)
	}
	tempPath := tempFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
			_ = os.Remove(tempPath + "-wal")
			_ = os.Remove(tempPath + "-shm")
		}
	}()
	if runtime.GOOS != "windows" {
		if err := tempFile.Chmod(0o600); err != nil {
			tempFile.Close()
			return fmt.Errorf("restrict restore staging permissions: %w", err)
		}
	}
	if _, err := io.Copy(tempFile, sourceFile); err != nil {
		tempFile.Close()
		return fmt.Errorf("copy backup into restore staging: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return fmt.Errorf("sync restore staging file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close restore staging file: %w", err)
	}

	registryStore, err := OpenSQLiteRegistryStore(tempPath)
	if err != nil {
		return fmt.Errorf("validate restored registry store: %w", err)
	}
	controlStore, err := OpenSQLiteControlStore(registryStore)
	if err != nil {
		registryStore.Close()
		return fmt.Errorf("validate restored control store: %w", err)
	}
	if err := controlStore.Validate(); err != nil {
		registryStore.Close()
		return fmt.Errorf("validate restored state: %w", err)
	}
	if _, err := registryStore.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		registryStore.Close()
		return fmt.Errorf("checkpoint restored state: %w", err)
	}
	if err := registryStore.Close(); err != nil {
		return fmt.Errorf("close validated restored state: %w", err)
	}
	_ = os.Remove(tempPath + "-wal")
	_ = os.Remove(tempPath + "-shm")
	if err := os.Rename(tempPath, targetAbs); err != nil {
		return fmt.Errorf("install restored state: %w", err)
	}
	cleanup = false
	return syncDirectory(filepath.Dir(targetAbs))
}

func syncFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open file for sync: %w", err)
	}
	defer file.Close()
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}
	return nil
}

func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory for sync: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}
