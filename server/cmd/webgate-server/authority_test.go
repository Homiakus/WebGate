package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/domain"
)

const testAuthorityBridgeToken = "0123456789abcdef0123456789abcdef"

func TestServiceAuthorizerFromEnvironmentDefaultsFailClosed(t *testing.T) {
	t.Setenv("WEBGATE_AUTHORITY_URL", "")
	t.Setenv("WEBGATE_AUTHORITY_TOKEN", "")
	authorizer, endpoint, err := serviceAuthorizerFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != "" {
		t.Fatalf("unconfigured authority endpoint = %q", endpoint)
	}
	if err := authorizer.AuthorizeServiceAccess(context.Background(), "session", nil, nil, domain.PermView); err != auth.ErrAuthorizationAuthorityUnavailable {
		t.Fatalf("unconfigured authority error = %v", err)
	}
}

func TestServiceAuthorizerFromEnvironmentUsesRemoteAuthority(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+testAuthorityBridgeToken {
			t.Fatal("bridge token missing")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"decision": "allow", "account_id": "account-1", "session_id": "session-1", "device_id": "device-1",
		})
	}))
	defer server.Close()

	t.Setenv("WEBGATE_AUTHORITY_URL", server.URL)
	t.Setenv("WEBGATE_AUTHORITY_TOKEN", testAuthorityBridgeToken)
	authorizer, endpoint, err := serviceAuthorizerFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	if endpoint != server.URL {
		t.Fatalf("authority endpoint = %q, want %q", endpoint, server.URL)
	}
	device := &domain.Device{ID: "device-1", AccountID: "account-1", Status: domain.DeviceStatusActive}
	service := &domain.ProtectedService{ID: "service-1", TenantID: "tenant-1", WorkspaceID: "workspace-1"}
	if err := authorizer.AuthorizeServiceAccess(context.Background(), "session-token", device, service, domain.PermView); err != nil {
		t.Fatalf("remote authorization failed: %v", err)
	}
}

func TestServiceAuthorizerFromEnvironmentRejectsPartialOrUnsafeConfiguration(t *testing.T) {
	t.Setenv("WEBGATE_AUTHORITY_URL", "http://127.0.0.1:8790")
	t.Setenv("WEBGATE_AUTHORITY_TOKEN", "")
	if _, _, err := serviceAuthorizerFromEnvironment(); err == nil {
		t.Fatal("explicit authority URL without token unexpectedly accepted")
	}

	t.Setenv("WEBGATE_AUTHORITY_URL", "http://192.168.1.10:8790")
	t.Setenv("WEBGATE_AUTHORITY_TOKEN", testAuthorityBridgeToken)
	if _, _, err := serviceAuthorizerFromEnvironment(); err == nil {
		t.Fatal("non-loopback authority unexpectedly accepted")
	}
}
