package auth

import (
	"testing"

	secureaccess "github.com/Homiakus/secureaccess/secureaccess"
)

func TestPinnedSecureAccessDependencyVersion(t *testing.T) {
	if secureaccess.Version != "0.4.0" {
		t.Fatalf("unexpected pinned SecureAccess version: %q", secureaccess.Version)
	}
}
