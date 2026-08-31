package delivery

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

var (
	ErrTelegramUserNotBound = errors.New("user has no verified Telegram chat ID bound")
	ErrDuplicateDelivery    = errors.New("delivery request with this idempotency key already processed")
)

type DeliveryReceipt struct {
	DeliveryID   string                   `json:"delivery_id"`
	UserID       string                   `json:"user_id"`
	TelegramChat int64                    `json:"telegram_chat_id"`
	Artifact     *domain.PlatformArtifact `json:"artifact"`
	Version      string                   `json:"version"`
	DispatchedAt time.Time                `json:"dispatched_at"`
	Method       string                   `json:"method"` // "DIRECT_FILE" or "PROTECTED_LINK"
}

type TelegramDeliveryService struct {
	mu           sync.Mutex
	releaseReg   *registry.ReleaseRegistry
	userBindings map[string]int64 // userID -> telegramChatID
	receipts     map[string]*DeliveryReceipt
}

func NewTelegramDeliveryService(releaseReg *registry.ReleaseRegistry) *TelegramDeliveryService {
	return &TelegramDeliveryService{
		releaseReg:   releaseReg,
		userBindings: make(map[string]int64),
		receipts:     make(map[string]*DeliveryReceipt),
	}
}

func (s *TelegramDeliveryService) BindUserTelegram(userID string, chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userBindings[userID] = chatID
}

// SendLatestWebGate resolves target user and device, checks latest promoted release, and dispatches installer package.
func (s *TelegramDeliveryService) SendLatestWebGate(
	idempotencyKey string,
	userID string,
	platform domain.DevicePlatform,
	arch domain.DeviceArchitecture,
) (*DeliveryReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if receipt, exists := s.receipts[idempotencyKey]; exists {
		return receipt, nil
	}

	chatID, bound := s.userBindings[userID]
	if !bound || chatID == 0 {
		return nil, ErrTelegramUserNotBound
	}

	rel, artifact, err := s.releaseReg.GetLatestPromoted(platform, arch)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve compatible release: %w", err)
	}

	method := "DIRECT_FILE"
	if artifact.SizeBytes > 50*1024*1024 { // Telegram 50MB direct bot limit
		method = "PROTECTED_LINK"
	}

	receipt := &DeliveryReceipt{
		DeliveryID:   fmt.Sprintf("dlv_%d", time.Now().UnixNano()),
		UserID:       userID,
		TelegramChat: chatID,
		Artifact:     artifact,
		Version:      rel.Version,
		DispatchedAt: time.Now().UTC(),
		Method:       method,
	}

	s.receipts[idempotencyKey] = receipt
	return receipt, nil
}
