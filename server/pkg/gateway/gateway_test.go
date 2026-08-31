package gateway_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/gateway"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func TestServerGatewayE2E(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/docs/page1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Upstream documentation payload"))
	}))
	defer upstream.Close()

	svcReg := registry.NewServiceRegistry()
	_ = svcReg.Register(&domain.ProtectedService{
		ID:          "svc_docs",
		WorkspaceID: "ws_docs",
		Slug:        "docs",
		UpstreamURL: upstream.URL,
		Status:      domain.ServiceStatusActive,
	})

	devReg := registry.NewDeviceRegistry()
	enrollAndActivateTestDevice(t, devReg, "dev_123", "user_alice")

	authorizer := auth.NewSecureAccessAuthorizer()
	authorizer.RegisterSession(&auth.UserSession{
		SessionID: "sess_xyz",
		UserID:    "user_alice",
		DeviceID:  "dev_123",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	authorizer.SetMembership("user_alice", "ws_docs", domain.PermView)

	gw := gateway.NewServerGateway(svcReg, devReg, authorizer, gateway.GatewayConfig{
		ProxyTimeout: 5 * time.Second,
	})

	req := httptest.NewRequest(http.MethodGet, "/svc/docs/docs/page1", nil)
	req.Header.Set("X-WebGate-Session", "sess_xyz")
	req.Header.Set("X-WebGate-Device", "dev_123")
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "Upstream documentation payload" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}

	reqDenied := httptest.NewRequest(http.MethodDelete, "/svc/docs/docs/page1", nil)
	reqDenied.Header.Set("X-WebGate-Session", "sess_xyz")
	reqDenied.Header.Set("X-WebGate-Device", "dev_123")
	recDenied := httptest.NewRecorder()
	gw.ServeHTTP(recDenied, reqDenied)
	if recDenied.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for DELETE without PermDelete, got %d", recDenied.Code)
	}

	enrollAndActivateTestDevice(t, devReg, "dev_other", "user_alice")
	reqReplay := httptest.NewRequest(http.MethodGet, "/svc/docs/docs/page1", nil)
	reqReplay.Header.Set("X-WebGate-Session", "sess_xyz")
	reqReplay.Header.Set("X-WebGate-Device", "dev_other")
	recReplay := httptest.NewRecorder()
	gw.ServeHTTP(recReplay, reqReplay)
	if recReplay.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for session/device mismatch, got %d", recReplay.Code)
	}

	_ = devReg.RevokeDevice("dev_123")
	recRevoked := httptest.NewRecorder()
	gw.ServeHTTP(recRevoked, req)
	if recRevoked.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for revoked device, got %d", recRevoked.Code)
	}
}

func TestServerGatewayRenderingModelsE2E(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/spa/index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>FactoryOS SPA</title></head><body><div id="app"></div><script src="/spa/bundle.js"></script></body></html>`))
		case "/csr/api/v1/metrics":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"service":"factory","status":"optimal","temperature_c":42.5}`))
		case "/ssr/report":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>SSR Audit Report</title></head><body><article><h1>Report v1</h1><p>Verified</p></article></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	svcReg := registry.NewServiceRegistry()
	_ = svcReg.Register(&domain.ProtectedService{
		ID:          "svc_factory",
		WorkspaceID: "ws_factory",
		Slug:        "factory",
		UpstreamURL: upstream.URL,
		Status:      domain.ServiceStatusActive,
	})

	devReg := registry.NewDeviceRegistry()
	enrollAndActivateTestDevice(t, devReg, "dev_prod", "user_bob")

	authorizer := auth.NewSecureAccessAuthorizer()
	authorizer.RegisterSession(&auth.UserSession{
		SessionID: "sess_bob",
		UserID:    "user_bob",
		DeviceID:  "dev_prod",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	authorizer.SetMembership("user_bob", "ws_factory", domain.PermView)

	gw := gateway.NewServerGateway(svcReg, devReg, authorizer, gateway.GatewayConfig{
		ProxyTimeout: 5 * time.Second,
	})

	reqSpa := httptest.NewRequest(http.MethodGet, "/svc/factory/spa/index.html", nil)
	reqSpa.Header.Set("X-WebGate-Session", "sess_bob")
	reqSpa.Header.Set("X-WebGate-Device", "dev_prod")
	recSpa := httptest.NewRecorder()
	gw.ServeHTTP(recSpa, reqSpa)
	if recSpa.Code != http.StatusOK || recSpa.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("failed SPA request: code %d, body: %s", recSpa.Code, recSpa.Body.String())
	}

	reqCsr := httptest.NewRequest(http.MethodGet, "/svc/factory/csr/api/v1/metrics", nil)
	reqCsr.Header.Set("X-WebGate-Session", "sess_bob")
	reqCsr.Header.Set("X-WebGate-Device", "dev_prod")
	recCsr := httptest.NewRecorder()
	gw.ServeHTTP(recCsr, reqCsr)
	if recCsr.Code != http.StatusOK || recCsr.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("failed CSR request: code %d, body: %s", recCsr.Code, recCsr.Body.String())
	}

	reqSsr := httptest.NewRequest(http.MethodGet, "/svc/factory/ssr/report", nil)
	reqSsr.Header.Set("X-WebGate-Session", "sess_bob")
	reqSsr.Header.Set("X-WebGate-Device", "dev_prod")
	recSsr := httptest.NewRecorder()
	gw.ServeHTTP(recSsr, reqSsr)
	if recSsr.Code != http.StatusOK || recSsr.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("failed SSR request: code %d, body: %s", recSsr.Code, recSsr.Body.String())
	}
}
