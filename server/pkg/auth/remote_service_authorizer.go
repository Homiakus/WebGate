package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Homiakus/WebGate/server/pkg/domain"
)

const (
	remoteAuthoritySchema       = "webgate.secureaccess.authorize/v1"
	defaultAuthorityTimeout     = 2 * time.Second
	maxAuthorityTimeout         = 10 * time.Second
	maxAuthorityResponseBytes   = 8 << 10
	maxAuthorityBridgeTokenSize = 512
)

type RemoteServiceAuthorizerConfig struct {
	Endpoint    string
	BridgeToken string
	Timeout     time.Duration
}

type RemoteServiceAuthorizer struct {
	authorizeURL string
	bridgeToken string
	httpClient  *http.Client
}

type remoteAuthorizeRequest struct {
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

type remoteAuthorizeResponse struct {
	Decision  string `json:"decision"`
	AccountID string `json:"account_id"`
	SessionID string `json:"session_id"`
	DeviceID  string `json:"device_id"`
}

func NewRemoteServiceAuthorizer(config RemoteServiceAuthorizerConfig) (ServiceAuthorizer, error) {
	endpoint, err := canonicalAuthorityEndpoint(config.Endpoint)
	if err != nil {
		return nil, err
	}
	if err := validateAuthorityBridgeToken(config.BridgeToken); err != nil {
		return nil, err
	}
	timeout := config.Timeout
	if timeout == 0 {
		timeout = defaultAuthorityTimeout
	}
	if timeout < 0 || timeout > maxAuthorityTimeout {
		return nil, fmt.Errorf("remote authority timeout must be in (0,%s]", maxAuthorityTimeout)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil

	return &RemoteServiceAuthorizer{
		authorizeURL: strings.TrimSuffix(endpoint, "/") + "/v1/authorize",
		bridgeToken: config.BridgeToken,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func canonicalAuthorityEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("remote authority endpoint is required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.Host == "" {
		return "", fmt.Errorf("remote authority endpoint is invalid")
	}
	if u.Scheme != "http" {
		return "", fmt.Errorf("remote authority endpoint must use HTTP on literal loopback")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/") {
		return "", fmt.Errorf("remote authority endpoint cannot contain credentials, path, query, or fragment")
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", fmt.Errorf("remote authority host must be a literal loopback IP")
	}
	port := u.Port()
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		return "", fmt.Errorf("remote authority endpoint requires a valid port")
	}
	if strings.Contains(ip.String(), ":") {
		u.Host = net.JoinHostPort(ip.String(), port)
	} else {
		u.Host = ip.String() + ":" + port
	}
	u.Path = ""
	return u.String(), nil
}

func validateAuthorityBridgeToken(token string) error {
	if len(token) < 32 {
		return fmt.Errorf("remote authority bridge token must be at least 32 bytes")
	}
	if len(token) > maxAuthorityBridgeTokenSize {
		return fmt.Errorf("remote authority bridge token exceeds %d bytes", maxAuthorityBridgeTokenSize)
	}
	if strings.IndexFunc(token, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return fmt.Errorf("remote authority bridge token cannot contain whitespace/control characters")
	}
	return nil
}

func (a *RemoteServiceAuthorizer) AuthorizeServiceAccess(
	ctx context.Context,
	sessionToken string,
	device *domain.Device,
	service *domain.ProtectedService,
	requiredPerm domain.PermissionBits,
) error {
	if device == nil || !device.Status.IsAllowedAccess() || strings.TrimSpace(device.ID) == "" || strings.TrimSpace(device.AccountID) == "" || strings.TrimSpace(sessionToken) == "" {
		return ErrAccessDenied
	}
	if service == nil || strings.TrimSpace(service.ID) == "" || strings.TrimSpace(service.TenantID) == "" || strings.TrimSpace(service.WorkspaceID) == "" || requiredPerm == 0 {
		return ErrAuthorizationAuthorityUnavailable
	}

	payload, err := json.Marshal(remoteAuthorizeRequest{
		Schema:              remoteAuthoritySchema,
		SessionToken:        sessionToken,
		DeviceID:            device.ID,
		AccountID:           device.AccountID,
		TenantID:            service.TenantID,
		WorkspaceID:         service.WorkspaceID,
		ResourceKind:        "service",
		ResourceID:          service.ID,
		RequiredPermissions: uint64(requiredPerm),
	})
	if err != nil {
		return ErrAuthorizationAuthorityUnavailable
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.authorizeURL, bytes.NewReader(payload))
	if err != nil {
		return ErrAuthorizationAuthorityUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+a.bridgeToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return ErrAuthorizationAuthorityUnavailable
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return ErrAccessDenied
	}
	if resp.StatusCode != http.StatusOK {
		return ErrAuthorizationAuthorityUnavailable
	}

	response, err := decodeRemoteAuthorizeResponse(resp.Body)
	if err != nil {
		return ErrAuthorizationAuthorityUnavailable
	}
	if response.Decision != "allow" ||
		response.AccountID != device.AccountID ||
		response.DeviceID != device.ID ||
		strings.TrimSpace(response.SessionID) == "" {
		return ErrAuthorizationAuthorityUnavailable
	}
	return nil
}

func decodeRemoteAuthorizeResponse(body io.Reader) (remoteAuthorizeResponse, error) {
	limited := io.LimitReader(body, maxAuthorityResponseBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return remoteAuthorizeResponse{}, err
	}
	if len(data) > maxAuthorityResponseBytes {
		return remoteAuthorizeResponse{}, fmt.Errorf("authority response exceeds %d bytes", maxAuthorityResponseBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var response remoteAuthorizeResponse
	if err := decoder.Decode(&response); err != nil {
		return remoteAuthorizeResponse{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return remoteAuthorizeResponse{}, fmt.Errorf("multiple JSON values")
		}
		return remoteAuthorizeResponse{}, err
	}
	return response, nil
}

var _ ServiceAuthorizer = (*RemoteServiceAuthorizer)(nil)
