package admin

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/config"
	"github.com/Homiakus/WebGate/server/pkg/domain"
)

type DurableAdminStateStore interface {
	LoadAudit() ([]domain.AuditEvent, error)
	AppendAuditBatch([]domain.AuditEvent) error
	LoadControlConfig() (*config.DurableServerConfig, error)
	SaveControlConfig(*config.DurableServerConfig) error
	SaveControlConfigWithAudit(*config.DurableServerConfig, domain.AuditEvent) error
}

// DurableAdminHandler is the production serialization/durability boundary around
// the historical AdminAPI mux. It owns durable config/audit endpoints and buffers
// ordinary admin responses until newly produced audit records are durable.
type DurableAdminHandler struct {
	api   *AdminAPI
	store DurableAdminStateStore
	next  http.Handler
	mu    sync.Mutex
}

func NewDurableAdminHandler(api *AdminAPI, store DurableAdminStateStore, next http.Handler) (*DurableAdminHandler, error) {
	if api == nil || store == nil || next == nil {
		return nil, errors.New("admin API, durable state store and inner handler are required")
	}
	events, err := store.LoadAudit()
	if err != nil {
		return nil, fmt.Errorf("restore durable audit log: %w", err)
	}
	api.replaceAuditLog(events)
	return &DurableAdminHandler{api: api, store: store, next: next}, nil
}

// InstallConfig installs an already validated effective configuration in memory.
// Service definitions are intentionally not applied here: the service registry is
// a separate durable authority after initial bootstrap.
func (a *AdminAPI) InstallConfig(cfg *config.ServerConfig) {
	if cfg == nil {
		return
	}
	clone := config.CloneServerConfig(cfg)
	a.cfg = clone
	if clone.TelegramBotToken != "" {
		a.adminBot.SetBotToken(clone.TelegramBotToken)
	}
	a.adminBot.SetAuthorizedAdmins(clone.TelegramAdminChatIDs)
}

func (a *AdminAPI) ConfigSnapshot() *config.ServerConfig {
	return config.CloneServerConfig(a.cfg)
}

func (a *AdminAPI) AuditSnapshot() []domain.AuditEvent {
	return append([]domain.AuditEvent(nil), a.auditLog...)
}

func (a *AdminAPI) replaceAuditLog(events []domain.AuditEvent) {
	a.auditLog = append([]domain.AuditEvent(nil), events...)
}

func (a *AdminAPI) appendAuditMemory(event domain.AuditEvent) {
	a.auditLog = append(a.auditLog, event)
}

func (h *DurableAdminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/admin/config":
		h.handleGetConfig(w)
		return
	case r.Method == http.MethodPost && r.URL.Path == "/api/admin/config/update":
		h.handleUpdateConfig(w, r)
		return
	case r.Method == http.MethodPost && r.URL.Path == "/api/admin/config/bind":
		h.handleBindConfig(w, r)
		return
	case r.Method == http.MethodGet && r.URL.Path == "/api/admin/audit":
		if err := h.syncAudit(); err != nil {
			http.Error(w, "durable audit unavailable: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, http.StatusOK, h.api.AuditSnapshot())
		return
	}

	buffer := newBufferedResponse()
	h.next.ServeHTTP(buffer, r)
	if err := h.syncAudit(); err != nil {
		http.Error(w, "durable audit unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	buffer.flushTo(w)
}

func (h *DurableAdminHandler) RecordAudit(action domain.AuditAction, actorID, targetID, details string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	event, err := h.newAuditEvent(action, actorID, targetID, details)
	if err != nil {
		return err
	}
	if err := h.store.AppendAuditBatch([]domain.AuditEvent{event}); err != nil {
		return err
	}
	h.api.appendAuditMemory(event)
	return nil
}

func (h *DurableAdminHandler) handleGetConfig(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, config.RedactedCopy(h.api.ConfigSnapshot()))
}

type durableConfigUpdateRequest struct {
	ServerName            *string                 `json:"server_name"`
	ListenAddr            *string                 `json:"listen_addr"`
	AdminAddr             *string                 `json:"admin_addr"`
	ProxyTimeoutSecs      *int                    `json:"proxy_timeout_seconds"`
	TelegramBotToken      *string                 `json:"telegram_bot_token"`
	TelegramBotEnabled    *bool                   `json:"telegram_bot_enabled"`
	TelegramAdminChatIDs  []int64                 `json:"telegram_admin_chat_ids"`
	TelegramAPIEndpoint   *string                 `json:"telegram_api_endpoint"`
	TelegramMaxFileSizeMB *int                    `json:"telegram_max_file_size_mb"`
	RelayNodes            []config.RelayNodeEntry `json:"relay_nodes"`
}

func (h *DurableAdminHandler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req durableConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	current := h.api.ConfigSnapshot()
	if current == nil {
		current = config.DefaultServerConfig()
	}
	next := config.CloneServerConfig(current)
	if req.ServerName != nil {
		next.ServerName = strings.TrimSpace(*req.ServerName)
	}
	if req.ListenAddr != nil {
		next.ListenAddr = strings.TrimSpace(*req.ListenAddr)
	}
	if req.AdminAddr != nil {
		next.AdminAddr = strings.TrimSpace(*req.AdminAddr)
	}
	if req.ProxyTimeoutSecs != nil {
		if *req.ProxyTimeoutSecs <= 0 {
			http.Error(w, "proxy_timeout_seconds must be positive", http.StatusBadRequest)
			return
		}
		next.ProxyTimeoutSecs = *req.ProxyTimeoutSecs
	}
	if req.TelegramBotToken != nil {
		next.TelegramBotToken = *req.TelegramBotToken
	}
	if req.TelegramBotEnabled != nil {
		next.TelegramBotEnabled = *req.TelegramBotEnabled
	}
	if req.TelegramAdminChatIDs != nil {
		next.TelegramAdminChatIDs = append([]int64(nil), req.TelegramAdminChatIDs...)
	}
	if req.TelegramAPIEndpoint != nil {
		next.TelegramAPIEndpoint = strings.TrimSpace(*req.TelegramAPIEndpoint)
	}
	if req.TelegramMaxFileSizeMB != nil {
		if *req.TelegramMaxFileSizeMB <= 0 {
			http.Error(w, "telegram_max_file_size_mb must be positive", http.StatusBadRequest)
			return
		}
		next.TelegramMaxFileSizeMB = *req.TelegramMaxFileSizeMB
	}
	if req.RelayNodes != nil {
		next.RelayNodes = append([]config.RelayNodeEntry(nil), req.RelayNodes...)
	}
	if err := config.HardenRuntimeAddresses(next); err != nil {
		http.Error(w, "unsafe control configuration: "+err.Error(), http.StatusBadRequest)
		return
	}

	restartRequired := runtimeConfigDiffers(current, next)
	event, err := h.newAuditEvent(domain.AuditActionServiceUpdated, "admin", "config", "Server control settings updated: "+next.ServerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.store.SaveControlConfigWithAudit(config.DurableSnapshot(next), event); err != nil {
		http.Error(w, "durable config update failed: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	h.api.InstallConfig(next)
	h.api.appendAuditMemory(event)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "updated",
		"restart_required": restartRequired,
		"config":           config.RedactedCopy(next),
	})
}

func (h *DurableAdminHandler) handleBindConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConfigPath string `json:"config_path"`
		Content    string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var newCfg *config.ServerConfig
	var err error
	var tempPath string
	if strings.TrimSpace(req.ConfigPath) != "" {
		newCfg, err = config.LoadConfigFile(req.ConfigPath)
	} else if req.Content != "" {
		tempPath, err = osCreateTempConfig(req.Content)
		if err == nil {
			defer os.Remove(tempPath)
			newCfg, err = config.LoadConfigFile(tempPath)
		}
	} else {
		http.Error(w, "Either config_path or content is required", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load config: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := config.HardenRuntimeAddresses(newCfg); err != nil {
		http.Error(w, "unsafe control configuration: "+err.Error(), http.StatusBadRequest)
		return
	}

	current := h.api.ConfigSnapshot()
	restartRequired := runtimeConfigDiffers(current, newCfg)
	event, err := h.newAuditEvent(domain.AuditActionServiceUpdated, "admin", "config", "Bound durable control config: "+newCfg.ServerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.store.SaveControlConfigWithAudit(config.DurableSnapshot(newCfg), event); err != nil {
		http.Error(w, "durable config bind failed: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	h.api.InstallConfig(newCfg)
	h.api.appendAuditMemory(event)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":                     "bound",
		"restart_required":           restartRequired,
		"service_registry_unchanged": true,
		"config":                     config.RedactedCopy(newCfg),
	})
}

func (h *DurableAdminHandler) syncAudit() error {
	events := h.api.AuditSnapshot()
	for i := range events {
		events[i].Details = h.sanitizeAuditDetails(events[i].Details)
		events[i].Timestamp = events[i].Timestamp.UTC()
	}
	if err := h.store.AppendAuditBatch(events); err != nil {
		return err
	}
	h.api.replaceAuditLog(events)
	return nil
}

func (h *DurableAdminHandler) newAuditEvent(action domain.AuditAction, actorID, targetID, details string) (domain.AuditEvent, error) {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return domain.AuditEvent{}, fmt.Errorf("generate audit event ID: %w", err)
	}
	return domain.AuditEvent{
		ID:        hex.EncodeToString(idBytes),
		Action:    action,
		ActorID:   actorID,
		TargetID:  targetID,
		Details:   h.sanitizeAuditDetails(details),
		Timestamp: time.Now().UTC(),
	}, nil
}

func (h *DurableAdminHandler) sanitizeAuditDetails(details string) string {
	secrets := []string{
		os.Getenv("WEBGATE_ADMIN_TOKEN"),
		os.Getenv("WEBGATE_AUTHORITY_TOKEN"),
	}
	if cfg := h.api.ConfigSnapshot(); cfg != nil {
		secrets = append(secrets, cfg.TelegramBotToken)
	}
	for _, secret := range secrets {
		if secret = strings.TrimSpace(secret); secret != "" {
			details = strings.ReplaceAll(details, secret, "[REDACTED]")
		}
	}
	return details
}

func runtimeConfigDiffers(current, next *config.ServerConfig) bool {
	if current == nil || next == nil {
		return true
	}
	if current.ListenAddr != next.ListenAddr || current.AdminAddr != next.AdminAddr || current.ProxyTimeoutSecs != next.ProxyTimeoutSecs {
		return true
	}
	if len(current.RelayNodes) != len(next.RelayNodes) {
		return true
	}
	for i := range current.RelayNodes {
		if current.RelayNodes[i] != next.RelayNodes[i] {
			return true
		}
	}
	return false
}

type bufferedResponse struct {
	header      http.Header
	body        bytes.Buffer
	status      int
	wroteHeader bool
}

func newBufferedResponse() *bufferedResponse {
	return &bufferedResponse{header: make(http.Header), status: http.StatusOK}
}

func (b *bufferedResponse) Header() http.Header { return b.header }

func (b *bufferedResponse) WriteHeader(status int) {
	if b.wroteHeader {
		return
	}
	b.status = status
	b.wroteHeader = true
}

func (b *bufferedResponse) Write(p []byte) (int, error) {
	if !b.wroteHeader {
		b.WriteHeader(http.StatusOK)
	}
	return b.body.Write(p)
}

func (b *bufferedResponse) flushTo(w http.ResponseWriter) {
	for key, values := range b.header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(b.status)
	_, _ = w.Write(b.body.Bytes())
}
