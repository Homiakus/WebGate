package admin_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/admin"
	"github.com/Homiakus/WebGate/server/pkg/config"
	"github.com/Homiakus/WebGate/server/pkg/delivery"
	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/persistence"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func TestDurableAdminHandlerPersistsAuditAndRedactsConfigSecrets(t *testing.T) {
	registryStore, err := persistence.OpenSQLiteRegistryStore(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer registryStore.Close()
	controlStore, err := persistence.OpenSQLiteControlStore(registryStore)
	if err != nil {
		t.Fatal(err)
	}

	svcReg := registry.NewServiceRegistryWithPersistence(registryStore)
	devReg := registry.NewDeviceRegistryWithPersistence(registryStore)
	relReg := registry.NewReleaseRegistryWithPersistence(registryStore)
	api := admin.NewAdminAPI(svcReg, devReg, relReg, delivery.NewTelegramDeliveryService(relReg), nil)
	cfg := config.DefaultServerConfig()
	cfg.ListenAddr = "127.0.0.1:8787"
	cfg.AdminAddr = "127.0.0.1:8788"
	cfg.TelegramBotToken = "RUNTIME-ONLY-TELEGRAM-SECRET"
	api.InstallConfig(cfg)

	inner := http.NewServeMux()
	api.RegisterRoutes(inner)
	handler, err := admin.NewDurableAdminHandler(api, controlStore, inner)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/config", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get config failed: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), cfg.TelegramBotToken) || strings.Contains(rec.Body.String(), "telegram_bot_token") {
		t.Fatalf("config endpoint leaked secret: %s", rec.Body.String())
	}

	updateBody, _ := json.Marshal(map[string]any{
		"server_name":          "durable-admin-node",
		"telegram_bot_enabled": false,
	})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/config/update", bytes.NewReader(updateBody))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("durable config update failed: %d %s", rec.Code, rec.Body.String())
	}
	loadedCfg, err := controlStore.LoadControlConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loadedCfg == nil || loadedCfg.ServerName != "durable-admin-node" {
		t.Fatalf("admin update was not persisted: %#v", loadedCfg)
	}

	svc := domain.ProtectedService{ID: "svc_audit", TenantID: "tenant", WorkspaceID: "ws", Name: "Audit Service", Slug: "audit-service", UpstreamURL: "http://127.0.0.1:9100", Status: domain.ServiceStatusActive}
	serviceBody, _ := json.Marshal(svc)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/services", bytes.NewReader(serviceBody))
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("service mutation failed: %d %s", rec.Code, rec.Body.String())
	}
	events, err := controlStore.LoadAudit()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 2 {
		t.Fatalf("expected durable config and service audit events, got %#v", events)
	}
	for _, event := range events {
		if strings.Contains(event.Details, cfg.TelegramBotToken) {
			t.Fatal("durable audit leaked runtime secret")
		}
	}
}
