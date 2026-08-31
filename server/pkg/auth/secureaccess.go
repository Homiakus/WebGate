package auth

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
)

var (
	ErrSessionExpired            = errors.New("user session expired")
	ErrDeviceRevokedOrInactive   = errors.New("device is not active or has been revoked")
	ErrSessionDeviceMismatch     = errors.New("session is not bound to the presented device")
	ErrSessionUserDeviceMismatch = errors.New("session user does not own the presented device")
	ErrAccessDenied              = errors.New("insufficient workspace permissions for service")
)

type UserSession struct {
	SessionID string
	UserID    string
	DeviceID  string
	ExpiresAt time.Time
}

type WorkspaceMembership struct {
	UserID      string
	WorkspaceID string
	Permissions domain.PermissionBits
}

// SecureAccessAuthorizer is the historical WebGate-owned in-memory surrogate.
// It remains temporarily for tests and the not-yet-requalified admin prototype.
// Production data-plane wiring MUST use ServiceAuthorizer and MUST NOT construct
// this type. T-052 supplies the authoritative SecureAcces provider; T-051 removes
// the remaining admin-prototype dependency.
type SecureAccessAuthorizer struct {
	mu          sync.RWMutex
	sessions    map[string]*UserSession
	memberships map[string]domain.PermissionBits // key: "userID:workspaceID"
}

func NewSecureAccessAuthorizer() *SecureAccessAuthorizer {
	return &SecureAccessAuthorizer{
		sessions:    make(map[string]*UserSession),
		memberships: make(map[string]domain.PermissionBits),
	}
}

func (a *SecureAccessAuthorizer) RegisterSession(sess *UserSession) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions[sess.SessionID] = sess
}

func (a *SecureAccessAuthorizer) RevokeSession(sessionID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, sessionID)
}

func (a *SecureAccessAuthorizer) SetMembership(userID, workspaceID string, perms domain.PermissionBits) {
	a.mu.Lock()
	defer a.mu.Unlock()
	key := userID + ":" + workspaceID
	a.memberships[key] = perms
}

// AuthorizeServiceAccess is retained as compatibility-only behavior for tests.
// The production data plane no longer constructs this surrogate.
func (a *SecureAccessAuthorizer) AuthorizeServiceAccess(
	_ context.Context,
	sessionID string,
	device *domain.Device,
	service *domain.ProtectedService,
	requiredPerm domain.PermissionBits,
) error {
	if device == nil || !device.Status.IsAllowedAccess() {
		return ErrDeviceRevokedOrInactive
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	sess, ok := a.sessions[sessionID]
	if !ok || time.Now().UTC().After(sess.ExpiresAt) {
		return ErrSessionExpired
	}
	if sess.DeviceID == "" || sess.DeviceID != device.ID {
		return ErrSessionDeviceMismatch
	}
	if sess.UserID == "" || sess.UserID != device.UserID {
		return ErrSessionUserDeviceMismatch
	}

	key := sess.UserID + ":" + service.WorkspaceID
	perms, ok := a.memberships[key]
	if !ok || !perms.Has(requiredPerm) {
		return ErrAccessDenied
	}

	return nil
}

var _ ServiceAuthorizer = (*SecureAccessAuthorizer)(nil)
