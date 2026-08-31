package domain

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
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

func (p PermissionBits) Has(required PermissionBits) bool {
	return (p & required) == required
}

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
	ErrInvalidUpstreamURL = errors.New("upstream URL must be an explicit HTTP/HTTPS loopback or private IP destination")
)

func CanonicalizeUpstreamURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalidUpstreamURL
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.Host == "" {
		return "", ErrInvalidUpstreamURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrInvalidUpstreamURL
	}
	if u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return "", ErrInvalidUpstreamURL
	}
	port := u.Port()
	if port != "" {
		parsedPort, err := strconv.Atoi(port)
		if err != nil || parsedPort < 1 || parsedPort > 65535 {
			return "", ErrInvalidUpstreamURL
		}
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" {
		host = "127.0.0.1"
	} else {
		ip := net.ParseIP(host)
		if ip != nil && !allowedPrivateUpstreamIP(ip) {
			return "", ErrInvalidUpstreamURL
		}
		if ip != nil {
			host = ip.String()
		}
	}
	if strings.Contains(host, ":") {
		if port == "" {
			u.Host = "[" + host + "]"
		} else {
			u.Host = net.JoinHostPort(host, port)
		}
	} else if port == "" {
		u.Host = host
	} else {
		u.Host = net.JoinHostPort(host, port)
	}
	u.RawPath = ""
	return u.String(), nil
}

func allowedPrivateUpstreamIP(ip net.IP) bool {
	if ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}

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
	canonicalUpstream, err := CanonicalizeUpstreamURL(s.UpstreamURL)
	if err != nil {
		return err
	}
	s.UpstreamURL = canonicalUpstream
	if s.ProcessState == "" {
		s.ProcessState = ProcessStateStopped
	}
	return nil
}
