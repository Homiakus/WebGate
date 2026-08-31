package process_test

import (
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/process"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func TestProcessManagerLifecycle(t *testing.T) {
	svcReg := registry.NewServiceRegistry()
	svc := &domain.ProtectedService{
		ID:             "svc_test_node",
		WorkspaceID:    "ws_default",
		Name:           "Test Node",
		Slug:           "testnode",
		Port:           8090,
		ExecutablePath: ":mock",
		Status:         domain.ServiceStatusActive,
	}
	if err := svcReg.Register(svc); err != nil {
		t.Fatalf("failed to register test service: %v", err)
	}

	pm := process.NewProcessManager(svcReg)

	inst, err := pm.StartService(svc.ID)
	if err != nil {
		t.Fatalf("failed to start service: %v", err)
	}
	if inst.PID <= 0 {
		t.Fatalf("expected valid PID, got %d", inst.PID)
	}
	if inst.State != string(domain.ProcessStateRunning) {
		t.Fatalf("expected state RUNNING, got %s", inst.State)
	}
	stored, err := svcReg.GetByID(svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ProcessState != domain.ProcessStateRunning || stored.ProcessPID != inst.PID || stored.StartedAt == nil {
		t.Fatalf("registry runtime not updated on start: %#v", stored)
	}
	configVersion := stored.Version

	if _, err := pm.StartService(svc.ID); err != process.ErrServiceAlreadyRunning {
		t.Fatalf("expected ErrServiceAlreadyRunning, got %v", err)
	}

	reinst, err := pm.RestartService(svc.ID)
	if err != nil {
		t.Fatalf("failed to restart service: %v", err)
	}
	if reinst.State != string(domain.ProcessStateRunning) {
		t.Fatalf("expected running state after restart, got %s", reinst.State)
	}
	stored, err = svcReg.GetByID(svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ProcessState != domain.ProcessStateRunning || stored.ProcessPID != reinst.PID {
		t.Fatalf("registry runtime not updated on restart: %#v", stored)
	}
	if stored.Version != configVersion {
		t.Fatalf("runtime lifecycle changed config version: got %d want %d", stored.Version, configVersion)
	}

	if err := pm.StopService(svc.ID); err != nil {
		t.Fatalf("failed to stop service: %v", err)
	}
	stored, err = svcReg.GetByID(svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ProcessState != domain.ProcessStateStopped || stored.ProcessPID != 0 || stored.StartedAt != nil {
		t.Fatalf("registry runtime not cleared on stop: %#v", stored)
	}
	if stored.Version != configVersion {
		t.Fatalf("stopping runtime changed config version: got %d want %d", stored.Version, configVersion)
	}
	if _, err := pm.StartService("non_existent_service"); err == nil {
		t.Fatalf("expected error for unknown service")
	}
}

func TestProcessManagerDoesNotFakeRunningOnSpawnFailure(t *testing.T) {
	svcReg := registry.NewServiceRegistry()
	svc := &domain.ProtectedService{
		ID:             "svc_missing_binary",
		WorkspaceID:    "ws_default",
		Name:           "Missing Binary",
		Slug:           "missing",
		Port:           8091,
		ExecutablePath: "__webgate_binary_that_does_not_exist__",
		Status:         domain.ServiceStatusActive,
	}
	if err := svcReg.Register(svc); err != nil {
		t.Fatalf("failed to register service: %v", err)
	}

	pm := process.NewProcessManager(svcReg)
	if inst, err := pm.StartService(svc.ID); err == nil || inst != nil {
		t.Fatalf("expected real spawn failure, got instance=%v err=%v", inst, err)
	}
	stored, err := svcReg.GetByID(svc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.ProcessState == domain.ProcessStateRunning || stored.ProcessPID != 0 || stored.StartedAt != nil {
		t.Fatalf("spawn failure must not report RUNNING: %#v", stored)
	}
	if _, exists := pm.GetProcess(svc.ID); exists {
		t.Fatalf("failed process must not be stored as an active instance")
	}
}
