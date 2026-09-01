package config

import "strings"

const TelegramBotTokenEnvironment = "WEBGATE_TELEGRAM_BOT_TOKEN"

// ApplyRuntimeSecrets overlays secrets that must not live in the WebGate durable
// SQLite state. Environment values have the final precedence over file/durable
// metadata so operators can rotate credentials without rewriting state backups.
func ApplyRuntimeSecrets(cfg *ServerConfig, lookup func(string) (string, bool)) {
	if cfg == nil || lookup == nil {
		return
	}
	if token, ok := lookup(TelegramBotTokenEnvironment); ok {
		cfg.TelegramBotToken = strings.TrimSpace(token)
	}
}
