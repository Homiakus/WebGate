package registry

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
)

var (
	ErrServiceNotFound     = errors.New("service not found")
	ErrServiceAlreadyExists = errors.New("service with this ID already exists")
	ErrSlugCollision       = errors.New("service with this slug already exists")
	ErrServiceInactive     = errors.New("service is inactive or disabled")
)

type ServiceRegistry struct {
	mu     sync.RWMutex
	byID   map[string]*domain.ProtectedService
	bySlug map[string]*domain.ProtectedService
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		byID:   make(map[string]*domain.ProtectedService),
		bySlug: make(map[string]*domain.ProtectedService),
	}
}

// Register adds a new validated ProtectedService to the registry.
func (r *ServiceRegistry) Register(svc *domain.ProtectedService) error {
	if err := svc.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[svc.ID]; exists {
		return ErrServiceAlreadyExists
	}
	if _, exists := r.bySlug[svc.Slug]; exists {
		return ErrSlugCollision
	}

	now := time.Now().UTC()
	svc.CreatedAt = now
	svc.UpdatedAt = now
	svc.Version = 1

	r.byID[svc.ID] = svc
	r.bySlug[svc.Slug] = svc
	return nil
}

// GetByID returns the service by its unique ID.
func (r *ServiceRegistry) GetByID(id string) (*domain.ProtectedService, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	svc, ok := r.byID[id]
	if !ok {
		return nil, ErrServiceNotFound
	}
	return svc, nil
}

// ResolveBySlug returns the service by its routing slug.
func (r *ServiceRegistry) ResolveBySlug(slug string) (*domain.ProtectedService, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	svc, ok := r.bySlug[slug]
	if !ok {
		return nil, ErrServiceNotFound
	}
	return svc, nil
}

// List returns a snapshot slice of all registered services.
func (r *ServiceRegistry) List() []*domain.ProtectedService {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*domain.ProtectedService, 0, len(r.byID))
	for _, svc := range r.byID {
		list = append(list, svc)
	}
	return list
}

// UpdateStatus changes the operational status of a service.
func (r *ServiceRegistry) UpdateStatus(id string, status domain.ServiceStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	svc, ok := r.byID[id]
	if !ok {
		return ErrServiceNotFound
	}

	svc.Status = status
	svc.Version++
	svc.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateRoute changes the upstream URL of an existing service.
func (r *ServiceRegistry) UpdateRoute(id string, upstreamURL string) error {
	canonicalUpstream, err := domain.CanonicalizeUpstreamURL(upstreamURL)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	svc, ok := r.byID[id]
	if !ok {
		return ErrServiceNotFound
	}

	svc.UpstreamURL = canonicalUpstream
	svc.Version++
	svc.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateExecutable updates the port, executable binary/script path, and startup arguments.
func (r *ServiceRegistry) UpdateExecutable(id string, port int, execPath string, execArgs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	svc, ok := r.byID[id]
	if !ok {
		return ErrServiceNotFound
	}

	svc.Port = port
	svc.ExecutablePath = execPath
	if len(execArgs) > 0 {
		svc.ExecArgs = execArgs
	}
	if port > 0 {
		svc.UpstreamURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	svc.Version++
	svc.UpdatedAt = time.Now().UTC()
	return nil
}

// Unregister removes a service from the registry.
func (r *ServiceRegistry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	svc, ok := r.byID[id]
	if !ok {
		return ErrServiceNotFound
	}

	delete(r.byID, id)
	delete(r.bySlug, svc.Slug)
	return nil
}

// UpdateFull updates full configuration of an existing service.
func (r *ServiceRegistry) UpdateFull(id, name, slug, description string, port int, execPath string, execArgs []string, workingDir string, autoStart bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	svc, ok := r.byID[id]
	if !ok {
		return ErrServiceNotFound
	}

	if slug != svc.Slug {
		if _, exists := r.bySlug[slug]; exists {
			return ErrSlugCollision
		}
		delete(r.bySlug, svc.Slug)
		svc.Slug = slug
		r.bySlug[slug] = svc
	}

	svc.Name = name
	svc.Description = description
	svc.Port = port
	svc.ExecutablePath = execPath
	svc.ExecArgs = execArgs
	svc.WorkingDir = workingDir
	svc.AutoStart = autoStart

	if port > 0 {
		svc.UpstreamURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}

	svc.Version++
	svc.UpdatedAt = time.Now().UTC()
	return nil
}
