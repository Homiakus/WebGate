package admin_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/admin"
	"github.com/Homiakus/WebGate/server/pkg/config"
	"github.com/Homiakus/WebGate/server/pkg/delivery"
	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

type failingDurableAdminStore struct{}

func (f failingDurableAdminStore) LoadAudit() ([]domain.AuditEvent, error) { return nil, nil }
func (f failingDurableAdminStore) AppendAuditBatch([]domain.AuditEvent) error {
	return errors.New("durable audit unavailable")
}
func (f failingDurableAdminStore) LoadControlConfig() (*config.DurableServerConfig, error) {
	return nil, nil
}
func (f failingDurableAdminStore) SaveControlConfig(*config.DurableServerConfig) error {
	return errors.New("durable config unavailable")
}
func (f failingDurableAdminStore) SaveControlConfigWithAudit(*config.DurableServerConfig, domain.AuditEvent) error {
	return errors.New("durable config unavailable")
}

func TestDurableConfigFailureDoesNotAdvanceMemory(t *testing.T) {
	svcReg := registry.NewServiceRegistry()
	devReg := registry.NewDeviceRegistry()
	relReg := registry.NewReleaseRegistry()
	api := admin.NewAdminAPI(svcReg, devReg, relReg, delivery.NewTelegramDeliveryService(relReg), nil)
	cfg := config.DefaultServerConfig()
	cfg.ServerName = "before-durable-failure"
	cfg.ListenAddr = "127.0.0.1:8787"
	cfg.AdminAddr = "127.0.0.1:8788"
	api.InstallConfig(cfg)

	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	handler, err := admin.NewDurableAdminHandler(api, failingDurableAdminStore{}, mux)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/config/update", bytes.NewBufferString(`{"server_name":"after-durable-failure"}`))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected fail-closed 503, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := api.ConfigSnapshot().ServerName; got != "before-durable-failure" {
		t.Fatalf("memory advanced despite durable failure: %q", got)
	}
}
