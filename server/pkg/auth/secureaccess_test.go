package auth_test

import (
	"testing"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/domain"
)

func TestSecureAccessAuthorization(t *testing.T) {
	authorizer := auth.NewSecureAccessAuthorizer()

	authorizer.RegisterSession(&auth.UserSession{
		SessionID: "sess_1",
		UserID:    "user_1",
		DeviceID:  "dev_1",
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	})

	svc := &domain.ProtectedService{
		ID:          "svc_factory",
		WorkspaceID: "ws_factory",
		Slug:        "factory",
	}

	// 1. Without membership -> Access Denied
	_, err := authorizer.AuthorizeServiceAccess("sess_1", domain.DeviceStatusActive, svc, domain.PermView)
	if err != auth.ErrAccessDenied {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}

	// 2. Grant PermView only
	authorizer.SetMembership("user_1", "ws_factory", domain.PermView)

	sess, err := authorizer.AuthorizeServiceAccess("sess_1", domain.DeviceStatusActive, svc, domain.PermView)
	if err != nil || sess.UserID != "user_1" {
		t.Fatalf("expected authorization success, got: %v, %v", sess, err)
	}

	// Requesting PermEdit with only PermView -> Access Denied
	_, err = authorizer.AuthorizeServiceAccess("sess_1", domain.DeviceStatusActive, svc, domain.PermEdit)
	if err != auth.ErrAccessDenied {
		t.Fatalf("expected ErrAccessDenied for PermEdit, got %v", err)
	}

	// 3. Inactive/Revoked device -> Device Denied
	_, err = authorizer.AuthorizeServiceAccess("sess_1", domain.DeviceStatusRevoked, svc, domain.PermView)
	if err != auth.ErrDeviceRevokedOrInactive {
		t.Fatalf("expected ErrDeviceRevokedOrInactive, got %v", err)
	}

	// 4. Revoke session -> Session Expired
	authorizer.RevokeSession("sess_1")
	_, err = authorizer.AuthorizeServiceAccess("sess_1", domain.DeviceStatusActive, svc, domain.PermView)
	if err != auth.ErrSessionExpired {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}
