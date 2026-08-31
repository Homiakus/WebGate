package gateway_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/gateway"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func TestAdversarialE2EQualification(t *testing.T) {
	docsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-WebGate-Session") != "" {
			http.Error(w, "internal session header leaked", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("docs_content"))
	}))
	defer docsServer.Close()

	factoryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("factory_content"))
	}))
	defer factoryServer.Close()

	svcReg := registry.NewServiceRegistry()
	_ = svcReg.Register(&domain.ProtectedService{
		ID:          "svc_docs",
		WorkspaceID: "ws_docs",
		Slug:        "docs",
		UpstreamURL: docsServer.URL,
		Status:      domain.ServiceStatusActive,
	})
	_ = svcReg.Register(&domain.ProtectedService{
		ID:          "svc_factory",
		WorkspaceID: "ws_factory",
		Slug:        "factory",
		UpstreamURL: factoryServer.URL,
		Status:      domain.ServiceStatusActive,
	})

	devReg := registry.NewDeviceRegistry()
	enrollAndActivateTestDevice(t, devReg, "dev_alice", "alice")

	authorizer := auth.NewSecureAccessAuthorizer()
	authorizer.RegisterSession(&auth.UserSession{
		SessionID: "sess_alice",
		UserID:    "alice",
		DeviceID:  "dev_alice",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	authorizer.SetMembership("alice", "ws_docs", domain.PermView|domain.PermEdit)

	gw := gateway.NewServerGateway(svcReg, devReg, authorizer, gateway.GatewayConfig{
		ProxyTimeout: 5 * time.Second,
	})

	t.Run("authorized_service_access", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/svc/docs/index", nil)
		req.Header.Set("X-WebGate-Session", "sess_alice")
		req.Header.Set("X-WebGate-Device", "dev_alice")
		rec := httptest.NewRecorder()

		gw.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != "docs_content" {
			t.Fatalf("expected 200 docs_content, got %d, %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("multi_service_isolation_denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/svc/factory/dashboard", nil)
		req.Header.Set("X-WebGate-Session", "sess_alice")
		req.Header.Set("X-WebGate-Device", "dev_alice")
		rec := httptest.NewRecorder()

		gw.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for unauthorized service, got %d", rec.Code)
		}
	})

	t.Run("unauthenticated_requests", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/svc/docs/index", nil)
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("instant_service_disable", func(t *testing.T) {
		_ = svcReg.UpdateStatus("svc_docs", domain.ServiceStatusDisabled)

		req := httptest.NewRequest(http.MethodGet, "/svc/docs/index", nil)
		req.Header.Set("X-WebGate-Session", "sess_alice")
		req.Header.Set("X-WebGate-Device", "dev_alice")
		rec := httptest.NewRecorder()

		gw.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 for disabled service, got %d", rec.Code)
		}
		_ = svcReg.UpdateStatus("svc_docs", domain.ServiceStatusActive)
	})

	t.Run("concurrent_adversarial_requests", func(t *testing.T) {
		var wg sync.WaitGroup
		concurrency := 20

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				slug := "docs"
				expectedStatus := http.StatusOK
				if idx%2 == 1 {
					slug = "factory"
					expectedStatus = http.StatusForbidden
				}

				req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/svc/%s/page_%d", slug, idx), nil)
				req.Header.Set("X-WebGate-Session", "sess_alice")
				req.Header.Set("X-WebGate-Device", "dev_alice")
				rec := httptest.NewRecorder()

				gw.ServeHTTP(rec, req)
				if rec.Code != expectedStatus {
					t.Errorf("request %d to %s expected status %d, got %d", idx, slug, expectedStatus, rec.Code)
				}
			}(i)
		}
		wg.Wait()
	})
}
