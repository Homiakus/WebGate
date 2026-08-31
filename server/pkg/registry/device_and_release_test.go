package registry_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func TestDeviceRegistryLifecycle(t *testing.T) {
	devReg := registry.NewDeviceRegistry()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	dev := &domain.Device{
		ID:           "dev_laptop",
		UserID:       "user_dave",
		PublicKeyHex: hex.EncodeToString(publicKey),
		Algorithm:    "Ed25519",
		Platform:     domain.PlatformWindows,
		Architecture: domain.ArchX86_64,
	}

	if err := devReg.Enroll(dev); err != nil {
		t.Fatalf("failed to enroll device: %v", err)
	}

	enrolled, _ := devReg.GetDevice("dev_laptop")
	if enrolled.Status != domain.DeviceStatusPending {
		t.Fatalf("enrolled device should be PENDING, got %s", enrolled.Status)
	}
	if err := devReg.UpdateStatus(dev.ID, domain.DeviceStatusActive); !errors.Is(err, registry.ErrActivationRequiresProof) {
		t.Fatalf("expected direct activation to be rejected, got %v", err)
	}

	chal, err := devReg.CreateChallenge("dev_laptop", time.Minute)
	if err != nil {
		t.Fatalf("failed to create challenge: %v", err)
	}
	payload, err := registry.ChallengeSigningPayload(chal, dev)
	if err != nil {
		t.Fatalf("failed to build signing payload: %v", err)
	}
	signature := ed25519.Sign(privateKey, payload)

	if err := devReg.VerifyAndActivate(chal.ChallengeID, hex.EncodeToString(signature)); err != nil {
		t.Fatalf("failed to activate device: %v", err)
	}

	activeDev, _ := devReg.GetDevice("dev_laptop")
	if activeDev.Status != domain.DeviceStatusActive {
		t.Fatalf("activated device should be ACTIVE, got %s", activeDev.Status)
	}

	if err := devReg.VerifyAndActivate(chal.ChallengeID, hex.EncodeToString(signature)); !errors.Is(err, registry.ErrChallengeExpired) {
		t.Fatalf("expected challenge replay to fail, got %v", err)
	}

	if err := devReg.RevokeDevice("dev_laptop"); err != nil {
		t.Fatalf("failed to revoke device: %v", err)
	}
	revokedDev, _ := devReg.GetDevice("dev_laptop")
	if revokedDev.Status != domain.DeviceStatusRevoked {
		t.Fatalf("revoked device should be REVOKED, got %s", revokedDev.Status)
	}
}

func TestDeviceRegistryRejectsForgedSignature(t *testing.T) {
	devReg := registry.NewDeviceRegistry()
	publicKey, _, _ := ed25519.GenerateKey(rand.Reader)
	_, attackerKey, _ := ed25519.GenerateKey(rand.Reader)
	dev := &domain.Device{
		ID:           "dev_secure",
		UserID:       "user_secure",
		PublicKeyHex: hex.EncodeToString(publicKey),
		Algorithm:    "Ed25519",
	}
	if err := devReg.Enroll(dev); err != nil {
		t.Fatal(err)
	}
	chal, err := devReg.CreateChallenge(dev.ID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := registry.ChallengeSigningPayload(chal, dev)
	if err != nil {
		t.Fatal(err)
	}
	forged := ed25519.Sign(attackerKey, payload)
	if err := devReg.VerifyAndActivate(chal.ChallengeID, hex.EncodeToString(forged)); !errors.Is(err, registry.ErrInvalidSignature) {
		t.Fatalf("expected forged signature rejection, got %v", err)
	}
}

func TestReleaseRegistryLifecycle(t *testing.T) {
	relReg := registry.NewReleaseRegistry()

	rel := &domain.Release{
		Version:      "v2.0.0",
		SourceCommit: "git_sha_xyz",
		Artifacts: []domain.PlatformArtifact{
			{
				Platform:     domain.PlatformAndroid,
				Architecture: domain.ArchArm64,
				FileName:     "webgate-arm64.apk",
				SHA256Hex:    "1234abcd",
				SizeBytes:    25 * 1024 * 1024,
			},
		},
	}

	if err := relReg.AddDraft(rel); err != nil {
		t.Fatalf("failed to add draft: %v", err)
	}

	_, _, err := relReg.GetLatestPromoted(domain.PlatformAndroid, domain.ArchArm64)
	if err != registry.ErrArtifactNotFound {
		t.Fatalf("draft release should not be returned as promoted")
	}

	if err := relReg.Verify("v2.0.0"); err != nil {
		t.Fatalf("failed to verify release: %v", err)
	}
	if err := relReg.Promote("v2.0.0"); err != nil {
		t.Fatalf("failed to promote release: %v", err)
	}

	promoted, art, err := relReg.GetLatestPromoted(domain.PlatformAndroid, domain.ArchArm64)
	if err != nil || promoted.Version != "v2.0.0" || art.FileName != "webgate-arm64.apk" {
		t.Fatalf("unexpected promoted release: %v, %v, %v", promoted, art, err)
	}

	if err := relReg.Revoke("v2.0.0"); err != nil {
		t.Fatalf("failed to revoke release: %v", err)
	}
	_, _, err = relReg.GetLatestPromoted(domain.PlatformAndroid, domain.ArchArm64)
	if err != registry.ErrArtifactNotFound {
		t.Fatalf("revoked release should no longer be returned")
	}
}
