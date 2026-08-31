package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/domain"
)

func TestRemoteServiceAuthorizerNetworkFailureIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	endpoint := server.URL
	server.Close()

	authorizer, err := auth.NewRemoteServiceAuthorizer(auth.RemoteServiceAuthorizerConfig{
		Endpoint:    endpoint,
		BridgeToken: testAuthorityToken,
		Timeout:     250 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := authorizer.AuthorizeServiceAccess(context.Background(), "session-secret", activeAccountDevice(), protectedService(), domain.PermView); err != auth.ErrAuthorizationAuthorityUnavailable {
		t.Fatalf("network failure error = %v, want ErrAuthorizationAuthorityUnavailable", err)
	}
}

func TestRemoteServiceAuthorizerDoesNotFollowRedirects(t *testing.T) {
	var redirectedRequests atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectedRequests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	authorizer := newRemoteAuthorizer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/capture", http.StatusFound)
	}))
	if err := authorizer.AuthorizeServiceAccess(context.Background(), "session-secret", activeAccountDevice(), protectedService(), domain.PermView); err != auth.ErrAuthorizationAuthorityUnavailable {
		t.Fatalf("redirect error = %v, want ErrAuthorizationAuthorityUnavailable", err)
	}
	if got := redirectedRequests.Load(); got != 0 {
		t.Fatalf("authority redirect target received %d requests", got)
	}
}

func TestRemoteServiceAuthorizerRejectsOversizedAndUnknownResponses(t *testing.T) {
	t.Run("oversized", func(t *testing.T) {
		authorizer := newRemoteAuthorizer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(strings.Repeat("x", 9<<10)))
		}))
		if err := authorizer.AuthorizeServiceAccess(context.Background(), "session-secret", activeAccountDevice(), protectedService(), domain.PermView); err != auth.ErrAuthorizationAuthorityUnavailable {
			t.Fatalf("oversized response error = %v", err)
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		authorizer := newRemoteAuthorizer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"decision": "allow", "account_id": "account-1", "session_id": "session-1", "device_id": "device-1", "unexpected": true,
			})
		}))
		if err := authorizer.AuthorizeServiceAccess(context.Background(), "session-secret", activeAccountDevice(), protectedService(), domain.PermView); err != auth.ErrAuthorizationAuthorityUnavailable {
			t.Fatalf("unknown-field response error = %v", err)
		}
	})
}

func TestRemoteServiceAuthorizerRejectsForceQueryEndpoint(t *testing.T) {
	if _, err := auth.NewRemoteServiceAuthorizer(auth.RemoteServiceAuthorizerConfig{
		Endpoint:    "http://127.0.0.1:8790?",
		BridgeToken: testAuthorityToken,
	}); err == nil {
		t.Fatal("authority endpoint with empty query marker unexpectedly accepted")
	}
}
