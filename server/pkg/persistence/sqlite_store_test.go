package persistence

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func TestSQLiteRegistryStoreSurvivesRestartWithoutRuntimeResurrection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "webgate-state.db")
	store, err := OpenSQLiteRegistryStore(path)
	if err != nil {
		t.Fatal(err)
	}

	services := registry.NewServiceRegistryWithPersistence(store)
	devices := registry.NewDeviceRegistryWithPersistence(store)
	releases := registry.NewReleaseRegistryWithPersistence(store)

	svc := &domain.ProtectedService{
		ID:          "svc-restart",
		TenantID:    "tenant-1",
		WorkspaceID: "workspace-1",
		Name:        "Restart Service",
		Slug:        "restart-service",
		UpstreamURL: "http://127.0.0.1:8123",
		Status:      domain.ServiceStatusActive,
	}
	if err := services.Register(svc); err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC()
	if err := services.UpdateProcessRuntime(svc.ID, domain.ProcessStateRunning, 7777, &started); err != nil {
		t.Fatal(err)
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dev := &domain.Device{
		ID:           "device-restart",
		AccountID:    "account-restart",
		UserID:       "legacy-user-restart",
		PublicKeyHex: hex.EncodeToString(publicKey),
		Algorithm:    "Ed25519",
		Platform:     domain.PlatformLinux,
		Architecture: domain.ArchX86_64,
	}
	if err := devices.Enroll(dev); err != nil {
		t.Fatal(err)
	}
	challenge, err := devices.CreateChallenge(dev.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := registry.ChallengeSigningPayload(challenge, dev)
	if err != nil {
		t.Fatal(err)
	}
	if err := devices.VerifyAndActivate(challenge.ChallengeID, hex.EncodeToString(ed25519.Sign(privateKey, payload))); err != nil {
		t.Fatal(err)
	}

	rel := &domain.Release{Version: "v9.9.9", SourceCommit: "deadbeef", Channel: "stable"}
	if err := releases.AddDraft(rel); err != nil {
		t.Fatal(err)
	}
	if err := releases.Verify(rel.Version); err != nil {
		t.Fatal(err)
	}
	if err := releases.Promote(rel.Version); err != nil {
		t.Fatal(err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenSQLiteRegistryStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	loadedServices, err := store.LoadServices()
	if err != nil {
		t.Fatal(err)
	}
	loadedDevices, err := store.LoadDevices()
	if err != nil {
		t.Fatal(err)
	}
	loadedReleases, err := store.LoadReleases()
	if err != nil {
		t.Fatal(err)
	}

	services2 := registry.NewServiceRegistryWithPersistence(store)
	devices2 := registry.NewDeviceRegistryWithPersistence(store)
	releases2 := registry.NewReleaseRegistryWithPersistence(store)
	if err := services2.Restore(loadedServices); err != nil {
		t.Fatal(err)
	}
	if err := devices2.Restore(loadedDevices); err != nil {
		t.Fatal(err)
	}
	if err := releases2.Restore(loadedReleases); err != nil {
		t.Fatal(err)
	}

	restoredService, err := services2.GetByID(svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restoredService.ProcessState != domain.ProcessStateStopped || restoredService.ProcessPID != 0 || restoredService.StartedAt != nil {
		t.Fatalf("runtime process state survived restart: %#v", restoredService)
	}
	restoredDevice, err := devices2.GetDevice(dev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restoredDevice.Status != domain.DeviceStatusActive || restoredDevice.AccountID != dev.AccountID {
		t.Fatalf("device identity/lifecycle did not survive restart: %#v", restoredDevice)
	}
	restoredReleases := releases2.List()
	if len(restoredReleases) != 1 || restoredReleases[0].Status != domain.ReleaseStatusPromoted {
		t.Fatalf("release lifecycle did not survive restart: %#v", restoredReleases)
	}
}

func TestSQLiteRegistryStoreRejectsCorruptedRecord(t *testing.T) {
	store, err := OpenSQLiteRegistryStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	release := &domain.Release{Version: "v-corrupt", SourceCommit: "abc", Channel: "stable"}
	if err := store.SaveRelease(release); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`UPDATE registry_records SET payload = ? WHERE kind = ? AND entity_key = ?`, []byte(`{"version":"tampered"}`), kindRelease, release.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadReleases(); !errors.Is(err, ErrCorruptState) {
		t.Fatalf("LoadReleases error = %v, want ErrCorruptState", err)
	}
}

func TestSQLiteRegistryStoreUsesWALAndFullSync(t *testing.T) {
	store, err := OpenSQLiteRegistryStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var journal string
	if err := store.db.QueryRow(`PRAGMA journal_mode`).Scan(&journal); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(journal, "wal") {
		t.Fatalf("journal_mode = %q, want WAL", journal)
	}
	var synchronous int
	if err := store.db.QueryRow(`PRAGMA synchronous`).Scan(&synchronous); err != nil {
		t.Fatal(err)
	}
	if synchronous != 2 {
		t.Fatalf("synchronous = %d, want FULL(2)", synchronous)
	}
}

func TestClosedSQLiteStoreCannotAdvanceRegistryMemory(t *testing.T) {
	store, err := OpenSQLiteRegistryStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	services := registry.NewServiceRegistryWithPersistence(store)
	svc := &domain.ProtectedService{
		ID:          "svc-closed-store",
		TenantID:    "tenant-1",
		WorkspaceID: "workspace-1",
		Name:        "Closed Store",
		Slug:        "closed-store",
		UpstreamURL: "http://127.0.0.1:8124",
		Status:      domain.ServiceStatusActive,
	}
	if err := services.Register(svc); err != nil {
		t.Fatal(err)
	}
	before, err := services.GetByID(svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := services.UpdateStatus(svc.ID, domain.ServiceStatusDisabled); err == nil {
		t.Fatal("UpdateStatus succeeded with closed durable store")
	}
	after, err := services.GetByID(svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != before.Status || after.Version != before.Version {
		t.Fatalf("memory advanced after durable failure: before=%#v after=%#v", before, after)
	}
}
