package gateway

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		"http://10.0.0.1:8080/",
		"http://172.16.0.1:8080/",
		"http://192.168.1.1:8080/",
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

func TestBuildSafeUpstreamURLAllowsOnlyExplicitLoopback(t *testing.T) {
	allowed := []string{
		"http://127.0.0.1:8080",
		"http://127.0.0.2:8080",
		"http://[::1]:8080",
		"http://localhost:8080",
	}

	for _, raw := range allowed {
		t.Run(raw, func(t *testing.T) {
			if _, err := buildSafeUpstreamURL(raw, "/health", ""); err != nil {
				t.Fatalf("expected allowed loopback upstream %q, got %v", raw, err)
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

	for _, unsafeRoute := range []string{
		"http://169.254.169.254/latest/meta-data",
		"http://192.168.1.1:8080/admin",
		"http://example.com/",
	} {
		if err := reg.UpdateRoute("svc_docs", unsafeRoute); !errors.Is(err, domain.ErrInvalidUpstreamURL) {
			t.Fatalf("expected invalid upstream error for %q, got %v", unsafeRoute, err)
		}
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

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
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

func TestRewriteSafeRedirectLocationKeepsBrowserInsideServiceNamespace(t *testing.T) {
	requestURL, err := url.Parse("http://127.0.0.1:8080/private/start")
	if err != nil {
		t.Fatal(err)
	}
	resp := &http.Response{
		Header:  http.Header{"Location": []string{"/login?next=%2Fdocs#step"}},
		Request: &http.Request{URL: requestURL},
	}

	if err := rewriteSafeRedirectLocation(resp, "http://127.0.0.1:8080", "docs"); err != nil {
		t.Fatalf("same-origin redirect rejected: %v", err)
	}
	if got := resp.Header.Get("Location"); got != "/svc/docs/login?next=%2Fdocs#step" {
		t.Fatalf("unexpected rewritten location: %q", got)
	}
}

func TestRewriteSafeRedirectLocationRejectsCrossOriginEscapes(t *testing.T) {
	requestURL, err := url.Parse("http://127.0.0.1:8080/private/start")
	if err != nil {
		t.Fatal(err)
	}

	for _, location := range []string{
		"http://127.0.0.1:9090/admin",
		"http://192.168.1.1/router",
		"http://example.com/login",
	} {
		t.Run(location, func(t *testing.T) {
			resp := &http.Response{
				Header:  http.Header{"Location": []string{location}},
				Request: &http.Request{URL: requestURL},
			}
			if err := rewriteSafeRedirectLocation(resp, "http://127.0.0.1:8080", "docs"); !errors.Is(err, ErrSSRFAttemptBlocked) {
				t.Fatalf("expected redirect escape rejection for %q, got %v", location, err)
			}
		})
	}
}
