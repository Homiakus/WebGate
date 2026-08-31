package delivery_test

import (
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/delivery"
	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func TestTelegramDelivery(t *testing.T) {
	relReg := registry.NewReleaseRegistry()

	// Add draft, verify, promote
	rel := &domain.Release{
		Version:      "v1.0.0",
		SourceCommit: "abc1234",
		Artifacts: []domain.PlatformArtifact{
			{
				Platform:     domain.PlatformWindows,
				Architecture: domain.ArchX86_64,
				FileName:     "webgate-installer.exe",
				SHA256Hex:    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				SizeBytes:    15 * 1024 * 1024,
				DownloadURL:  "https://releases.webgate.corp/v1.0.0/webgate-installer.exe",
			},
		},
	}
	_ = relReg.AddDraft(rel)
	_ = relReg.Verify("v1.0.0")
	_ = relReg.Promote("v1.0.0")

	deliverySvc := delivery.NewTelegramDeliveryService(relReg)

	// User without telegram binding -> fail
	_, err := deliverySvc.SendLatestWebGate("idem_1", "user_bob", domain.PlatformWindows, domain.ArchX86_64)
	if err != delivery.ErrTelegramUserNotBound {
		t.Fatalf("expected ErrTelegramUserNotBound, got %v", err)
	}

	// Bind telegram
	deliverySvc.BindUserTelegram("user_bob", 987654321)

	// Dispatch delivery
	receipt, err := deliverySvc.SendLatestWebGate("idem_1", "user_bob", domain.PlatformWindows, domain.ArchX86_64)
	if err != nil {
		t.Fatalf("unexpected delivery error: %v", err)
	}

	if receipt.Version != "v1.0.0" || receipt.TelegramChat != 987654321 || receipt.Method != "DIRECT_FILE" {
		t.Fatalf("unexpected receipt content: %+v", receipt)
	}

	// Test idempotency
	receipt2, _ := deliverySvc.SendLatestWebGate("idem_1", "user_bob", domain.PlatformWindows, domain.ArchX86_64)
	if receipt2.DeliveryID != receipt.DeliveryID {
		t.Fatalf("idempotent delivery ID changed")
	}
}
