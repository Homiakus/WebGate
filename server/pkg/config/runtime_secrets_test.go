package config_test

import (
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/config"
)

func TestApplyRuntimeSecretsOverridesDurableOrFileToken(t *testing.T) {
	cfg := config.DefaultServerConfig()
	cfg.TelegramBotToken = "file-token"

	config.ApplyRuntimeSecrets(cfg, func(key string) (string, bool) {
		if key == config.TelegramBotTokenEnvironment {
			return "  environment-token  ", true
		}
		return "", false
	})
	if cfg.TelegramBotToken != "environment-token" {
		t.Fatalf("runtime secret did not win precedence: %q", cfg.TelegramBotToken)
	}
}

func TestApplyRuntimeSecretsLeavesConfigUntouchedWhenUnset(t *testing.T) {
	cfg := config.DefaultServerConfig()
	cfg.TelegramBotToken = "file-token"
	config.ApplyRuntimeSecrets(cfg, func(string) (string, bool) { return "", false })
	if cfg.TelegramBotToken != "file-token" {
		t.Fatalf("unset runtime secret unexpectedly changed config: %q", cfg.TelegramBotToken)
	}
}
