package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/domain"
)

const testAuthorityToken = "0123456789abcdef0123456789abcdef"

func activeAccountDevice() *domain.Device {
	return &domain.Device{ID: "device-1", AccountID: "account-1", Status: domain.DeviceStatusActive}
}

func protectedService() *domain.ProtectedService {
	return &domain.ProtectedService{ID: "service-1", TenantID: "tenant-1", WorkspaceID: "workspace-1", Status: domain.ServiceStatusActive}
}

func newRemoteAuthorizer(t *testing.T, handler http.Handler) auth.ServiceAuthorizer {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	authorizer, err := auth.NewRemoteServiceAuthorizer(auth.RemoteServiceAuthorizerConfig{
		Endpoint:    server.URL,
		BridgeToken: testAuthorityToken,
		Timeout:     time.Second,
	})
	if err != nil {
		t.Fatalf("new remote authorizer: %v", err)
	}
	return authorizer
}

func TestRemoteServiceAuthorizerSendsServerOwnedAuthorizationContext(t *testing.T) {
	authorizer := newRemoteAuthorizer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/authorize" || r.Method != http.MethodPost {
			t.Fatalf("unexpected authority request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+testAuthorityToken {
			t.Fatalf("bridge authorization header = %q", got)
		}
		var request struct {
			Schema              string `json:"schema"`
			SessionToken        string `json:"session_token"`
			DeviceID            string `json:"device_id"`
			AccountID           string `json:"account_id"`
			TenantID            string `json:"tenant_id"`
			WorkspaceID         string `json:"workspace_id"`
			ResourceKind        string `json:"resource_kind"`
			ResourceID          string `json:"resource_id"`
			RequiredPermissions uint64 `json:"required_permissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Schema != "webgate.secureaccess.authorize/v1" || request.SessionToken != "session-secret" || request.DeviceID != "device-1" || request.AccountID != "account-1" || request.TenantID != "tenant-1" || request.WorkspaceID != "workspace-1" || request.ResourceKind != "service" || request.ResourceID != "service-1" || request.RequiredPermissions != uint64(domain.PermView) {
			t.Fatalf("unexpected authority request: %#v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"decision": "allow", "account_id": "account-1", "session_id": "session-1", "device_id": "device-1",
		})
	}))

	if err := authorizer.AuthorizeServiceAccess(context.Background(), "session-secret", activeAccountDevice(), protectedService(), domain.PermView); err != nil {
		t.Fatalf("authorization failed: %v", err)
	}
}

func TestRemoteServiceAuthorizerFailsClosedWithoutAccountBinding(t *testing.T) {
	var requests atomic.Int32
	authorizer := newRemoteAuthorizer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	device := activeAccountDevice()
	device.AccountID = ""
	if err := authorizer.AuthorizeServiceAccess(context.Background(), "session-secret", device, protectedService(), domain.PermView); err == nil {
		t.Fatal("device without SecureAcces AccountID unexpectedly authorized")
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("authority contacted for unbound device: %d requests", got)
	}
}

func TestRemoteServiceAuthorizerClassifiesDenyAndAuthorityFailure(t *testing.T) {
	t.Run("policy deny", func(t *testing.T) {
		authorizer := newRemoteAuthorizer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "authorization denied", http.StatusForbidden)
		}))
		if err := authorizer.AuthorizeServiceAccess(context.Background(), "session-secret", activeAccountDevice(), protectedService(), domain.PermView); err != auth.ErrAccessDenied {
			t.Fatalf("deny error = %v, want ErrAccessDenied", err)
		}
	})

	for name, status := range map[string]int{
		"bridge auth failure": http.StatusUnauthorized,
		"bad contract":        http.StatusBadRequest,
		"authority outage":    http.StatusServiceUnavailable,
	} {
		t.Run(name, func(t *testing.T) {
			authorizer := newRemoteAuthorizer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "authority unavailable", status)
			}))
			if err := authorizer.AuthorizeServiceAccess(context.Background(), "session-secret", activeAccountDevice(), protectedService(), domain.PermView); err != auth.ErrAuthorizationAuthorityUnavailable {
				t.Fatalf("authority error = %v, want ErrAuthorizationAuthorityUnavailable", err)
			}
		})
	}
}

func TestRemoteServiceAuthorizerRejectsMismatchedOrMalformedAllow(t *testing.T) {
	for name, response := range map[string]any{
		"wrong account": map[string]string{"decision": "allow", "account_id": "other", "session_id": "session-1", "device_id": "device-1"},
		"wrong device":  map[string]string{"decision": "allow", "account_id": "account-1", "session_id": "session-1", "device_id": "other"},
		"empty session": map[string]string{"decision": "allow", "account_id": "account-1", "session_id": "", "device_id": "device-1"},
		"deny in 200":   map[string]string{"decision": "deny", "account_id": "account-1", "session_id": "session-1", "device_id": "device-1"},
	} {
		t.Run(name, func(t *testing.T) {
			authorizer := newRemoteAuthorizer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(response)
			}))
			if err := authorizer.AuthorizeServiceAccess(context.Background(), "session-secret", activeAccountDevice(), protectedService(), domain.PermView); err != auth.ErrAuthorizationAuthorityUnavailable {
				t.Fatalf("malformed allow error = %v, want authority unavailable", err)
			}
		})
	}
}

func TestRemoteServiceAuthorizerEndpointAndSecretAreFailClosed(t *testing.T) {
	for _, endpoint := range []string{
		"http://0.0.0.0:8790",
		"http://192.168.1.10:8790",
		"http://localhost:8790",
		"https://127.0.0.1:8790",
		"http://127.0.0.1:8790/path",
		"http://user:pass@127.0.0.1:8790",
	} {
		if _, err := auth.NewRemoteServiceAuthorizer(auth.RemoteServiceAuthorizerConfig{Endpoint: endpoint, BridgeToken: testAuthorityToken}); err == nil {
			t.Fatalf("unsafe endpoint unexpectedly accepted: %s", endpoint)
		}
	}
	if _, err := auth.NewRemoteServiceAuthorizer(auth.RemoteServiceAuthorizerConfig{Endpoint: "http://127.0.0.1:8790", BridgeToken: "short"}); err == nil {
		t.Fatal("weak bridge token unexpectedly accepted")
	}
}
