package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

var (
	ErrUnsafeListenerAddress = errors.New("listener must bind to a loopback address")
	ErrListenerCollision     = errors.New("data-plane and admin listeners must use different addresses")
)

// HardenRuntimeAddresses upgrades legacy prototype defaults to loopback-only
// listeners and validates that the data and admin planes cannot accidentally be
// exposed on wildcard/public interfaces.
func HardenRuntimeAddresses(cfg *ServerConfig) error {
	if cfg == nil {
		return errors.New("server config is nil")
	}

	// Legacy defaults used ":8787" for the combined listener and reused 8787 for
	// AdminAddr. Convert only that known combined topology into the hardened split.
	rawData := strings.TrimSpace(cfg.ListenAddr)
	rawAdmin := strings.TrimSpace(cfg.AdminAddr)
	legacyData := rawData == "" || rawData == ":8787"
	if legacyData {
		cfg.ListenAddr = "127.0.0.1:8787"
	}
	if rawAdmin == "" || (rawAdmin == "127.0.0.1:8787" && (legacyData || rawData == "127.0.0.1:8787")) {
		cfg.AdminAddr = "127.0.0.1:8788"
	}

	dataAddr, err := normalizeLoopbackAddress(cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("invalid data-plane listen_addr %q: %w", cfg.ListenAddr, err)
	}
	adminAddr, err := normalizeLoopbackAddress(cfg.AdminAddr)
	if err != nil {
		return fmt.Errorf("invalid admin_addr %q: %w", cfg.AdminAddr, err)
	}
	if dataAddr == adminAddr {
		return ErrListenerCollision
	}

	cfg.ListenAddr = dataAddr
	cfg.AdminAddr = adminAddr
	return nil
}

func normalizeLoopbackAddress(raw string) (string, error) {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}

	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || port == 0 {
		return "", fmt.Errorf("invalid port %q", portText)
	}

	if strings.EqualFold(host, "localhost") {
		host = "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", ErrUnsafeListenerAddress
	}

	return net.JoinHostPort(ip.String(), strconv.FormatUint(port, 10)), nil
}
