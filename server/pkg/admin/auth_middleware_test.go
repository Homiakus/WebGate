package admin_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/admin"
)

const testAdminToken = "0123456789abcdef0123456789abcdef"

func TestRequireAdminToken(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler, err := admin.RequireAdminToken(inner, testAdminToken)
	if err != nil {
		t.Fatalf("failed to create admin auth middleware: %v", err)
	}

	t.Run("rejects missing credentials", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("accepts bearer token", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.Header.Set("Authorization", "Bearer "+testAdminToken)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rec.Code)
		}
	})

	t.Run("accepts browser basic auth", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.SetBasicAuth("webgate-admin", testAdminToken)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rec.Code)
		}
	})

	t.Run("rejects wrong token", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.Header.Set("X-WebGate-Admin-Token", "wrong-token")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

func TestRequireAdminTokenRejectsWeakSecret(t *testing.T) {
	_, err := admin.RequireAdminToken(http.NotFoundHandler(), "short")
	if !errorsIs(err, admin.ErrAdminTokenTooShort) {
		t.Fatalf("expected ErrAdminTokenTooShort, got %v", err)
	}
}

func errorsIs(err, target error) bool {
	return err == target
}
