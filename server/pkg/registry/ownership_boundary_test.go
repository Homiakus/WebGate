package registry_test

import (
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func TestServiceRegistryOwnsStoredState(t *testing.T) {
	reg := registry.NewServiceRegistry()
	input := &domain.ProtectedService{
		ID:          "svc-owned",
		TenantID:    "tenant-1",
		WorkspaceID: "workspace-1",
		Name:        "Owned",
		Slug:        "owned",
		UpstreamURL: "http://127.0.0.1:8080",
		ExecArgs:    []string{"--mode", "safe"},
		Status:      domain.ServiceStatusActive,
	}
	if err := reg.Register(input); err != nil {
		t.Fatal(err)
	}

	input.Name = "mutated-outside"
	input.ExecArgs[0] = "--unsafe"

	stored, err := reg.GetByID(input.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Name != "Owned" || stored.ExecArgs[0] != "--mode" {
		t.Fatalf("registry retained caller alias: %#v", stored)
	}

	stored.Name = "mutated-copy"
	stored.ExecArgs[1] = "broken"
	storedAgain, err := reg.GetByID(input.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedAgain.Name != "Owned" || storedAgain.ExecArgs[1] != "safe" {
		t.Fatalf("GetByID exposed internal mutable state: %#v", storedAgain)
	}

	resolved, err := reg.ResolveBySlug("owned")
	if err != nil {
		t.Fatal(err)
	}
	resolved.ExecArgs[0] = "--resolved-mutation"
	if got, _ := reg.GetByID(input.ID); got.ExecArgs[0] != "--mode" {
		t.Fatalf("ResolveBySlug exposed internal mutable state: %#v", got)
	}

	listed := reg.List()
	if len(listed) != 1 {
		t.Fatalf("list length = %d", len(listed))
	}
	listed[0].ExecArgs[0] = "--list-mutation"
	if got, _ := reg.GetByID(input.ID); got.ExecArgs[0] != "--mode" {
		t.Fatalf("List exposed internal mutable state: %#v", got)
	}
}

func TestDeviceRegistryOwnsStoredState(t *testing.T) {
	dev, privateKey := newTestDevice(t, "device-owned", "user-owned")
	_ = privateKey
	reg := registry.NewDeviceRegistry()
	if err := reg.Enroll(dev); err != nil {
		t.Fatal(err)
	}

	originalLabel := dev.Label
	dev.Label = "mutated-outside"
	dev.AccountID = "mutated-account"

	stored, err := reg.GetDevice(dev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Label != originalLabel || stored.AccountID == "mutated-account" {
		t.Fatalf("device registry retained caller alias: %#v", stored)
	}

	stored.Label = "mutated-copy"
	stored.AccountID = "other-account"
	storedAgain, err := reg.GetDevice(dev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedAgain.Label != originalLabel || storedAgain.AccountID == "other-account" {
		t.Fatalf("GetDevice exposed internal mutable state: %#v", storedAgain)
	}

	listed := reg.List()
	if len(listed) != 1 {
		t.Fatalf("list length = %d", len(listed))
	}
	listed[0].Label = "mutated-list"
	if got, _ := reg.GetDevice(dev.ID); got.Label != originalLabel {
		t.Fatalf("device List exposed internal mutable state: %#v", got)
	}
}
