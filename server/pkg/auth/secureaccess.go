package auth

import (
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

// SecureAccessAuthorizer evaluates authorization policies using SecureAcces primitives.
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

// AuthorizeServiceAccess validates session, presented device identity/status,
// user-to-device ownership, and workspace permission for a service action.
func (a *SecureAccessAuthorizer) AuthorizeServiceAccess(
	sessionID string,
	device *domain.Device,
	service *domain.ProtectedService,
	requiredPerm domain.PermissionBits,
) (*UserSession, error) {
	if device == nil || !device.Status.IsAllowedAccess() {
		return nil, ErrDeviceRevokedOrInactive
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	sess, ok := a.sessions[sessionID]
	if !ok || time.Now().UTC().After(sess.ExpiresAt) {
		return nil, ErrSessionExpired
	}
	if sess.DeviceID == "" || sess.DeviceID != device.ID {
		return nil, ErrSessionDeviceMismatch
	}
	if sess.UserID == "" || sess.UserID != device.UserID {
		return nil, ErrSessionUserDeviceMismatch
	}

	key := sess.UserID + ":" + service.WorkspaceID
	perms, ok := a.memberships[key]
	if !ok || !perms.Has(requiredPerm) {
		return nil, ErrAccessDenied
	}

	return sess, nil
}
