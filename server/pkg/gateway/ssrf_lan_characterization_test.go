package gateway

import (
	"errors"
	"testing"
)

func TestBuildSafeUpstreamURLRejectsUnownedPrivateLANPivots(t *testing.T) {
	for _, raw := range []string{
		"http://10.0.0.1:8080/",
		"http://172.16.0.1:8080/",
		"http://192.168.1.1:8080/",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := buildSafeUpstreamURL(raw, "/", ""); !errors.Is(err, ErrSSRFAttemptBlocked) {
				t.Fatalf("expected unowned private LAN destination %q to be rejected, got %v", raw, err)
			}
		})
	}
}
