package registry

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
)

var (
	ErrServiceNotFound      = errors.New("service not found")
	ErrServiceAlreadyExists = errors.New("service with this ID already exists")
	ErrSlugCollision        = errors.New("service with this slug already exists")
	ErrServiceInactive      = errors.New("service is inactive or disabled")
)

type ServicePersistence interface {
	SaveService(*domain.ProtectedService) error
	DeleteService(string) error
}

type ServiceRegistry struct {
	mu          sync.RWMutex
	byID        map[string]*domain.ProtectedService
	bySlug      map[string]*domain.ProtectedService
	persistence ServicePersistence
}

func NewServiceRegistry() *ServiceRegistry {
	return NewServiceRegistryWithPersistence(nil)
}

func NewServiceRegistryWithPersistence(persistence ServicePersistence) *ServiceRegistry {
	return &ServiceRegistry{
		byID:        make(map[string]*domain.ProtectedService),
		bySlug:      make(map[string]*domain.ProtectedService),
		persistence: persistence,
	}
}

// Restore atomically replaces the registry with durable snapshots. Runtime
// process state is never resurrected across a server restart.
func (r *ServiceRegistry) Restore(services []*domain.ProtectedService) error {
	byID := make(map[string]*domain.ProtectedService, len(services))
	bySlug := make(map[string]*domain.ProtectedService, len(services))
	for _, svc := range services {
		candidate := durableServiceSnapshot(svc)
		if candidate == nil {
			return fmt.Errorf("validation error: %w", domain.ErrInvalidServiceID)
		}
		if err := candidate.Validate(); err != nil {
			return fmt.Errorf("validation error: %w", err)
		}
		if _, exists := byID[candidate.ID]; exists {
			return ErrServiceAlreadyExists
		}
		if _, exists := bySlug[candidate.Slug]; exists {
			return ErrSlugCollision
		}
		byID[candidate.ID] = candidate
		bySlug[candidate.Slug] = candidate
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID = byID
	r.bySlug = bySlug
	return nil
}

// Register adds a new validated ProtectedService to the registry. The registry
// owns an isolated copy so callers cannot mutate registered state outside the
// lock/validation/versioning boundary.
func (r *ServiceRegistry) Register(svc *domain.ProtectedService) error {
	candidate := durableServiceSnapshot(svc)
	if candidate == nil {
		return fmt.Errorf("validation error: %w", domain.ErrInvalidServiceID)
	}
	if err := candidate.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[candidate.ID]; exists {
		return ErrServiceAlreadyExists
	}
	if _, exists := r.bySlug[candidate.Slug]; exists {
		return ErrSlugCollision
	}

	now := time.Now().UTC()
	candidate.CreatedAt = now
	candidate.UpdatedAt = now
	candidate.Version = 1
	if err := r.saveServiceLocked(candidate); err != nil {
		return err
	}

	r.byID[candidate.ID] = candidate
	r.bySlug[candidate.Slug] = candidate
	return nil
}

// GetByID returns a detached snapshot of the service by its unique ID.
func (r *ServiceRegistry) GetByID(id string) (*domain.ProtectedService, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	svc, ok := r.byID[id]
	if !ok {
		return nil, ErrServiceNotFound
	}
	return cloneProtectedService(svc), nil
}

// ResolveBySlug returns a detached snapshot of the service by its routing slug.
func (r *ServiceRegistry) ResolveBySlug(slug string) (*domain.ProtectedService, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	svc, ok := r.bySlug[slug]
	if !ok {
		return nil, ErrServiceNotFound
	}
	return cloneProtectedService(svc), nil
}

// List returns detached snapshots of all registered services.
func (r *ServiceRegistry) List() []*domain.ProtectedService {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*domain.ProtectedService, 0, len(r.byID))
	for _, svc := range r.byID {
		list = append(list, cloneProtectedService(svc))
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

	// MUTANT: violate persist-before-memory. A durable write failure now leaves
	// authoritative memory ahead of disk and must be killed by tests.
	svc.Status = status
	svc.Version++
	svc.UpdatedAt = time.Now().UTC()
	if err := r.saveServiceLocked(svc); err != nil {
		return err
	}
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
	candidate := cloneProtectedService(svc)
	candidate.UpstreamURL = canonicalUpstream
	candidate.Version++
	candidate.UpdatedAt = time.Now().UTC()
	if err := r.saveServiceLocked(candidate); err != nil {
		return err
	}
	r.replaceServiceLocked(svc, candidate)
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
	candidate := cloneProtectedService(svc)
	candidate.Port = port
	candidate.ExecutablePath = execPath
	if len(execArgs) > 0 {
		candidate.ExecArgs = append([]string(nil), execArgs...)
	}
	if port > 0 {
		candidate.UpstreamURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	candidate.Version++
	candidate.UpdatedAt = time.Now().UTC()
	if err := r.saveServiceLocked(candidate); err != nil {
		return err
	}
	r.replaceServiceLocked(svc, candidate)
	return nil
}

// UpdateProcessRuntime records process-manager-owned runtime state through the
// registry lock. Runtime PID/state changes intentionally do not increment or
// write durable configuration state.
func (r *ServiceRegistry) UpdateProcessRuntime(id string, state domain.ProcessState, pid int, startedAt *time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	svc, ok := r.byID[id]
	if !ok {
		return ErrServiceNotFound
	}

	svc.ProcessState = state
	svc.ProcessPID = pid
	if startedAt == nil {
		svc.StartedAt = nil
	} else {
		startedCopy := startedAt.UTC()
		svc.StartedAt = &startedCopy
	}
	svc.UpdatedAt = time.Now().UTC()
	return nil
}

// Unregister removes a service from the registry only after durable deletion succeeds.
func (r *ServiceRegistry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	svc, ok := r.byID[id]
	if !ok {
		return ErrServiceNotFound
	}
	if r.persistence != nil {
		if err := r.persistence.DeleteService(id); err != nil {
			return err
		}
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
	}

	candidate := cloneProtectedService(svc)
	candidate.Name = name
	candidate.Slug = slug
	candidate.Description = description
	candidate.Port = port
	candidate.ExecutablePath = execPath
	candidate.ExecArgs = append([]string(nil), execArgs...)
	candidate.WorkingDir = workingDir
	candidate.AutoStart = autoStart
	if port > 0 {
		candidate.UpstreamURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	candidate.Version++
	candidate.UpdatedAt = time.Now().UTC()
	if err := candidate.Validate(); err != nil {
		return fmt.Errorf("validation error: %w", err)
	}
	if err := r.saveServiceLocked(candidate); err != nil {
		return err
	}
	r.replaceServiceLocked(svc, candidate)
	return nil
}

func (r *ServiceRegistry) saveServiceLocked(svc *domain.ProtectedService) error {
	if r.persistence == nil {
		return nil
	}
	return r.persistence.SaveService(durableServiceSnapshot(svc))
}

func (r *ServiceRegistry) replaceServiceLocked(previous, candidate *domain.ProtectedService) {
	if previous.Slug != candidate.Slug {
		delete(r.bySlug, previous.Slug)
	}
	r.byID[candidate.ID] = candidate
	r.bySlug[candidate.Slug] = candidate
}

func durableServiceSnapshot(svc *domain.ProtectedService) *domain.ProtectedService {
	clone := cloneProtectedService(svc)
	if clone == nil {
		return nil
	}
	clone.ProcessState = domain.ProcessStateStopped
	clone.ProcessPID = 0
	clone.StartedAt = nil
	return clone
}

func cloneProtectedService(svc *domain.ProtectedService) *domain.ProtectedService {
	if svc == nil {
		return nil
	}
	clone := *svc
	clone.ExecArgs = append([]string(nil), svc.ExecArgs...)
	if svc.StartedAt != nil {
		startedAt := *svc.StartedAt
		clone.StartedAt = &startedAt
	}
	return &clone
}
