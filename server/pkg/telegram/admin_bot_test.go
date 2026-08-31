package telegram_test

import (
	"strings"
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/process"
	"github.com/Homiakus/WebGate/server/pkg/registry"
	"github.com/Homiakus/WebGate/server/pkg/telegram"
)

func TestAdminBotCommands(t *testing.T) {
	svcReg := registry.NewServiceRegistry()
	svc := &domain.ProtectedService{
		ID:             "svc_tg_node",
		WorkspaceID:    "ws_default",
		Name:           "Telegram Controlled Service",
		Slug:           "tgnode",
		Port:           8095,
		ExecutablePath: ":mock",
		Status:         domain.ServiceStatusActive,
	}
	_ = svcReg.Register(svc)

	pm := process.NewProcessManager(svcReg)
	bot := telegram.NewAdminBot(svcReg, pm)
	bot.AuthorizeAdmin(999001)

	// 1. /help
	resp := bot.HandleCommand(999001, "/help")
	if !strings.Contains(resp.Text, "WEBGATE CONTROL PLANE") {
		t.Fatalf("unexpected help response: %s", resp.Text)
	}

	// 2. /services
	resp = bot.HandleCommand(999001, "/services")
	if !strings.Contains(resp.Text, "tgnode") {
		t.Fatalf("expected tgnode in services response: %s", resp.Text)
	}

	// 3. /ports
	resp = bot.HandleCommand(999001, "/ports")
	if !strings.Contains(resp.Text, "8095") {
		t.Fatalf("expected port 8095 in ports response: %s", resp.Text)
	}

	// 4. /start_service tgnode
	resp = bot.HandleCommand(999001, "/start_service tgnode")
	if !strings.Contains(resp.Text, "успешно запущен") {
		t.Fatalf("failed to start service via bot: %s", resp.Text)
	}

	// 5. Callback query stop
	resp = bot.HandleCallbackQuery(999001, "stop:"+svc.ID)
	if !strings.Contains(resp.Text, "остановлен") {
		t.Fatalf("failed to stop service via callback: %s", resp.Text)
	}

	// 6. /bind command
	resp = bot.HandleCommand(999001, "/bind tgnode 8099 ./bin/new-node.exe")
	if !strings.Contains(resp.Text, "8099") || !strings.Contains(resp.Text, "привязаны") {
		t.Fatalf("bind command failed: %s", resp.Text)
	}
	if svc.Port != 8099 {
		t.Fatalf("expected port updated to 8099, got %d", svc.Port)
	}

	// 7. Unauthorized user rejected
	unauthResp := bot.HandleCommand(12345, "/services")
	if !strings.Contains(unauthResp.Text, "Access Denied") {
		t.Fatalf("expected access denied for unauthorized chat ID")
	}
}
