package persistence

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/config"
)

func TestCorruptControlConfigFailsClosedAndRestoreRefusesIt(t *testing.T) {
	dir := t.TempDir()
	livePath := filepath.Join(dir, "live.db")
	registryStore, err := OpenSQLiteRegistryStore(livePath)
	if err != nil {
		t.Fatal(err)
	}
	control, err := OpenSQLiteControlStore(registryStore)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultServerConfig()
	cfg.ServerName = "trusted-control-state"
	if err := control.SaveControlConfig(config.DurableSnapshot(cfg)); err != nil {
		t.Fatal(err)
	}
	backupPath := filepath.Join(dir, "backup.db")
	if err := control.BackupTo(backupPath); err != nil {
		t.Fatal(err)
	}
	if err := registryStore.Close(); err != nil {
		t.Fatal(err)
	}

	tamperDB, err := sql.Open("sqlite", backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tamperDB.Exec(`UPDATE control_config SET payload = '{"server_name":"tampered"}' WHERE singleton = 1`); err != nil {
		tamperDB.Close()
		t.Fatal(err)
	}
	if err := tamperDB.Close(); err != nil {
		t.Fatal(err)
	}

	checkStore, err := OpenSQLiteRegistryStore(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	checkControl, err := OpenSQLiteControlStore(checkStore)
	if err != nil {
		checkStore.Close()
		t.Fatal(err)
	}
	if _, err := checkControl.LoadControlConfig(); !errors.Is(err, ErrCorruptState) {
		checkStore.Close()
		t.Fatalf("corrupt control config did not fail closed: %v", err)
	}
	if err := checkStore.Close(); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "restored.db")
	if err := RestoreSQLiteBackup(backupPath, target); !errors.Is(err, ErrCorruptState) {
		t.Fatalf("restore accepted corrupt control state: %v", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed restore installed a target file: %v", err)
	}
}
