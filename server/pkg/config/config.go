package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

// ServerConfig holds the full server-side control plane, gateway, and service registry configuration.
type ServerConfig struct {
	ConfigPath            string                  `json:"config_path,omitempty"`
	LoadedAt              time.Time               `json:"loaded_at"`
	ServerName            string                  `json:"server_name"`
	ListenAddr            string                  `json:"listen_addr"`
	AdminAddr             string                  `json:"admin_addr"`
	ProxyTimeoutSecs      int                     `json:"proxy_timeout_seconds"`
	TelegramBotToken      string                  `json:"telegram_bot_token,omitempty"`
	TelegramBotEnabled    bool                    `json:"telegram_bot_enabled"`
	TelegramAdminChatIDs  []int64                 `json:"telegram_admin_chat_ids,omitempty"`
	TelegramAPIEndpoint   string                  `json:"telegram_api_endpoint,omitempty"`
	TelegramMaxFileSizeMB int                     `json:"telegram_max_file_size_mb,omitempty"`
	Services              []ProtectedServiceEntry `json:"services"`
	RelayNodes            []RelayNodeEntry        `json:"relay_nodes"`
}

// ProtectedServiceEntry defines upstream routing, executables, and ports for protected services.
type ProtectedServiceEntry struct {
	ID             string   `json:"id"`
	TenantID       string   `json:"tenant_id"`
	WorkspaceID    string   `json:"workspace_id"`
	Name           string   `json:"name"`
	Slug           string   `json:"slug"`
	UpstreamURL    string   `json:"upstream_url"`
	Port           int      `json:"port"`
	ExecutablePath string   `json:"executable_path"`
	ExecArgs       []string `json:"exec_args,omitempty"`
	Status         string   `json:"status"`
}

// RelayNodeEntry defines an active relay node.
type RelayNodeEntry struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Role    string `json:"role"`
}

// DefaultServerConfig returns standard production-grade defaults.
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		LoadedAt:              time.Now().UTC(),
		ServerName:            "WebGate Editorial Gateway Node Alpha",
		ListenAddr:            ":8787",
		AdminAddr:             "127.0.0.1:8787",
		ProxyTimeoutSecs:      15,
		TelegramBotToken:      "",
		TelegramBotEnabled:    true,
		TelegramAdminChatIDs:  []int64{999001},
		TelegramAPIEndpoint:   "https://api.telegram.org",
		TelegramMaxFileSizeMB: 50,
		Services: []ProtectedServiceEntry{
			{
				ID:             "svc_docs",
				TenantID:       "tenant_default",
				WorkspaceID:    "ws_docs",
				Name:           "Corporate Knowledge Base",
				Slug:           "docs",
				UpstreamURL:    "http://127.0.0.1:8081",
				Port:           8081,
				ExecutablePath: "./bin/docs-server.exe",
				Status:         string(domain.ServiceStatusActive),
			},
			{
				ID:             "svc_factory",
				TenantID:       "tenant_default",
				WorkspaceID:    "ws_factory",
				Name:           "FactoryOS Production Terminal",
				Slug:           "factory",
				UpstreamURL:    "http://127.0.0.1:8082",
				Port:           8082,
				ExecutablePath: "./bin/factoryos.exe",
				Status:         string(domain.ServiceStatusActive),
			},
			{
				ID:             "svc_monitoring",
				TenantID:       "tenant_default",
				WorkspaceID:    "ws_monitoring",
				Name:           "Infrastructure Telemetry & Metrics",
				Slug:           "monitoring",
				UpstreamURL:    "http://127.0.0.1:8083",
				Port:           8083,
				ExecutablePath: "./bin/telemetry-agent.exe",
				Status:         string(domain.ServiceStatusActive),
			},
		},
		RelayNodes: []RelayNodeEntry{
			{
				ID:      "relay_alpha",
				Name:    "Relay-Alpha (Primary Transit)",
				Address: "127.0.0.1",
				Port:    43111,
				Role:    "primary",
			},
			{
				ID:      "relay_beta",
				Name:    "Relay-Beta (Failover Transit)",
				Address: "127.0.0.1",
				Port:    43112,
				Role:    "fallback",
			},
		},
	}
}

// LoadConfigFile loads and parses a configuration file (JSON or TOML-like format).
func LoadConfigFile(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read server config file: %w", err)
	}

	cfg := DefaultServerConfig()
	cfg.ConfigPath = path
	cfg.LoadedAt = time.Now().UTC()

	// Try JSON first
	if err := json.Unmarshal(data, cfg); err == nil {
		return cfg, nil
	}

	// Fallback to simple TOML/key-value parser
	lines := strings.Split(string(data), "\n")
	var customServices []ProtectedServiceEntry

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		switch k {
		case "server_name":
			cfg.ServerName = v
		case "listen_addr":
			cfg.ListenAddr = v
		case "admin_addr":
			cfg.AdminAddr = v
		case "telegram_bot_token":
			cfg.TelegramBotToken = v
		case "telegram_bot_enabled":
			cfg.TelegramBotEnabled = strings.ToLower(v) == "true" || v == "1"
		case "telegram_api_endpoint":
			cfg.TelegramAPIEndpoint = v
		case "telegram_max_file_size_mb":
			if mb, err := strconv.Atoi(v); err == nil {
				cfg.TelegramMaxFileSizeMB = mb
			}
		case "telegram_admin_chat_ids":
			var ids []int64
			for _, item := range strings.Split(v, ",") {
				if id, err := strconv.ParseInt(strings.TrimSpace(item), 10, 64); err == nil {
					ids = append(ids, id)
				}
			}
			if len(ids) > 0 {
				cfg.TelegramAdminChatIDs = ids
			}
		case "service":
			// Format: id|name|slug|upstream_url|port|executable_path|status
			sp := strings.Split(v, "|")
			if len(sp) >= 4 {
				port := 0
				execPath := ""
				status := "active"

				if len(sp) >= 5 {
					if p, err := strconv.Atoi(strings.TrimSpace(sp[4])); err == nil {
						port = p
					}
				}
				if len(sp) >= 6 {
					execPath = strings.TrimSpace(sp[5])
				}
				if len(sp) >= 7 {
					status = strings.TrimSpace(sp[6])
				}

				customServices = append(customServices, ProtectedServiceEntry{
					ID:             strings.TrimSpace(sp[0]),
					TenantID:       "tenant_default",
					WorkspaceID:    "ws_default",
					Name:           strings.TrimSpace(sp[1]),
					Slug:           strings.TrimSpace(sp[2]),
					UpstreamURL:    strings.TrimSpace(sp[3]),
					Port:           port,
					ExecutablePath: execPath,
					Status:         status,
				})
			}
		}
	}

	if len(customServices) > 0 {
		cfg.Services = customServices
	}

	return cfg, nil
}

// ApplyToRegistries populates or updates service and relay definitions into active registries.
func (c *ServerConfig) ApplyToRegistries(svcReg *registry.ServiceRegistry) error {
	if svcReg == nil {
		return errors.New("service registry is nil")
	}

	for _, s := range c.Services {
		port := s.Port
		upstream := s.UpstreamURL
		if port > 0 && upstream == "" {
			upstream = fmt.Sprintf("http://127.0.0.1:%d", port)
		}
		svc := &domain.ProtectedService{
			ID:             s.ID,
			TenantID:       s.TenantID,
			WorkspaceID:    s.WorkspaceID,
			Name:           s.Name,
			Slug:           s.Slug,
			UpstreamURL:    upstream,
			Port:           port,
			ExecutablePath: s.ExecutablePath,
			ExecArgs:       s.ExecArgs,
			Status:         domain.ServiceStatus(s.Status),
			ProcessState:   domain.ProcessStateStopped,
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}
		_ = svcReg.Register(svc)
	}

	return nil
}
