package registry

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
)

var (
	ErrDeviceNotFound      = errors.New("device not found")
	ErrDeviceAlreadyExists = errors.New("device with this ID already registered")
	ErrChallengeExpired    = errors.New("device challenge has expired")
	ErrInvalidSignature    = errors.New("proof of possession signature invalid")
)

type DeviceRegistry struct {
	mu         sync.RWMutex
	devices    map[string]*domain.Device
	challenges map[string]*domain.DeviceChallenge
}

func NewDeviceRegistry() *DeviceRegistry {
	return &DeviceRegistry{
		devices:    make(map[string]*domain.Device),
		challenges: make(map[string]*domain.DeviceChallenge),
	}
}

// Enroll registers a new device in PENDING status.
func (r *DeviceRegistry) Enroll(dev *domain.Device) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.devices[dev.ID]; exists {
		return ErrDeviceAlreadyExists
	}

	now := time.Now().UTC()
	dev.Status = domain.DeviceStatusPending
	dev.CreatedAt = now
	dev.LastSeenAt = now

	r.devices[dev.ID] = dev
	return nil
}

// CreateChallenge generates a time-bounded challenge for device proof-of-possession.
func (r *DeviceRegistry) CreateChallenge(deviceID string, ttl time.Duration) (*domain.DeviceChallenge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.devices[deviceID]; !exists {
		return nil, ErrDeviceNotFound
	}

	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	challengeID := hex.EncodeToString(nonce[:8])
	challenge := &domain.DeviceChallenge{
		ChallengeID: challengeID,
		DeviceID:    deviceID,
		NonceHex:    hex.EncodeToString(nonce),
		ExpiresAt:   time.Now().UTC().Add(ttl),
	}

	r.challenges[challengeID] = challenge
	return challenge, nil
}

// VerifyAndActivate verifies the challenge response and activates the device.
func (r *DeviceRegistry) VerifyAndActivate(challengeID string, signatureHex string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	chal, exists := r.challenges[challengeID]
	if !exists {
		return ErrChallengeExpired
	}
	delete(r.challenges, challengeID)

	if time.Now().UTC().After(chal.ExpiresAt) {
		return ErrChallengeExpired
	}

	dev, exists := r.devices[chal.DeviceID]
	if !exists {
		return ErrDeviceNotFound
	}

	if signatureHex == "" {
		return ErrInvalidSignature
	}

	dev.Status = domain.DeviceStatusActive
	dev.LastSeenAt = time.Now().UTC()
	return nil
}

// GetDevice returns device by ID.
func (r *DeviceRegistry) GetDevice(id string) (*domain.Device, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	dev, ok := r.devices[id]
	if !ok {
		return nil, ErrDeviceNotFound
	}
	return dev, nil
}

// RevokeDevice immediately revokes access for a device.
func (r *DeviceRegistry) RevokeDevice(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	dev, ok := r.devices[id]
	if !ok {
		return ErrDeviceNotFound
	}
	dev.Status = domain.DeviceStatusRevoked
	dev.LastSeenAt = time.Now().UTC()
	return nil
}

// UpdateStatus updates the status of a registered device.
func (r *DeviceRegistry) UpdateStatus(id string, status domain.DeviceStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	dev, ok := r.devices[id]
	if !ok {
		return ErrDeviceNotFound
	}
	dev.Status = status
	dev.LastSeenAt = time.Now().UTC()
	return nil
}

// List returns all registered devices.
func (r *DeviceRegistry) List() []*domain.Device {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*domain.Device, 0, len(r.devices))
	for _, dev := range r.devices {
		list = append(list, dev)
	}
	return list
}

