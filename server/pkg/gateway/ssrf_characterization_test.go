package gateway

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func TestBuildSafeUpstreamURLRejectsDangerousDestinations(t *testing.T) {
	tests := []string{
		"http://169.254.169.254/latest/meta-data",
		"http://8.8.8.8/",
		"http://0.0.0.0:8080/",
		"http://224.0.0.1/",
		"http://user:pass@127.0.0.1:8080/",
		"http://example.com/",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if _, err := buildSafeUpstreamURL(raw, "/", ""); !errors.Is(err, ErrSSRFAttemptBlocked) {
				t.Fatalf("expected SSRF rejection for %q, got %v", raw, err)
			}
		})
	}
}

func TestBuildSafeUpstreamURLAllowsExplicitPrivateAndLoopbackLiterals(t *testing.T) {
	allowed := []string{
		"http://127.0.0.1:8080",
		"http://[::1]:8080",
		"http://10.42.0.7:8080",
		"http://172.20.1.2:8080",
		"http://192.168.50.10:8080",
		"http://localhost:8080",
	}

	for _, raw := range allowed {
		t.Run(raw, func(t *testing.T) {
			if _, err := buildSafeUpstreamURL(raw, "/health", ""); err != nil {
				t.Fatalf("expected allowed upstream %q, got %v", raw, err)
			}
		})
	}
}

func TestServiceRegistryUpdateRouteCannotBypassUpstreamValidation(t *testing.T) {
	reg := registry.NewServiceRegistry()
	svc := &domain.ProtectedService{
		ID:          "svc_docs",
		WorkspaceID: "ws_docs",
		Slug:        "docs",
		UpstreamURL: "http://127.0.0.1:8080",
		Status:      domain.ServiceStatusActive,
	}
	if err := reg.Register(svc); err != nil {
		t.Fatalf("register: %v", err)
	}

	if err := reg.UpdateRoute("svc_docs", "http://169.254.169.254/latest/meta-data"); !errors.Is(err, domain.ErrInvalidUpstreamURL) {
		t.Fatalf("expected invalid upstream error, got %v", err)
	}

	got, err := reg.GetByID("svc_docs")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.UpstreamURL != "http://127.0.0.1:8080" {
		t.Fatalf("unsafe route mutation was applied: %q", got.UpstreamURL)
	}
}

func TestGatewayHTTPClientDoesNotFollowServerSideRedirects(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, target.URL, http.StatusFound)
	}))
	defer redirector.Close()

	gw := NewServerGateway(nil, nil, nil, GatewayConfig{})
	resp, err := gw.httpClient.Get(redirector.URL)
	if err != nil {
		t.Fatalf("redirect request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected un-followed 302, got %d", resp.StatusCode)
	}
	if targetHits.Load() != 0 {
		t.Fatalf("redirect target was contacted server-side %d times", targetHits.Load())
	}
}
