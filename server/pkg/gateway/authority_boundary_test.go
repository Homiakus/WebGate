package gateway_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/gateway"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func TestGatewayFailsClosedWhenAuthorizationAuthorityIsUnavailable(t *testing.T) {
	svcReg := registry.NewServiceRegistry()
	if err := svcReg.Register(&domain.ProtectedService{
		ID:          "svc_docs",
		TenantID:    "tenant_default",
		WorkspaceID: "ws_docs",
		Slug:        "docs",
		UpstreamURL: "http://127.0.0.1:65535",
		Status:      domain.ServiceStatusActive,
	}); err != nil {
		t.Fatal(err)
	}
	devReg := registry.NewDeviceRegistry()
	enrollAndActivateTestDevice(t, devReg, "dev_1", "account_1")

	for name, authorizer := range map[string]auth.ServiceAuthorizer{
		"explicit unavailable": auth.NewUnavailableServiceAuthorizer(),
		"nil defaults unavailable": nil,
	} {
		t.Run(name, func(t *testing.T) {
			gw := gateway.NewServerGateway(svcReg, devReg, authorizer, gateway.GatewayConfig{})
			req := httptest.NewRequest(http.MethodGet, "/svc/docs/index", nil)
			req.Header.Set("X-WebGate-Session", "opaque-session-token")
			req.Header.Set("X-WebGate-Device", "dev_1")
			rec := httptest.NewRecorder()
			gw.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
			}
			if rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("authorization failures must be non-cacheable")
			}
		})
	}
}

func TestGatewayDistinguishesPolicyDenyFromAuthorityOutage(t *testing.T) {
	svcReg := registry.NewServiceRegistry()
	if err := svcReg.Register(&domain.ProtectedService{
		ID:          "svc_docs",
		TenantID:    "tenant_default",
		WorkspaceID: "ws_docs",
		Slug:        "docs",
		UpstreamURL: "http://127.0.0.1:65535",
		Status:      domain.ServiceStatusActive,
	}); err != nil {
		t.Fatal(err)
	}
	devReg := registry.NewDeviceRegistry()
	enrollAndActivateTestDevice(t, devReg, "dev_1", "account_1")

	denied := auth.ServiceAuthorizerFunc(func(context.Context, string, *domain.Device, *domain.ProtectedService, domain.PermissionBits) error {
		return errors.New("policy deny")
	})
	gw := gateway.NewServerGateway(svcReg, devReg, denied, gateway.GatewayConfig{})
	req := httptest.NewRequest(http.MethodGet, "/svc/docs/index", nil)
	req.Header.Set("X-WebGate-Session", "opaque-session-token")
	req.Header.Set("X-WebGate-Device", "dev_1")
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 policy denial, got %d", rec.Code)
	}
}

func TestGatewayPropagatesRequestContextToAuthority(t *testing.T) {
	type contextKey string
	const key contextKey = "authority-test"

	svcReg := registry.NewServiceRegistry()
	if err := svcReg.Register(&domain.ProtectedService{
		ID:          "svc_docs",
		TenantID:    "tenant_default",
		WorkspaceID: "ws_docs",
		Slug:        "docs",
		UpstreamURL: "http://127.0.0.1:65535",
		Status:      domain.ServiceStatusActive,
	}); err != nil {
		t.Fatal(err)
	}
	devReg := registry.NewDeviceRegistry()
	enrollAndActivateTestDevice(t, devReg, "dev_1", "account_1")

	seen := false
	authorizer := auth.ServiceAuthorizerFunc(func(ctx context.Context, _ string, _ *domain.Device, _ *domain.ProtectedService, _ domain.PermissionBits) error {
		seen = ctx.Value(key) == "present"
		return errors.New("stop before upstream")
	})
	gw := gateway.NewServerGateway(svcReg, devReg, authorizer, gateway.GatewayConfig{})
	req := httptest.NewRequest(http.MethodGet, "/svc/docs/index", nil)
	req = req.WithContext(context.WithValue(req.Context(), key, "present"))
	req.Header.Set("X-WebGate-Session", "opaque-session-token")
	req.Header.Set("X-WebGate-Device", "dev_1")
	rec := httptest.NewRecorder()
	gw.ServeHTTP(rec, req)

	if !seen {
		t.Fatal("request context was not propagated to authorization authority")
	}
}
