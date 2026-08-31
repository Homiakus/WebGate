package registry

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
)

var (
	ErrDeviceNotFound          = errors.New("device not found")
	ErrDeviceAlreadyExists     = errors.New("device with this ID already registered")
	ErrChallengeExpired        = errors.New("device challenge has expired")
	ErrInvalidSignature        = errors.New("proof of possession signature invalid")
	ErrInvalidDeviceIdentity   = errors.New("device ID and user ID are required")
	ErrInvalidPublicKey        = errors.New("device public key is invalid")
	ErrUnsupportedKeyAlgorithm = errors.New("device key algorithm is not supported")
	ErrActivationRequiresProof = errors.New("device activation requires cryptographic proof of possession")
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

// Enroll registers a new device in PENDING status after validating the public
// identity material required for proof-of-possession.
func (r *DeviceRegistry) Enroll(dev *domain.Device) error {
	if dev == nil || strings.TrimSpace(dev.ID) == "" || strings.TrimSpace(dev.UserID) == "" {
		return ErrInvalidDeviceIdentity
	}
	if err := validateDevicePublicKey(dev); err != nil {
		return err
	}

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

// CreateChallenge generates a time-bounded, single-use challenge for device proof-of-possession.
func (r *DeviceRegistry) CreateChallenge(deviceID string, ttl time.Duration) (*domain.DeviceChallenge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ttl <= 0 {
		return nil, ErrChallengeExpired
	}
	dev, exists := r.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}

	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	challengeIDBytes := make([]byte, 16)
	if _, err := rand.Read(challengeIDBytes); err != nil {
		return nil, err
	}

	challenge := &domain.DeviceChallenge{
		ChallengeID: hex.EncodeToString(challengeIDBytes),
		DeviceID:    deviceID,
		NonceHex:    hex.EncodeToString(nonce),
		ExpiresAt:   time.Now().UTC().Add(ttl),
	}
	payload, err := buildChallengeSigningPayload(challenge, dev)
	if err != nil {
		return nil, err
	}
	challenge.SigningPayload = string(payload)

	r.challenges[challenge.ChallengeID] = challenge
	return challenge, nil
}

// ChallengeSigningPayload returns the canonical WebGate v1 proof-of-possession
// message. Clients must sign these exact bytes with the enrolled private key.
func ChallengeSigningPayload(chal *domain.DeviceChallenge, dev *domain.Device) ([]byte, error) {
	if chal == nil || dev == nil || chal.DeviceID != dev.ID {
		return nil, ErrInvalidSignature
	}
	if chal.SigningPayload != "" {
		return []byte(chal.SigningPayload), nil
	}
	return buildChallengeSigningPayload(chal, dev)
}

func buildChallengeSigningPayload(chal *domain.DeviceChallenge, dev *domain.Device) ([]byte, error) {
	pub, err := decodeEd25519PublicKey(dev)
	if err != nil {
		return nil, err
	}
	fingerprint := sha256.Sum256(pub)
	payload := fmt.Sprintf(
		"webgate-device-pop-v1\nchallenge_id=%s\ndevice_id=%s\nnonce=%s\nexpires_at=%d\nalgorithm=Ed25519\npublic_key_sha256=%x\n",
		chal.ChallengeID,
		chal.DeviceID,
		chal.NonceHex,
		chal.ExpiresAt.UTC().Unix(),
		fingerprint,
	)
	return []byte(payload), nil
}

// VerifyAndActivate verifies the challenge response and activates the device.
// Challenges are consumed before verification so failed attempts cannot be replayed.
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
	pub, err := decodeEd25519PublicKey(dev)
	if err != nil {
		return err
	}
	signature, err := hex.DecodeString(strings.TrimSpace(signatureHex))
	if err != nil || len(signature) != ed25519.SignatureSize {
		return ErrInvalidSignature
	}
	payload, err := ChallengeSigningPayload(chal, dev)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, payload, signature) {
		return ErrInvalidSignature
	}

	dev.Status = domain.DeviceStatusActive
	dev.LastSeenAt = time.Now().UTC()
	return nil
}

func validateDevicePublicKey(dev *domain.Device) error {
	_, err := decodeEd25519PublicKey(dev)
	return err
}

func decodeEd25519PublicKey(dev *domain.Device) (ed25519.PublicKey, error) {
	if !strings.EqualFold(strings.TrimSpace(dev.Algorithm), "Ed25519") {
		return nil, ErrUnsupportedKeyAlgorithm
	}
	pub, err := hex.DecodeString(strings.TrimSpace(dev.PublicKeyHex))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, ErrInvalidPublicKey
	}
	return ed25519.PublicKey(pub), nil
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

// UpdateStatus updates administrative lifecycle states. PENDING -> ACTIVE is
// intentionally forbidden here; activation must pass VerifyAndActivate.
func (r *DeviceRegistry) UpdateStatus(id string, status domain.DeviceStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	dev, ok := r.devices[id]
	if !ok {
		return ErrDeviceNotFound
	}
	if status == domain.DeviceStatusActive && dev.Status != domain.DeviceStatusActive {
		return ErrActivationRequiresProof
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
