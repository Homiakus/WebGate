package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	_ "modernc.org/sqlite"
)

const (
	databaseSchemaVersion = 1
	recordSchemaVersion   = 1

	kindService = "service"
	kindDevice  = "device"
	kindRelease = "release"
)

var (
	ErrCorruptState         = errors.New("corrupt durable WebGate state")
	ErrUnsupportedSchema    = errors.New("unsupported durable state schema")
	ErrMissingDeviceAccount = errors.New("durable device is missing SecureAcces account ID")
)

type SQLiteRegistryStore struct {
	db   *sql.DB
	path string
}

func OpenSQLiteRegistryStore(path string) (*SQLiteRegistryStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("state database path is required")
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create state directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open state database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &SQLiteRegistryStore{db: db, path: path}
	if err := store.initialize(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o600); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("restrict state database permissions: %w", err)
		}
	}
	return store, nil
}

func (s *SQLiteRegistryStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteRegistryStore) initialize() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping state database: %w", err)
	}
	var journalMode string
	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&journalMode); err != nil {
		return fmt.Errorf("enable WAL: %w", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		return fmt.Errorf("enable WAL: database reported %q", journalMode)
	}
	for _, statement := range []string{
		"PRAGMA synchronous=FULL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
		"PRAGMA trusted_schema=OFF",
	} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("configure state database %q: %w", statement, err)
		}
	}
	return s.migrate(ctx)
}

func (s *SQLiteRegistryStore) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin state migration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS webgate_schema (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    version INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS registry_records (
    kind TEXT NOT NULL,
    entity_key TEXT NOT NULL,
    schema_version INTEGER NOT NULL,
    payload BLOB NOT NULL,
    checksum TEXT NOT NULL,
    updated_at_unix INTEGER NOT NULL,
    PRIMARY KEY (kind, entity_key),
    CHECK (kind IN ('service', 'device', 'release'))
);`); err != nil {
		return fmt.Errorf("create durable state schema: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO webgate_schema(singleton, version) VALUES(1, ?) ON CONFLICT(singleton) DO NOTHING`,
		databaseSchemaVersion,
	); err != nil {
		return fmt.Errorf("initialize durable state schema version: %w", err)
	}
	var version int
	if err := tx.QueryRowContext(ctx, `SELECT version FROM webgate_schema WHERE singleton = 1`).Scan(&version); err != nil {
		return fmt.Errorf("read durable state schema version: %w", err)
	}
	if version != databaseSchemaVersion {
		return fmt.Errorf("%w: database=%d supported=%d", ErrUnsupportedSchema, version, databaseSchemaVersion)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit durable state migration: %w", err)
	}
	return nil
}

func (s *SQLiteRegistryStore) SaveService(service *domain.ProtectedService) error {
	if service == nil || strings.TrimSpace(service.ID) == "" {
		return errors.New("service ID is required for persistence")
	}
	clone := *service
	clone.ExecArgs = append([]string(nil), service.ExecArgs...)
	clone.ProcessState = domain.ProcessStateStopped
	clone.ProcessPID = 0
	clone.StartedAt = nil
	return s.saveRecord(kindService, clone.ID, &clone)
}

func (s *SQLiteRegistryStore) DeleteService(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("service ID is required for durable deletion")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin durable service deletion: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM registry_records WHERE kind = ? AND entity_key = ?`, kindService, id); err != nil {
		return fmt.Errorf("delete durable service: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit durable service deletion: %w", err)
	}
	return nil
}

func (s *SQLiteRegistryStore) SaveDevice(device *domain.Device) error {
	if device == nil || strings.TrimSpace(device.ID) == "" {
		return errors.New("device ID is required for persistence")
	}
	if strings.TrimSpace(device.AccountID) == "" {
		return ErrMissingDeviceAccount
	}
	clone := *device
	return s.saveRecord(kindDevice, clone.ID, &clone)
}

func (s *SQLiteRegistryStore) SaveRelease(release *domain.Release) error {
	if release == nil || strings.TrimSpace(release.Version) == "" {
		return errors.New("release version is required for persistence")
	}
	clone := *release
	clone.Artifacts = append([]domain.PlatformArtifact(nil), release.Artifacts...)
	if release.PromotedAt != nil {
		promotedAt := *release.PromotedAt
		clone.PromotedAt = &promotedAt
	}
	return s.saveRecord(kindRelease, clone.Version, &clone)
}

func (s *SQLiteRegistryStore) saveRecord(kind, key string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal durable %s %q: %w", kind, key, err)
	}
	checksum := checksumFor(kind, key, payload)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin durable %s write: %w", kind, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO registry_records(kind, entity_key, schema_version, payload, checksum, updated_at_unix)
VALUES(?, ?, ?, ?, ?, ?)
ON CONFLICT(kind, entity_key) DO UPDATE SET
    schema_version = excluded.schema_version,
    payload = excluded.payload,
    checksum = excluded.checksum,
    updated_at_unix = excluded.updated_at_unix`,
		kind, key, recordSchemaVersion, payload, checksum, time.Now().UTC().UnixNano(),
	); err != nil {
		return fmt.Errorf("write durable %s %q: %w", kind, key, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit durable %s %q: %w", kind, key, err)
	}
	return nil
}

func (s *SQLiteRegistryStore) LoadServices() ([]*domain.ProtectedService, error) {
	records, err := s.loadKind(kindService)
	if err != nil {
		return nil, err
	}
	services := make([]*domain.ProtectedService, 0, len(records))
	for _, record := range records {
		var service domain.ProtectedService
		if err := json.Unmarshal(record.payload, &service); err != nil {
			return nil, fmt.Errorf("%w: decode service %q: %v", ErrCorruptState, record.key, err)
		}
		if service.ID != record.key {
			return nil, fmt.Errorf("%w: service key mismatch %q != %q", ErrCorruptState, service.ID, record.key)
		}
		service.ProcessState = domain.ProcessStateStopped
		service.ProcessPID = 0
		service.StartedAt = nil
		services = append(services, &service)
	}
	return services, nil
}

func (s *SQLiteRegistryStore) LoadDevices() ([]*domain.Device, error) {
	records, err := s.loadKind(kindDevice)
	if err != nil {
		return nil, err
	}
	devices := make([]*domain.Device, 0, len(records))
	for _, record := range records {
		var device domain.Device
		if err := json.Unmarshal(record.payload, &device); err != nil {
			return nil, fmt.Errorf("%w: decode device %q: %v", ErrCorruptState, record.key, err)
		}
		if device.ID != record.key || strings.TrimSpace(device.AccountID) == "" {
			return nil, fmt.Errorf("%w: invalid durable device identity %q", ErrCorruptState, record.key)
		}
		devices = append(devices, &device)
	}
	return devices, nil
}

func (s *SQLiteRegistryStore) LoadReleases() ([]*domain.Release, error) {
	records, err := s.loadKind(kindRelease)
	if err != nil {
		return nil, err
	}
	releases := make([]*domain.Release, 0, len(records))
	for _, record := range records {
		var release domain.Release
		if err := json.Unmarshal(record.payload, &release); err != nil {
			return nil, fmt.Errorf("%w: decode release %q: %v", ErrCorruptState, record.key, err)
		}
		if release.Version != record.key {
			return nil, fmt.Errorf("%w: release key mismatch %q != %q", ErrCorruptState, release.Version, record.key)
		}
		releases = append(releases, &release)
	}
	return releases, nil
}

type durableRecord struct {
	key     string
	payload []byte
}

func (s *SQLiteRegistryStore) loadKind(kind string) ([]durableRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := s.db.QueryContext(ctx, `
SELECT entity_key, schema_version, payload, checksum
FROM registry_records
WHERE kind = ?
ORDER BY entity_key`, kind)
	if err != nil {
		return nil, fmt.Errorf("load durable %s records: %w", kind, err)
	}
	defer rows.Close()

	var records []durableRecord
	for rows.Next() {
		var key, checksum string
		var schemaVersion int
		var payload []byte
		if err := rows.Scan(&key, &schemaVersion, &payload, &checksum); err != nil {
			return nil, fmt.Errorf("scan durable %s record: %w", kind, err)
		}
		if schemaVersion != recordSchemaVersion {
			return nil, fmt.Errorf("%w: %s %q record=%d supported=%d", ErrUnsupportedSchema, kind, key, schemaVersion, recordSchemaVersion)
		}
		if expected := checksumFor(kind, key, payload); !strings.EqualFold(checksum, expected) {
			return nil, fmt.Errorf("%w: checksum mismatch for %s %q", ErrCorruptState, kind, key)
		}
		records = append(records, durableRecord{key: key, payload: append([]byte(nil), payload...)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate durable %s records: %w", kind, err)
	}
	return records, nil
}

func checksumFor(kind, key string, payload []byte) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(kind))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(key))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(payload)
	return hex.EncodeToString(hash.Sum(nil))
}
