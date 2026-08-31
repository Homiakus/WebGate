package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/domain"
)

func TestUnavailableServiceAuthorizerFailsClosed(t *testing.T) {
	authorizer := auth.NewUnavailableServiceAuthorizer()
	err := authorizer.AuthorizeServiceAccess(
		context.Background(),
		"token",
		&domain.Device{ID: "dev", Status: domain.DeviceStatusActive},
		&domain.ProtectedService{ID: "svc", WorkspaceID: "ws"},
		domain.PermView,
	)
	if !errors.Is(err, auth.ErrAuthorizationAuthorityUnavailable) {
		t.Fatalf("expected unavailable authority error, got %v", err)
	}
}

func TestNilServiceAuthorizerFuncFailsClosed(t *testing.T) {
	var authorizer auth.ServiceAuthorizerFunc
	if !errors.Is(authorizer.AuthorizeServiceAccess(context.Background(), "", nil, nil, domain.PermView), auth.ErrAuthorizationAuthorityUnavailable) {
		t.Fatal("nil functional authorizer must fail closed")
	}
}
