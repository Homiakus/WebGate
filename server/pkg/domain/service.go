package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type ServiceStatus string

const (
	ServiceStatusActive   ServiceStatus = "ACTIVE"
	ServiceStatusDisabled ServiceStatus = "DISABLED"
	ServiceStatusDraining ServiceStatus = "DRAINING"
)

type ProcessState string

const (
	ProcessStateStopped  ProcessState = "STOPPED"
	ProcessStateStarting ProcessState = "STARTING"
	ProcessStateRunning  ProcessState = "RUNNING"
	ProcessStateCrashed  ProcessState = "CRASHED"
)

type PermissionBits uint32

const (
	PermView            PermissionBits = 1 << 0
	PermDownload        PermissionBits = 1 << 1
	PermUpload          PermissionBits = 1 << 2
	PermEdit            PermissionBits = 1 << 3
	PermDelete          PermissionBits = 1 << 4
	PermManageMembers   PermissionBits = 1 << 5
	PermManageWorkspace PermissionBits = 1 << 6
)

// Has checks if all bits in required are present in p.
func (p PermissionBits) Has(required PermissionBits) bool {
	return (p & required) == required
}

// ProtectedService represents a routable private application managed by WebGate.
type ProtectedService struct {
	ID             string        `json:"id"`
	TenantID       string        `json:"tenant_id"`
	WorkspaceID    string        `json:"workspace_id"`
	Name           string        `json:"name"`
	Slug           string        `json:"slug"`
	Description    string        `json:"description"`
	UpstreamURL    string        `json:"upstream_url"`
	Port           int           `json:"port"`
	ExecutablePath string        `json:"executable_path"`
	ExecArgs       []string      `json:"exec_args,omitempty"`
	WorkingDir     string        `json:"working_dir,omitempty"`
	AutoStart      bool          `json:"auto_start"`
	ProcessState   ProcessState  `json:"process_state"`
	ProcessPID     int           `json:"process_pid"`
	StartedAt      *time.Time    `json:"started_at,omitempty"`
	PublicPath     string        `json:"public_path"`
	Status         ServiceStatus `json:"status"`
	Version        uint64        `json:"version"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

var (
	ErrInvalidServiceID   = errors.New("service ID cannot be empty")
	ErrInvalidWorkspaceID = errors.New("workspace ID cannot be empty")
	ErrInvalidSlug        = errors.New("slug must be alphanumeric or hyphenated")
	ErrInvalidUpstreamURL = errors.New("upstream URL must be valid HTTP/HTTPS loopback or private destination")
)

// Validate validates server-owned fields of ProtectedService.
func (s *ProtectedService) Validate() error {
	if strings.TrimSpace(s.ID) == "" {
		return ErrInvalidServiceID
	}
	if strings.TrimSpace(s.WorkspaceID) == "" {
		return ErrInvalidWorkspaceID
	}
	slug := strings.TrimSpace(s.Slug)
	if slug == "" || strings.ContainsAny(slug, " /\\?#") {
		return ErrInvalidSlug
	}
	if s.Port > 0 && strings.TrimSpace(s.UpstreamURL) == "" {
		s.UpstreamURL = fmt.Sprintf("http://127.0.0.1:%d", s.Port)
	}
	if strings.TrimSpace(s.UpstreamURL) == "" {
		return ErrInvalidUpstreamURL
	}
	if s.ProcessState == "" {
		s.ProcessState = ProcessStateStopped
	}
	return nil
}
