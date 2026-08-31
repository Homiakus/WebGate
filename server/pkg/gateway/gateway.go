package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

var (
	ErrInvalidRequestFormat = errors.New("invalid request format or missing service slug")
	ErrSSRFAttemptBlocked   = errors.New("upstream destination rejected by SSRF guard")
)

type GatewayConfig struct {
	ProxyTimeout time.Duration
}

type ServerGateway struct {
	services   *registry.ServiceRegistry
	devices    *registry.DeviceRegistry
	authorizer *auth.SecureAccessAuthorizer
	httpClient *http.Client
	config     GatewayConfig
}

func NewServerGateway(
	services *registry.ServiceRegistry,
	devices *registry.DeviceRegistry,
	authorizer *auth.SecureAccessAuthorizer,
	config GatewayConfig,
) *ServerGateway {
	if config.ProxyTimeout == 0 {
		config.ProxyTimeout = 15 * time.Second
	}

	return &ServerGateway{
		services:   services,
		devices:    devices,
		authorizer: authorizer,
		httpClient: &http.Client{
			Timeout: config.ProxyTimeout,
		},
		config: config,
	}
}

// ServeHTTP handles incoming proxied requests with strict authentication and SSRF filtering.
func (g *ServerGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. Extract session & device headers
	sessionID := r.Header.Get("X-WebGate-Session")
	deviceID := r.Header.Get("X-WebGate-Device")

	if sessionID == "" || deviceID == "" {
		http.Error(w, "missing session or device authentication context", http.StatusUnauthorized)
		return
	}

	// 2. Lookup device status
	dev, err := g.devices.GetDevice(deviceID)
	if err != nil || !dev.Status.IsAllowedAccess() {
		http.Error(w, "device unauthorized or revoked", http.StatusForbidden)
		return
	}

	// 3. Resolve target service from path `/svc/{slug}/...`
	slug, subpath := extractSlugAndSubpath(r.URL.Path)
	if slug == "" {
		http.Error(w, "invalid service path format; expected /svc/{slug}/...", http.StatusBadRequest)
		return
	}

	svc, err := g.services.ResolveBySlug(slug)
	if err != nil {
		http.Error(w, fmt.Sprintf("service '%s' not found", slug), http.StatusNotFound)
		return
	}

	if svc.Status != domain.ServiceStatusActive {
		http.Error(w, fmt.Sprintf("service '%s' is inactive", slug), http.StatusServiceUnavailable)
		return
	}

	// 4. Evaluate SecureAcces authorization
	reqPerm := mapMethodToPermission(r.Method)
	_, err = g.authorizer.AuthorizeServiceAccess(sessionID, dev.Status, svc, reqPerm)
	if err != nil {
		http.Error(w, "access denied by security policy", http.StatusForbidden)
		return
	}

	// 5. Build safe upstream URL (SSRF protected)
	targetURL, err := buildSafeUpstreamURL(svc.UpstreamURL, subpath, r.URL.RawQuery)
	if err != nil {
		http.Error(w, "invalid upstream destination", http.StatusBadGateway)
		return
	}

	// 6. Proxy request
	ctx, cancel := context.WithTimeout(r.Context(), g.config.ProxyTimeout)
	defer cancel()

	proxyReq, err := http.NewRequestWithContext(ctx, r.Method, targetURL, r.Body)
	if err != nil {
		http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// Copy allowed headers
	for k, vv := range r.Header {
		if strings.HasPrefix(strings.ToLower(k), "x-webgate-") {
			continue // Strip internal broker headers before upstream
		}
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}

	resp, err := g.httpClient.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func extractSlugAndSubpath(path string) (string, string) {
	trimmed := strings.TrimPrefix(path, "/")
	parts := strings.SplitN(trimmed, "/", 3)
	if len(parts) < 2 || parts[0] != "svc" {
		return "", ""
	}
	slug := parts[1]
	subpath := "/"
	if len(parts) == 3 {
		subpath = "/" + parts[2]
	}
	return slug, subpath
}

func mapMethodToPermission(method string) domain.PermissionBits {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return domain.PermView
	case http.MethodPost:
		return domain.PermUpload | domain.PermEdit
	case http.MethodPut, http.MethodPatch:
		return domain.PermEdit
	case http.MethodDelete:
		return domain.PermDelete
	default:
		return domain.PermView
	}
}

func buildSafeUpstreamURL(baseUpstream, subpath, query string) (string, error) {
	u, err := url.Parse(baseUpstream)
	if err != nil {
		return "", err
	}

	// Reject external non-http schemes
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrSSRFAttemptBlocked
	}

	u.Path = strings.TrimSuffix(u.Path, "/") + subpath
	if query != "" {
		u.RawQuery = query
	}
	return u.String(), nil
}
