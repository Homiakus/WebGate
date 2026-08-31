package gateway_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func enrollAndActivateTestDevice(t *testing.T, devReg *registry.DeviceRegistry, deviceID, userID string) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate device key: %v", err)
	}
	dev := &domain.Device{
		ID:           deviceID,
		UserID:       userID,
		PublicKeyHex: hex.EncodeToString(publicKey),
		Algorithm:    "Ed25519",
	}
	if err := devReg.Enroll(dev); err != nil {
		t.Fatalf("enroll device: %v", err)
	}
	chal, err := devReg.CreateChallenge(deviceID, time.Minute)
	if err != nil {
		t.Fatalf("create challenge: %v", err)
	}
	payload, err := registry.ChallengeSigningPayload(chal, dev)
	if err != nil {
		t.Fatalf("build challenge payload: %v", err)
	}
	sig := ed25519.Sign(privateKey, payload)
	if err := devReg.VerifyAndActivate(chal.ChallengeID, hex.EncodeToString(sig)); err != nil {
		t.Fatalf("activate device: %v", err)
	}
}
