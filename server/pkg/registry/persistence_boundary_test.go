package registry_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

var errInjectedPersistence = errors.New("injected persistence failure")

type servicePersistenceProbe struct {
	saves  int
	failAt int
}

func (p *servicePersistenceProbe) SaveService(*domain.ProtectedService) error {
	p.saves++
	if p.failAt > 0 && p.saves == p.failAt {
		return errInjectedPersistence
	}
	return nil
}
func (*servicePersistenceProbe) DeleteService(string) error { return nil }

type devicePersistenceProbe struct {
	saves  int
	failAt int
}

func (p *devicePersistenceProbe) SaveDevice(*domain.Device) error {
	p.saves++
	if p.failAt > 0 && p.saves == p.failAt {
		return errInjectedPersistence
	}
	return nil
}

type releasePersistenceProbe struct {
	saves  int
	failAt int
}

func (p *releasePersistenceProbe) SaveRelease(*domain.Release) error {
	p.saves++
	if p.failAt > 0 && p.saves == p.failAt {
		return errInjectedPersistence
	}
	return nil
}

func validPersistenceDevice(t *testing.T, id, userID string) *domain.Device {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate device key: %v", err)
	}
	return &domain.Device{
		ID:           id,
		AccountID:    "account-" + id,
		UserID:       userID,
		PublicKeyHex: hex.EncodeToString(publicKey),
		Algorithm:    "Ed25519",
		Platform:     domain.PlatformLinux,
		Architecture: domain.ArchX86_64,
	}
}

func TestServicePersistenceFailureNeverCommitsMemory(t *testing.T) {
	probe := &servicePersistenceProbe{failAt: 1}
	reg := registry.NewServiceRegistryWithPersistence(probe)
	svc := &domain.ProtectedService{
		ID:          "svc-durable",
		TenantID:    "tenant-1",
		WorkspaceID: "workspace-1",
		Name:        "Durable",
		Slug:        "durable",
		UpstreamURL: "http://127.0.0.1:8090",
		Status:      domain.ServiceStatusActive,
	}
	if err := reg.Register(svc); !errors.Is(err, errInjectedPersistence) {
		t.Fatalf("Register error = %v, want injected persistence failure", err)
	}
	if got := reg.List(); len(got) != 0 {
		t.Fatalf("failed durable register mutated memory: %#v", got)
	}
}

func TestServiceUpdatePersistenceFailureRollsBackAndRuntimeIsEphemeral(t *testing.T) {
	probe := &servicePersistenceProbe{failAt: 2}
	reg := registry.NewServiceRegistryWithPersistence(probe)
	svc := &domain.ProtectedService{
		ID:          "svc-durable-update",
		TenantID:    "tenant-1",
		WorkspaceID: "workspace-1",
		Name:        "Durable",
		Slug:        "durable-update",
		UpstreamURL: "http://127.0.0.1:8091",
		Status:      domain.ServiceStatusActive,
	}
	if err := reg.Register(svc); err != nil {
		t.Fatal(err)
	}
	before, _ := reg.GetByID(svc.ID)
	if err := reg.UpdateStatus(svc.ID, domain.ServiceStatusDisabled); !errors.Is(err, errInjectedPersistence) {
		t.Fatalf("UpdateStatus error = %v, want injected persistence failure", err)
	}
	after, _ := reg.GetByID(svc.ID)
	if after.Status != before.Status || after.Version != before.Version {
		t.Fatalf("failed durable update changed memory: before=%#v after=%#v", before, after)
	}

	probe.failAt = 0
	started := time.Now().UTC()
	if err := reg.UpdateProcessRuntime(svc.ID, domain.ProcessStateRunning, 4242, &started); err != nil {
		t.Fatal(err)
	}
	if probe.saves != 2 {
		t.Fatalf("runtime-only state unexpectedly persisted: saves=%d", probe.saves)
	}
}

func TestServiceRestoreNeverResurrectsRunningProcess(t *testing.T) {
	reg := registry.NewServiceRegistryWithPersistence(&servicePersistenceProbe{})
	started := time.Now().UTC().Add(-time.Minute)
	persisted := &domain.ProtectedService{
		ID:           "svc-restart",
		TenantID:     "tenant-1",
		WorkspaceID:  "workspace-1",
		Name:         "Restart",
		Slug:         "restart",
		UpstreamURL:  "http://127.0.0.1:8092",
		Status:       domain.ServiceStatusActive,
		Version:      7,
		ProcessState: domain.ProcessStateRunning,
		ProcessPID:   9999,
		StartedAt:    &started,
	}
	if err := reg.Restore([]*domain.ProtectedService{persisted}); err != nil {
		t.Fatal(err)
	}
	got, err := reg.GetByID(persisted.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProcessState != domain.ProcessStateStopped || got.ProcessPID != 0 || got.StartedAt != nil {
		t.Fatalf("restore resurrected process runtime: %#v", got)
	}
	if got.Version != 7 {
		t.Fatalf("restore changed durable config version: %d", got.Version)
	}
}

func TestDeviceAndReleasePersistenceFailureNeverCommitsMemory(t *testing.T) {
	devProbe := &devicePersistenceProbe{failAt: 1}
	devReg := registry.NewDeviceRegistryWithPersistence(devProbe)
	dev := validPersistenceDevice(t, "device-durable", "user-durable")
	if err := devReg.Enroll(dev); !errors.Is(err, errInjectedPersistence) {
		t.Fatalf("Enroll error = %v, want injected persistence failure", err)
	}
	if got := devReg.List(); len(got) != 0 {
		t.Fatalf("failed durable enroll mutated memory: %#v", got)
	}

	relProbe := &releasePersistenceProbe{failAt: 1}
	relReg := registry.NewReleaseRegistryWithPersistence(relProbe)
	rel := &domain.Release{Version: "v-durable", SourceCommit: "abc", Channel: "stable"}
	if err := relReg.AddDraft(rel); !errors.Is(err, errInjectedPersistence) {
		t.Fatalf("AddDraft error = %v, want injected persistence failure", err)
	}
	if got := relReg.List(); len(got) != 0 {
		t.Fatalf("failed durable release mutated memory: %#v", got)
	}
}
