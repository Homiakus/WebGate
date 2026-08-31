package domain

import "time"

type DeviceStatus string

const (
	DeviceStatusPending   DeviceStatus = "PENDING"
	DeviceStatusActive    DeviceStatus = "ACTIVE"
	DeviceStatusSuspended DeviceStatus = "SUSPENDED"
	DeviceStatusRevoked   DeviceStatus = "REVOKED"
)

func (s DeviceStatus) IsAllowedAccess() bool {
	return s == DeviceStatusActive
}

type DevicePlatform string

const (
	PlatformWindows DevicePlatform = "windows"
	PlatformAndroid DevicePlatform = "android"
	PlatformLinux   DevicePlatform = "linux"
	PlatformMacOS   DevicePlatform = "macos"
)

type DeviceArchitecture string

const (
	ArchX86_64 DeviceArchitecture = "x86_64"
	ArchArm64  DeviceArchitecture = "arm64"
)

// Device represents an enrolled or enrolling client workstation or mobile device.
// AccountID is the authoritative global SecureAcces account binding used by the
// production authorization path. UserID is retained temporarily as legacy
// tenant-local metadata for unrequalified admin/tests and MUST NOT be promoted
// to AccountID implicitly.
type Device struct {
	ID           string             `json:"id"`
	AccountID    string             `json:"account_id,omitempty"`
	UserID       string             `json:"user_id,omitempty"`
	PublicKeyHex string             `json:"public_key_hex"`
	Algorithm    string             `json:"algorithm"`
	Platform     DevicePlatform     `json:"platform"`
	Architecture DeviceArchitecture `json:"architecture"`
	Label        string             `json:"label"`
	Status       DeviceStatus       `json:"status"`
	CreatedAt    time.Time          `json:"created_at"`
	LastSeenAt   time.Time          `json:"last_seen_at"`
}

// DeviceChallenge is an ephemeral, single-use proof-of-possession challenge.
// SigningPayload is the exact byte string (UTF-8) a client must sign.
type DeviceChallenge struct {
	ChallengeID    string    `json:"challenge_id"`
	DeviceID       string    `json:"device_id"`
	NonceHex       string    `json:"nonce_hex"`
	SigningPayload string    `json:"signing_payload"`
	ExpiresAt      time.Time `json:"expires_at"`
}
