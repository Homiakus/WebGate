package auth

import (
	"context"
	"errors"

	"github.com/Homiakus/WebGate/server/pkg/domain"
)

var ErrAuthorizationAuthorityUnavailable = errors.New("authorization authority unavailable")

// ServiceAuthorizer is the narrow data-plane authorization boundary used by
// the WebGate gateway. Implementations own authentication/authorization
// decisions; the gateway owns service/resource resolution and device state.
//
// Production implementations must fail closed when their authoritative state
// cannot be reached or validated. T-052 supplies the real SecureAcces-backed
// provider; until then the production bootstrap uses UnavailableServiceAuthorizer.
type ServiceAuthorizer interface {
	AuthorizeServiceAccess(
		ctx context.Context,
		sessionToken string,
		device *domain.Device,
		service *domain.ProtectedService,
		requiredPerm domain.PermissionBits,
	) error
}

// ServiceAuthorizerFunc is a compact adapter for tests and narrow integrations.
type ServiceAuthorizerFunc func(
	ctx context.Context,
	sessionToken string,
	device *domain.Device,
	service *domain.ProtectedService,
	requiredPerm domain.PermissionBits,
) error

func (f ServiceAuthorizerFunc) AuthorizeServiceAccess(
	ctx context.Context,
	sessionToken string,
	device *domain.Device,
	service *domain.ProtectedService,
	requiredPerm domain.PermissionBits,
) error {
	if f == nil {
		return ErrAuthorizationAuthorityUnavailable
	}
	return f(ctx, sessionToken, device, service, requiredPerm)
}

type UnavailableServiceAuthorizer struct{}

func NewUnavailableServiceAuthorizer() ServiceAuthorizer {
	return UnavailableServiceAuthorizer{}
}

func (UnavailableServiceAuthorizer) AuthorizeServiceAccess(
	context.Context,
	string,
	*domain.Device,
	*domain.ProtectedService,
	domain.PermissionBits,
) error {
	return nil
}

var _ ServiceAuthorizer = UnavailableServiceAuthorizer{}
