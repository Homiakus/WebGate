package admin_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/admin"
	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/delivery"
	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func TestAdminAPIFlow(t *testing.T) {
	svcReg := registry.NewServiceRegistry()
	devReg := registry.NewDeviceRegistry()
	relReg := registry.NewReleaseRegistry()
	delSvc := delivery.NewTelegramDeliveryService(relReg)
	authorizer := auth.NewSecureAccessAuthorizer()

	api := admin.NewAdminAPI(svcReg, devReg, relReg, delSvc, authorizer)
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthcheck failed with code %d", rec.Code)
	}

	svcPayload := domain.ProtectedService{
		ID:          "svc_factory",
		WorkspaceID: "ws_factory",
		Slug:        "factory",
		Name:        "Factory OS",
		UpstreamURL: "http://127.0.0.1:8082",
		Status:      domain.ServiceStatusActive,
	}
	body, _ := json.Marshal(svcPayload)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/services", bytes.NewReader(body))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create service failed with code %d, body: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/services", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list services failed with code %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/audit", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("audit check failed with code %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("WebGate Control Plane")) {
		t.Fatalf("dashboard UI test failed")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/config", nil)
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get config failed with code %d", rec.Code)
	}

	routePayload := map[string]string{
		"service_id":   "svc_factory",
		"upstream_url": "http://127.0.0.1:9099",
	}
	rbody, _ := json.Marshal(routePayload)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/services/route", bytes.NewReader(rbody))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update route failed with code %d, body: %s", rec.Code, rec.Body.String())
	}

	fullPayload := map[string]interface{}{
		"id":              "svc_factory",
		"name":            "Factory OS Pro",
		"slug":            "factory",
		"port":            8099,
		"executable_path": "./bin/factory.exe",
		"working_dir":     "./bin",
	}
	fbody, _ := json.Marshal(fullPayload)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/services/update-full", bytes.NewReader(fbody))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update full service failed with code %d", rec.Code)
	}

	cfgPayload := map[string]interface{}{
		"server_name":           "Updated WebGate Gateway",
		"listen_addr":           ":8788",
		"proxy_timeout_seconds": 25,
	}
	cbody, _ := json.Marshal(cfgPayload)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/config/update", bytes.NewReader(cbody))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update config failed with code %d", rec.Code)
	}

	devPayload := domain.Device{
		ID:           "dev_test_1",
		UserID:       "usr_test",
		Label:        "Test Workstation",
		Platform:     domain.PlatformWindows,
		Architecture: domain.ArchX86_64,
		PublicKeyHex: strings.Repeat("00", 32),
		Algorithm:    "Ed25519",
	}
	dbody, _ := json.Marshal(devPayload)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/devices/enroll", bytes.NewReader(dbody))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll device failed with code %d, body: %s", rec.Code, rec.Body.String())
	}

	statusPayload := map[string]string{
		"device_id": "dev_test_1",
		"status":    "SUSPENDED",
	}
	sbody, _ := json.Marshal(statusPayload)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/devices/status", bytes.NewReader(sbody))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update device status failed with code %d", rec.Code)
	}

	delPayload := map[string]string{"service_id": "svc_factory"}
	dlbody, _ := json.Marshal(delPayload)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/services/delete", bytes.NewReader(dlbody))
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete service failed with code %d", rec.Code)
	}
}
