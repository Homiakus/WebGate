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

	// 1. Start service
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

	// 2. Prevent duplicate start
	if _, err := pm.StartService(svc.ID); err != process.ErrServiceAlreadyRunning {
		t.Fatalf("expected ErrServiceAlreadyRunning, got %v", err)
	}

	// 3. Restart
	reinst, err := pm.RestartService(svc.ID)
	if err != nil {
		t.Fatalf("failed to restart service: %v", err)
	}
	if reinst.State != string(domain.ProcessStateRunning) {
		t.Fatalf("expected running state after restart, got %s", reinst.State)
	}

	// 4. Stop
	if err := pm.StopService(svc.ID); err != nil {
		t.Fatalf("failed to stop service: %v", err)
	}

	// 5. Verify stopped
	if _, err := pm.StartService("non_existent_service"); err == nil {
		t.Fatalf("expected error for unknown service")
	}
}
