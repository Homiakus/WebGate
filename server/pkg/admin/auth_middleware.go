package admin

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
)

const minAdminTokenBytes = 32

var ErrAdminTokenTooShort = errors.New("admin token must contain at least 32 bytes")

// RequireAdminToken wraps the admin control-plane with a bootstrap authentication
// boundary. The token is deliberately supplied out-of-band (normally through the
// WEBGATE_ADMIN_TOKEN environment variable) and is never stored in ServerConfig.
//
// This is the P0 bootstrap guard. SecureAcces-backed administrator authorization
// remains the target control-plane authority, but no unauthenticated admin surface
// is allowed while that adapter is being implemented.
func RequireAdminToken(next http.Handler, token string) (http.Handler, error) {
	if len(token) < minAdminTokenBytes {
		return nil, ErrAdminTokenTooShort
	}

	expected := sha256.Sum256([]byte(token))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		candidate := adminCredential(r)
		candidateHash := sha256.Sum256([]byte(candidate))
		if candidate == "" || subtle.ConstantTimeCompare(expected[:], candidateHash[:]) != 1 {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("WWW-Authenticate", `Basic realm="WebGate Admin", charset="UTF-8"`)
			http.Error(w, "administrator authentication required", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	}), nil
}

func adminCredential(r *http.Request) string {
	if token := strings.TrimSpace(r.Header.Get("X-WebGate-Admin-Token")); token != "" {
		return token
	}

	if authorization := strings.TrimSpace(r.Header.Get("Authorization")); authorization != "" {
		const bearerPrefix = "Bearer "
		if len(authorization) > len(bearerPrefix) && strings.EqualFold(authorization[:len(bearerPrefix)], bearerPrefix) {
			return strings.TrimSpace(authorization[len(bearerPrefix):])
		}
	}

	if username, password, ok := r.BasicAuth(); ok && username == "webgate-admin" {
		return password
	}

	return ""
}
