package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/domain"
)

func TestLegacyMemoryAuthorizationCompatibility(t *testing.T) {
	authorizer := auth.NewSecureAccessAuthorizer()

	authorizer.RegisterSession(&auth.UserSession{
		SessionID: "sess_1",
		UserID:    "user_1",
		DeviceID:  "dev_1",
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	})

	device := &domain.Device{ID: "dev_1", UserID: "user_1", Status: domain.DeviceStatusActive}
	svc := &domain.ProtectedService{
		ID:          "svc_factory",
		WorkspaceID: "ws_factory",
		Slug:        "factory",
	}

	err := authorizer.AuthorizeServiceAccess(context.Background(), "sess_1", device, svc, domain.PermView)
	if err != auth.ErrAccessDenied {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}

	authorizer.SetMembership("user_1", "ws_factory", domain.PermView)

	err = authorizer.AuthorizeServiceAccess(context.Background(), "sess_1", device, svc, domain.PermView)
	if err != nil {
		t.Fatalf("expected authorization success, got: %v", err)
	}

	err = authorizer.AuthorizeServiceAccess(context.Background(), "sess_1", device, svc, domain.PermEdit)
	if err != auth.ErrAccessDenied {
		t.Fatalf("expected ErrAccessDenied for PermEdit, got %v", err)
	}

	otherDevice := &domain.Device{ID: "dev_2", UserID: "user_1", Status: domain.DeviceStatusActive}
	err = authorizer.AuthorizeServiceAccess(context.Background(), "sess_1", otherDevice, svc, domain.PermView)
	if err != auth.ErrSessionDeviceMismatch {
		t.Fatalf("expected ErrSessionDeviceMismatch, got %v", err)
	}

	foreignDevice := &domain.Device{ID: "dev_1", UserID: "user_2", Status: domain.DeviceStatusActive}
	err = authorizer.AuthorizeServiceAccess(context.Background(), "sess_1", foreignDevice, svc, domain.PermView)
	if err != auth.ErrSessionUserDeviceMismatch {
		t.Fatalf("expected ErrSessionUserDeviceMismatch, got %v", err)
	}

	device.Status = domain.DeviceStatusRevoked
	err = authorizer.AuthorizeServiceAccess(context.Background(), "sess_1", device, svc, domain.PermView)
	if err != auth.ErrDeviceRevokedOrInactive {
		t.Fatalf("expected ErrDeviceRevokedOrInactive, got %v", err)
	}
	device.Status = domain.DeviceStatusActive

	authorizer.RevokeSession("sess_1")
	err = authorizer.AuthorizeServiceAccess(context.Background(), "sess_1", device, svc, domain.PermView)
	if err != auth.ErrSessionExpired {
		t.Fatalf("expected ErrSessionExpired, got %v", err)
	}
}
