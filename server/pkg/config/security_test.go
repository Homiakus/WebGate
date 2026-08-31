package config_test

import (
	"errors"
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/config"
)

func TestHardenRuntimeAddressesUpgradesLegacyDefaults(t *testing.T) {
	cfg := config.DefaultServerConfig()
	if err := config.HardenRuntimeAddresses(cfg); err != nil {
		t.Fatalf("hardening failed: %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:8787" {
		t.Fatalf("unexpected data-plane address: %s", cfg.ListenAddr)
	}
	if cfg.AdminAddr != "127.0.0.1:8788" {
		t.Fatalf("unexpected admin address: %s", cfg.AdminAddr)
	}
}

func TestHardenRuntimeAddressesRejectsWildcardAdmin(t *testing.T) {
	cfg := config.DefaultServerConfig()
	cfg.ListenAddr = "127.0.0.1:8787"
	cfg.AdminAddr = "0.0.0.0:8788"
	if err := config.HardenRuntimeAddresses(cfg); !errors.Is(err, config.ErrUnsafeListenerAddress) {
		t.Fatalf("expected ErrUnsafeListenerAddress, got %v", err)
	}
}

func TestHardenRuntimeAddressesRejectsCollision(t *testing.T) {
	cfg := config.DefaultServerConfig()
	cfg.ListenAddr = "127.0.0.1:9000"
	cfg.AdminAddr = "127.0.0.1:9000"
	if err := config.HardenRuntimeAddresses(cfg); !errors.Is(err, config.ErrListenerCollision) {
		t.Fatalf("expected ErrListenerCollision, got %v", err)
	}
}
