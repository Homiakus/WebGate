package domain

import "time"

type ReleaseStatus string

const (
	ReleaseStatusDraft     ReleaseStatus = "DRAFT"
	ReleaseStatusVerified  ReleaseStatus = "VERIFIED"
	ReleaseStatusPromoted  ReleaseStatus = "PROMOTED"
	ReleaseStatusRevoked   ReleaseStatus = "REVOKED"
)

type PlatformArtifact struct {
	Platform     DevicePlatform     `json:"platform"`
	Architecture DeviceArchitecture `json:"architecture"`
	FileName     string             `json:"file_name"`
	SHA256Hex    string             `json:"sha256_hex"`
	SignatureHex string             `json:"signature_hex"`
	SizeBytes    int64              `json:"size_bytes"`
	DownloadURL  string             `json:"download_url"`
}

type Release struct {
	Version      string             `json:"version"`
	SourceCommit string             `json:"source_commit"`
	Status       ReleaseStatus      `json:"status"`
	Channel      string             `json:"channel"`
	Artifacts    []PlatformArtifact `json:"artifacts"`
	CreatedAt    time.Time          `json:"created_at"`
	PromotedAt   *time.Time         `json:"promoted_at,omitempty"`
}

// FindArtifact finds matching platform/architecture artifact in the release.
func (r *Release) FindArtifact(platform DevicePlatform, arch DeviceArchitecture) *PlatformArtifact {
	for i := range r.Artifacts {
		if r.Artifacts[i].Platform == platform && r.Artifacts[i].Architecture == arch {
			return &r.Artifacts[i]
		}
	}
	return nil
}
