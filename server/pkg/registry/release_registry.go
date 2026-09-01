package registry

import (
	"errors"
	"sync"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
)

var (
	ErrReleaseNotFound    = errors.New("release not found")
	ErrReleaseExists      = errors.New("release version already exists")
	ErrInvalidStateChange = errors.New("invalid release state transition")
	ErrArtifactNotFound   = errors.New("no compatible artifact for target platform/architecture")
)

type ReleasePersistence interface {
	SaveRelease(*domain.Release) error
}

type ReleaseRegistry struct {
	mu          sync.RWMutex
	releases    map[string]*domain.Release
	persistence ReleasePersistence
}

func NewReleaseRegistry() *ReleaseRegistry {
	return NewReleaseRegistryWithPersistence(nil)
}

func NewReleaseRegistryWithPersistence(persistence ReleasePersistence) *ReleaseRegistry {
	return &ReleaseRegistry{
		releases:    make(map[string]*domain.Release),
		persistence: persistence,
	}
}

// Restore atomically replaces durable release state without writing it again.
func (r *ReleaseRegistry) Restore(releases []*domain.Release) error {
	restored := make(map[string]*domain.Release, len(releases))
	for _, rel := range releases {
		candidate := cloneRelease(rel)
		if candidate == nil || candidate.Version == "" {
			return ErrInvalidStateChange
		}
		if _, exists := restored[candidate.Version]; exists {
			return ErrReleaseExists
		}
		restored[candidate.Version] = candidate
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.releases = restored
	return nil
}

// AddDraft creates a new draft release candidate and owns an isolated copy of
// the release/artifact state.
func (r *ReleaseRegistry) AddDraft(rel *domain.Release) error {
	candidate := cloneRelease(rel)
	if candidate == nil || candidate.Version == "" {
		return ErrInvalidStateChange
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.releases[candidate.Version]; exists {
		return ErrReleaseExists
	}

	candidate.Status = domain.ReleaseStatusDraft
	candidate.CreatedAt = time.Now().UTC()
	if err := r.saveReleaseLocked(candidate); err != nil {
		return err
	}
	r.releases[candidate.Version] = candidate
	return nil
}

// Verify marks a draft release as verified by automated qualification suites.
func (r *ReleaseRegistry) Verify(version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rel, ok := r.releases[version]
	if !ok {
		return ErrReleaseNotFound
	}
	if rel.Status != domain.ReleaseStatusDraft {
		return ErrInvalidStateChange
	}
	candidate := cloneRelease(rel)
	candidate.Status = domain.ReleaseStatusVerified
	if err := r.saveReleaseLocked(candidate); err != nil {
		return err
	}
	r.releases[version] = candidate
	return nil
}

// Promote promotes a verified release to PROMOTED status.
func (r *ReleaseRegistry) Promote(version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rel, ok := r.releases[version]
	if !ok {
		return ErrReleaseNotFound
	}
	if rel.Status != domain.ReleaseStatusVerified {
		return ErrInvalidStateChange
	}

	candidate := cloneRelease(rel)
	now := time.Now().UTC()
	candidate.Status = domain.ReleaseStatusPromoted
	candidate.PromotedAt = &now
	if err := r.saveReleaseLocked(candidate); err != nil {
		return err
	}
	r.releases[version] = candidate
	return nil
}

// Revoke revokes a release immediately.
func (r *ReleaseRegistry) Revoke(version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rel, ok := r.releases[version]
	if !ok {
		return ErrReleaseNotFound
	}
	candidate := cloneRelease(rel)
	candidate.Status = domain.ReleaseStatusRevoked
	if err := r.saveReleaseLocked(candidate); err != nil {
		return err
	}
	r.releases[version] = candidate
	return nil
}

// GetLatestPromoted finds the latest promoted release containing a compatible
// artifact and returns detached snapshots only.
func (r *ReleaseRegistry) GetLatestPromoted(platform domain.DevicePlatform, arch domain.DeviceArchitecture) (*domain.Release, *domain.PlatformArtifact, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var latest *domain.Release
	for _, rel := range r.releases {
		if rel.Status != domain.ReleaseStatusPromoted {
			continue
		}
		if rel.FindArtifact(platform, arch) == nil {
			continue
		}

		if latest == nil || (rel.PromotedAt != nil && latest.PromotedAt != nil && rel.PromotedAt.After(*latest.PromotedAt)) {
			latest = rel
		}
	}

	if latest == nil {
		return nil, nil, ErrArtifactNotFound
	}
	result := cloneRelease(latest)
	artifact := result.FindArtifact(platform, arch)
	if artifact == nil {
		return nil, nil, ErrArtifactNotFound
	}
	return result, artifact, nil
}

// List returns detached snapshots of all releases.
func (r *ReleaseRegistry) List() []*domain.Release {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*domain.Release, 0, len(r.releases))
	for _, rel := range r.releases {
		list = append(list, cloneRelease(rel))
	}
	return list
}

func (r *ReleaseRegistry) saveReleaseLocked(rel *domain.Release) error {
	if r.persistence == nil {
		return nil
	}
	return r.persistence.SaveRelease(cloneRelease(rel))
}

func cloneRelease(rel *domain.Release) *domain.Release {
	if rel == nil {
		return nil
	}
	clone := *rel
	clone.Artifacts = append([]domain.PlatformArtifact(nil), rel.Artifacts...)
	if rel.PromotedAt != nil {
		promotedAt := *rel.PromotedAt
		clone.PromotedAt = &promotedAt
	}
	return &clone
}
