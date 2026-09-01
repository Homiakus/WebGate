package config_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/config"
)

func TestDurableServerConfigExcludesSecretsAndServiceRegistry(t *testing.T) {
	cfg := config.DefaultServerConfig()
	cfg.ServerName = "durable-node"
	cfg.TelegramBotToken = "TOP-SECRET-TELEGRAM-TOKEN"
	cfg.Services[0].ExecutablePath = "secret-looking-service-path"
	cfg.TelegramAdminChatIDs = []int64{10, 20}

	snapshot := config.DurableSnapshot(cfg)
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(cfg.TelegramBotToken)) {
		t.Fatalf("durable config leaked Telegram bot token: %s", raw)
	}
	if bytes.Contains(raw, []byte("telegram_bot_token")) {
		t.Fatalf("durable config contains secret-bearing field: %s", raw)
	}
	if bytes.Contains(raw, []byte("secret-looking-service-path")) || bytes.Contains(raw, []byte("services")) {
		t.Fatalf("durable control config duplicated service-registry state: %s", raw)
	}

	base := config.DefaultServerConfig()
	base.TelegramBotToken = "runtime-only-token"
	base.Services[0].Name = "registry-owned-service"
	config.ApplyDurableSnapshot(base, snapshot)

	if base.ServerName != "durable-node" {
		t.Fatalf("durable metadata was not restored: %q", base.ServerName)
	}
	if base.TelegramBotToken != "runtime-only-token" {
		t.Fatal("durable metadata overwrote runtime-only secret")
	}
	if base.Services[0].Name != "registry-owned-service" {
		t.Fatal("durable metadata overwrote registry-owned service definitions")
	}

	snapshot.TelegramAdminChatIDs[0] = 999
	if base.TelegramAdminChatIDs[0] == 999 {
		t.Fatal("durable config slices alias caller-owned memory")
	}
}
