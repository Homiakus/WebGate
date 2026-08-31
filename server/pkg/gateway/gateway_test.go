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
	// 1. Mock upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/docs/page1" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Upstream documentation payload"))
	}))
	defer upstream.Close()

	// 2. Setup registries & auth
	svcReg := registry.NewServiceRegistry()
	_ = svcReg.Register(&domain.ProtectedService{
		ID:          "svc_docs",
		WorkspaceID: "ws_docs",
		Slug:        "docs",
		UpstreamURL: upstream.URL,
		Status:      domain.ServiceStatusActive,
	})

	devReg := registry.NewDeviceRegistry()
	_ = devReg.Enroll(&domain.Device{
		ID:           "dev_123",
		UserID:       "user_alice",
		Status:       domain.DeviceStatusActive,
		PublicKeyHex: "abcd",
	})
	// Activate device
	chal, _ := devReg.CreateChallenge("dev_123", time.Minute)
	_ = devReg.VerifyAndActivate(chal.ChallengeID, "signature_ok")

	authorizer := auth.NewSecureAccessAuthorizer()
	authorizer.RegisterSession(&auth.UserSession{
		SessionID: "sess_xyz",
		UserID:    "user_alice",
		DeviceID:  "dev_123",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	authorizer.SetMembership("user_alice", "ws_docs", domain.PermView)

	// 3. Create Gateway
	gw := gateway.NewServerGateway(svcReg, devReg, authorizer, gateway.GatewayConfig{
		ProxyTimeout: 5 * time.Second,
	})

	// 4. Test Authorized Request -> 200 OK
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

	// 5. Test Unauthorized / Missing Permission -> 403 Forbidden
	reqDenied := httptest.NewRequest(http.MethodDelete, "/svc/docs/docs/page1", nil)
	reqDenied.Header.Set("X-WebGate-Session", "sess_xyz")
	reqDenied.Header.Set("X-WebGate-Device", "dev_123")
	recDenied := httptest.NewRecorder()

	gw.ServeHTTP(recDenied, reqDenied)
	if recDenied.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for DELETE without PermDelete, got %d", recDenied.Code)
	}

	// 6. Test Revoked Device -> 403 Forbidden
	_ = devReg.RevokeDevice("dev_123")
	recRevoked := httptest.NewRecorder()
	gw.ServeHTTP(recRevoked, req)
	if recRevoked.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for revoked device, got %d", recRevoked.Code)
	}
}

func TestServerGatewayRenderingModelsE2E(t *testing.T) {
	// Mock SPA, CSR, and SSR endpoints on upstream
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
	_ = devReg.Enroll(&domain.Device{
		ID:           "dev_prod",
		UserID:       "user_bob",
		Status:       domain.DeviceStatusActive,
		PublicKeyHex: "beef1234",
	})
	chal, _ := devReg.CreateChallenge("dev_prod", time.Minute)
	_ = devReg.VerifyAndActivate(chal.ChallengeID, "sig_ok")

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

	// 1. SPA shell loading
	reqSpa := httptest.NewRequest(http.MethodGet, "/svc/factory/spa/index.html", nil)
	reqSpa.Header.Set("X-WebGate-Session", "sess_bob")
	reqSpa.Header.Set("X-WebGate-Device", "dev_prod")
	recSpa := httptest.NewRecorder()
	gw.ServeHTTP(recSpa, reqSpa)
	if recSpa.Code != http.StatusOK || recSpa.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("failed SPA request: code %d, body: %s", recSpa.Code, recSpa.Body.String())
	}

	// 2. CSR API JSON data fetch
	reqCsr := httptest.NewRequest(http.MethodGet, "/svc/factory/csr/api/v1/metrics", nil)
	reqCsr.Header.Set("X-WebGate-Session", "sess_bob")
	reqCsr.Header.Set("X-WebGate-Device", "dev_prod")
	recCsr := httptest.NewRecorder()
	gw.ServeHTTP(recCsr, reqCsr)
	if recCsr.Code != http.StatusOK || recCsr.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("failed CSR request: code %d, body: %s", recCsr.Code, recCsr.Body.String())
	}

	// 3. SSR pre-rendered document
	reqSsr := httptest.NewRequest(http.MethodGet, "/svc/factory/ssr/report", nil)
	reqSsr.Header.Set("X-WebGate-Session", "sess_bob")
	reqSsr.Header.Set("X-WebGate-Device", "dev_prod")
	recSsr := httptest.NewRecorder()
	gw.ServeHTTP(recSsr, reqSsr)
	if recSsr.Code != http.StatusOK || recSsr.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("failed SSR request: code %d, body: %s", recSsr.Code, recSsr.Body.String())
	}
}
