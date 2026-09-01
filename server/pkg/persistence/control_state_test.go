package persistence_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/config"
	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/persistence"
)

func TestControlStatePersistsAuditConfigAndBackupRestore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "state.db")
	registryStore, err := persistence.OpenSQLiteRegistryStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	control, err := persistence.OpenSQLiteControlStore(registryStore)
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultServerConfig()
	cfg.ServerName = "before-backup"
	cfg.TelegramBotToken = "DO-NOT-PERSIST-THIS-TOKEN"
	if err := control.SaveControlConfig(config.DurableSnapshot(cfg)); err != nil {
		t.Fatal(err)
	}

	event := domain.AuditEvent{
		ID:        "audit-0001",
		Action:    domain.AuditActionServiceUpdated,
		ActorID:   "admin",
		TargetID:  "config",
		Details:   "durable change",
		Timestamp: time.Now().UTC(),
	}
	if err := control.AppendAuditBatch([]domain.AuditEvent{event}); err != nil {
		t.Fatal(err)
	}
	if err := control.AppendAuditBatch([]domain.AuditEvent{event}); err != nil {
		t.Fatalf("idempotent audit replay should succeed: %v", err)
	}

	loadedCfg, err := control.LoadControlConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loadedCfg == nil || loadedCfg.ServerName != "before-backup" {
		t.Fatalf("unexpected durable config: %#v", loadedCfg)
	}
	events, err := control.LoadAudit()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("unexpected audit log: %#v", events)
	}

	backupPath := filepath.Join(dir, "backup.db")
	if err := control.BackupTo(backupPath); err != nil {
		t.Fatal(err)
	}
	backupBytes, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(backupBytes, []byte(cfg.TelegramBotToken)) {
		t.Fatal("backup contains runtime-only Telegram secret")
	}

	cfg.ServerName = "after-backup"
	if err := control.SaveControlConfig(config.DurableSnapshot(cfg)); err != nil {
		t.Fatal(err)
	}
	if err := registryStore.Close(); err != nil {
		t.Fatal(err)
	}

	restoredPath := filepath.Join(dir, "restored.db")
	if err := persistence.RestoreSQLiteBackup(backupPath, restoredPath); err != nil {
		t.Fatal(err)
	}
	restoredRegistry, err := persistence.OpenSQLiteRegistryStore(restoredPath)
	if err != nil {
		t.Fatal(err)
	}
	defer restoredRegistry.Close()
	restoredControl, err := persistence.OpenSQLiteControlStore(restoredRegistry)
	if err != nil {
		t.Fatal(err)
	}
	restoredCfg, err := restoredControl.LoadControlConfig()
	if err != nil {
		t.Fatal(err)
	}
	if restoredCfg == nil || restoredCfg.ServerName != "before-backup" {
		t.Fatalf("restore did not reproduce backup snapshot: %#v", restoredCfg)
	}
	restoredEvents, err := restoredControl.LoadAudit()
	if err != nil {
		t.Fatal(err)
	}
	if len(restoredEvents) != 1 || restoredEvents[0].ID != event.ID {
		t.Fatalf("audit log was not restored: %#v", restoredEvents)
	}
}

func TestAuditTableIsAppendOnly(t *testing.T) {
	dir := t.TempDir()
	registryStore, err := persistence.OpenSQLiteRegistryStore(filepath.Join(dir, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer registryStore.Close()
	control, err := persistence.OpenSQLiteControlStore(registryStore)
	if err != nil {
		t.Fatal(err)
	}

	event := domain.AuditEvent{ID: "audit-immutable", Action: domain.AuditActionAccessDenied, ActorID: "actor", TargetID: "target", Details: "deny", Timestamp: time.Now().UTC()}
	if err := control.AppendAuditBatch([]domain.AuditEvent{event}); err != nil {
		t.Fatal(err)
	}
	if err := control.TestOnlyAttemptAuditMutation(event.ID); err == nil {
		t.Fatal("audit UPDATE unexpectedly succeeded")
	}
	if err := control.TestOnlyAttemptAuditDeletion(event.ID); err == nil {
		t.Fatal("audit DELETE unexpectedly succeeded")
	}
}
