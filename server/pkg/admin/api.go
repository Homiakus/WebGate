package admin

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/config"
	"github.com/Homiakus/WebGate/server/pkg/delivery"
	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/process"
	"github.com/Homiakus/WebGate/server/pkg/registry"
	"github.com/Homiakus/WebGate/server/pkg/telegram"
)

//go:embed dashboard.html
var dashboardHTML []byte

type AdminAPI struct {
	services    *registry.ServiceRegistry
	devices     *registry.DeviceRegistry
	releases    *registry.ReleaseRegistry
	delivery    *delivery.TelegramDeliveryService
	authorizer  *auth.SecureAccessAuthorizer
	procManager *process.ProcessManager
	adminBot    *telegram.AdminBot
	auditLog    []domain.AuditEvent
	cfg         *config.ServerConfig
}

func NewAdminAPI(
	services *registry.ServiceRegistry,
	devices *registry.DeviceRegistry,
	releases *registry.ReleaseRegistry,
	delivery *delivery.TelegramDeliveryService,
	authorizer *auth.SecureAccessAuthorizer,
) *AdminAPI {
	pm := process.NewProcessManager(services)
	bot := telegram.NewAdminBot(services, pm)

	return &AdminAPI{
		services:    services,
		devices:     devices,
		releases:    releases,
		delivery:    delivery,
		authorizer:  authorizer,
		procManager: pm,
		adminBot:    bot,
		auditLog:    make([]domain.AuditEvent, 0),
		cfg:         config.DefaultServerConfig(),
	}
}

// GetProcessManager returns the active process manager.
func (a *AdminAPI) GetProcessManager() *process.ProcessManager {
	return a.procManager
}

// GetAdminBot returns the telegram admin bot instance.
func (a *AdminAPI) GetAdminBot() *telegram.AdminBot {
	return a.adminBot
}

// SetConfig sets the initial or updated server configuration.
func (a *AdminAPI) SetConfig(cfg *config.ServerConfig) {
	if cfg != nil {
		a.cfg = cfg
		_ = a.cfg.ApplyToRegistries(a.services)
		if cfg.TelegramBotToken != "" {
			a.adminBot.SetBotToken(cfg.TelegramBotToken)
		}
		if len(cfg.TelegramAdminChatIDs) > 0 {
			a.adminBot.SetAuthorizedAdmins(cfg.TelegramAdminChatIDs)
		}
	}
}

// LogAudit appends an immutable audit event.
func (a *AdminAPI) LogAudit(action domain.AuditAction, actorID, targetID, details string) {
	a.auditLog = append(a.auditLog, domain.AuditEvent{
		ID:        time.Now().Format("20060102150405.000000"),
		Action:    action,
		ActorID:   actorID,
		TargetID:  targetID,
		Details:   details,
		Timestamp: time.Now().UTC(),
	})
}

// RegisterRoutes registers all Admin API and UI endpoints onto a mux.
func (a *AdminAPI) RegisterRoutes(mux *http.ServeMux) {
	// Service management
	mux.HandleFunc("GET /api/admin/services", a.handleListServices)
	mux.HandleFunc("POST /api/admin/services", a.handleCreateService)
	mux.HandleFunc("POST /api/admin/services/status", a.handleUpdateServiceStatus)
	mux.HandleFunc("POST /api/admin/services/route", a.handleUpdateServiceRoute)
	mux.HandleFunc("POST /api/admin/services/exec", a.handleUpdateServiceExec)
	mux.HandleFunc("POST /api/admin/services/update-full", a.handleUpdateServiceFull)
	mux.HandleFunc("POST /api/admin/services/delete", a.handleDeleteService)

	// Process lifecycle control
	mux.HandleFunc("POST /api/admin/services/process/start", a.handleStartServiceProcess)
	mux.HandleFunc("POST /api/admin/services/process/stop", a.handleStopServiceProcess)
	mux.HandleFunc("POST /api/admin/services/process/restart", a.handleRestartServiceProcess)
	mux.HandleFunc("GET /api/admin/services/process/list", a.handleListProcesses)

	// File browser
	mux.HandleFunc("POST /api/admin/fs/browse-file", a.handleBrowseFile)

	// Telegram Admin Bot commands & live status
	mux.HandleFunc("POST /api/admin/telegram/command", a.handleTelegramCommand)
	mux.HandleFunc("POST /api/admin/telegram/callback", a.handleTelegramCallback)
	mux.HandleFunc("GET /api/admin/telegram/status", a.handleTelegramStatus)
	mux.HandleFunc("POST /api/admin/telegram/broadcast", a.handleTelegramBroadcast)

	// Config
	mux.HandleFunc("GET /api/admin/config", a.handleGetConfig)
	mux.HandleFunc("POST /api/admin/config/bind", a.handleBindConfig)
	mux.HandleFunc("POST /api/admin/config/update", a.handleUpdateConfig)

	// Devices
	mux.HandleFunc("GET /api/admin/devices", a.handleListDevices)
	mux.HandleFunc("POST /api/admin/devices/enroll", a.handleEnrollDevice)
	mux.HandleFunc("POST /api/admin/devices/status", a.handleUpdateDeviceStatus)
	mux.HandleFunc("POST /api/admin/devices/revoke", a.handleRevokeDevice)

	// Releases
	mux.HandleFunc("GET /api/admin/releases", a.handleListReleases)
	mux.HandleFunc("POST /api/admin/releases/draft", a.handleCreateReleaseDraft)
	mux.HandleFunc("POST /api/admin/releases/verify", a.handleVerifyRelease)
	mux.HandleFunc("POST /api/admin/releases/promote", a.handlePromoteRelease)
	mux.HandleFunc("POST /api/admin/releases/revoke", a.handleRevokeRelease)

	// Delivery & Audit
	mux.HandleFunc("POST /api/admin/delivery/send", a.handleSendReleaseTelegram)
	mux.HandleFunc("GET /api/admin/audit", a.handleListAudit)
	mux.HandleFunc("GET /healthz", a.handleHealthCheck)

	// Dashboard UI endpoints
	mux.HandleFunc("GET /admin", a.handleServeDashboardUI)
	mux.HandleFunc("GET /admin/", a.handleServeDashboardUI)
	mux.HandleFunc("GET /dashboard", a.handleServeDashboardUI)
	mux.HandleFunc("GET /dashboard/", a.handleServeDashboardUI)
	mux.HandleFunc("GET /{$}", a.handleServeDashboardUI)
}

func (a *AdminAPI) handleStartServiceProcess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceID string `json:"service_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	inst, err := a.procManager.StartService(req.ServiceID)
	if err != nil {
		http.Error(w, "Failed to start service process: "+err.Error(), http.StatusBadRequest)
		return
	}

	a.LogAudit(domain.AuditActionServiceUpdated, "admin", req.ServiceID, "Started service process (PID: "+string(rune(inst.PID))+")")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "started",
		"process": inst,
	})
}

func (a *AdminAPI) handleStopServiceProcess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceID string `json:"service_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.procManager.StopService(req.ServiceID); err != nil {
		http.Error(w, "Failed to stop service process: "+err.Error(), http.StatusBadRequest)
		return
	}

	a.LogAudit(domain.AuditActionServiceUpdated, "admin", req.ServiceID, "Stopped service process")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "stopped",
		"service_id": req.ServiceID,
	})
}

func (a *AdminAPI) handleRestartServiceProcess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceID string `json:"service_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	inst, err := a.procManager.RestartService(req.ServiceID)
	if err != nil {
		http.Error(w, "Failed to restart service process: "+err.Error(), http.StatusBadRequest)
		return
	}

	a.LogAudit(domain.AuditActionServiceUpdated, "admin", req.ServiceID, "Restarted service process")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "restarted",
		"process": inst,
	})
}

func (a *AdminAPI) handleListProcesses(w http.ResponseWriter, r *http.Request) {
	processes := a.procManager.ListAll()
	writeJSON(w, http.StatusOK, processes)
}

func (a *AdminAPI) handleUpdateServiceExec(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceID      string   `json:"service_id"`
		Port           int      `json:"port"`
		ExecutablePath string   `json:"executable_path"`
		ExecArgs       []string `json:"exec_args,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.services.UpdateExecutable(req.ServiceID, req.Port, req.ExecutablePath, req.ExecArgs); err != nil {
		http.Error(w, "Error updating service executable: "+err.Error(), http.StatusNotFound)
		return
	}

	a.LogAudit(domain.AuditActionServiceUpdated, "admin", req.ServiceID, "Bound executable: "+req.ExecutablePath)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":          "updated",
		"service_id":      req.ServiceID,
		"port":            req.Port,
		"executable_path": req.ExecutablePath,
	})
}

func (a *AdminAPI) handleTelegramCommand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChatID  int64  `json:"chat_id"`
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.ChatID == 0 {
		req.ChatID = 999001 // Default admin chat ID
	}

	resp := a.adminBot.HandleCommand(req.ChatID, req.Command)
	a.LogAudit(domain.AuditActionServiceUpdated, "telegram_admin", "bot", "Executed command: "+req.Command)
	writeJSON(w, http.StatusOK, resp)
}

func (a *AdminAPI) handleTelegramCallback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChatID int64  `json:"chat_id"`
		Data   string `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.ChatID == 0 {
		req.ChatID = 999001
	}

	resp := a.adminBot.HandleCallbackQuery(req.ChatID, req.Data)
	writeJSON(w, http.StatusOK, resp)
}

func (a *AdminAPI) handleTelegramStatus(w http.ResponseWriter, r *http.Request) {
	status := a.adminBot.GetStatus()
	writeJSON(w, http.StatusOK, status)
}

func (a *AdminAPI) handleTelegramBroadcast(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Message == "" {
		req.Message = "🔔 Тестовое системное оповещение WebGate: Шлюз доступа работает в штатном режиме."
	}

	for _, chatID := range a.cfg.TelegramAdminChatIDs {
		a.adminBot.HandleCommand(chatID, "/services")
	}

	a.LogAudit(domain.AuditActionDeliveryDispatched, "admin", "telegram_broadcast", req.Message)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "sent",
		"message": req.Message,
	})
}


func (a *AdminAPI) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.cfg)
}

func (a *AdminAPI) handleBindConfig(w http.ResponseWriter, r *http.Request) {
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

	if req.ConfigPath != "" {
		newCfg, err = config.LoadConfigFile(req.ConfigPath)
		if err != nil {
			http.Error(w, "Failed to load config file: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else if req.Content != "" {
		tmpFile, err := osCreateTempConfig(req.Content)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		newCfg, err = config.LoadConfigFile(tmpFile)
		if err != nil {
			http.Error(w, "Failed to parse config content: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		http.Error(w, "Either config_path or content is required", http.StatusBadRequest)
		return
	}

	a.cfg = newCfg
	_ = a.cfg.ApplyToRegistries(a.services)
	a.LogAudit(domain.AuditActionServiceUpdated, "admin", "config", "Bound server config: "+a.cfg.ServerName)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "bound",
		"message": "Configuration successfully bound & applied to registries",
		"config":  a.cfg,
	})
}

func osCreateTempConfig(content string) (string, error) {
	file, err := os.CreateTemp("", "webgate-config-*.toml")
	if err != nil {
		return "", err
	}
	defer file.Close()
	_, err = file.WriteString(content)
	return file.Name(), err
}

func (a *AdminAPI) handleUpdateServiceRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceID   string `json:"service_id"`
		UpstreamURL string `json:"upstream_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.services.UpdateRoute(req.ServiceID, req.UpstreamURL); err != nil {
		http.Error(w, "Service update error: "+err.Error(), http.StatusNotFound)
		return
	}

	a.LogAudit(domain.AuditActionServiceUpdated, "admin", req.ServiceID, "Updated upstream URL to "+req.UpstreamURL)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "updated",
		"service_id":   req.ServiceID,
		"upstream_url": req.UpstreamURL,
	})
}

func (a *AdminAPI) handleListServices(w http.ResponseWriter, r *http.Request) {
	services := a.services.List()
	writeJSON(w, http.StatusOK, services)
}

func (a *AdminAPI) handleCreateService(w http.ResponseWriter, r *http.Request) {
	var svc domain.ProtectedService
	if err := json.NewDecoder(r.Body).Decode(&svc); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if svc.TenantID == "" {
		svc.TenantID = "tenant_default"
	}
	if svc.Status == "" {
		svc.Status = domain.ServiceStatusActive
	}
	svc.CreatedAt = time.Now().UTC()
	svc.UpdatedAt = time.Now().UTC()

	if err := a.services.Register(&svc); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	a.LogAudit(domain.AuditActionServiceCreated, "admin", svc.ID, svc.Name)
	writeJSON(w, http.StatusCreated, svc)
}

func (a *AdminAPI) handleUpdateServiceStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceID string               `json:"service_id"`
		Status    domain.ServiceStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.services.UpdateStatus(req.ServiceID, req.Status); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	a.LogAudit(domain.AuditActionServiceUpdated, "admin", req.ServiceID, string(req.Status))
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (a *AdminAPI) handleListDevices(w http.ResponseWriter, r *http.Request) {
	devices := a.devices.List()
	writeJSON(w, http.StatusOK, devices)
}

func (a *AdminAPI) handleRevokeDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.devices.RevokeDevice(req.DeviceID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	a.LogAudit(domain.AuditActionDeviceRevoked, "admin", req.DeviceID, "revoked by admin")
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (a *AdminAPI) handleListReleases(w http.ResponseWriter, r *http.Request) {
	releases := a.releases.List()
	writeJSON(w, http.StatusOK, releases)
}

func (a *AdminAPI) handleCreateReleaseDraft(w http.ResponseWriter, r *http.Request) {
	var rel domain.Release
	if err := json.NewDecoder(r.Body).Decode(&rel); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.releases.AddDraft(&rel); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	writeJSON(w, http.StatusCreated, rel)
}

func (a *AdminAPI) handleVerifyRelease(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.releases.Verify(req.Version); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

func (a *AdminAPI) handlePromoteRelease(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.releases.Promote(req.Version); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.LogAudit(domain.AuditActionReleasePromoted, "admin", req.Version, "promoted to fleet")
	writeJSON(w, http.StatusOK, map[string]string{"status": "promoted"})
}

func (a *AdminAPI) handleRevokeRelease(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.releases.Revoke(req.Version); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.LogAudit(domain.AuditActionReleaseRevoked, "admin", req.Version, "revoked from fleet")
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (a *AdminAPI) handleSendReleaseTelegram(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IdempotencyKey string                    `json:"idempotency_key"`
		UserID         string                    `json:"user_id"`
		Platform       domain.DevicePlatform     `json:"platform"`
		Architecture   domain.DeviceArchitecture `json:"architecture"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.IdempotencyKey) == "" {
		req.IdempotencyKey = "tx_" + time.Now().Format("20060102150405")
	}

	receipt, err := a.delivery.SendLatestWebGate(req.IdempotencyKey, req.UserID, req.Platform, req.Architecture)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	a.LogAudit(domain.AuditActionDeliveryDispatched, "admin", req.UserID, receipt.DeliveryID)
	writeJSON(w, http.StatusOK, receipt)
}

func (a *AdminAPI) handleListAudit(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.auditLog)
}

func (a *AdminAPI) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "HEALTHY",
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"services_ct": len(a.services.List()),
		"devices_ct":  len(a.devices.List()),
		"releases_ct": len(a.releases.List()),
		"audit_ct":    len(a.auditLog),
	})
}

func (a *AdminAPI) handleBrowseFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Title == "" {
		req.Title = "Выберите исполняемый файл или файл конфигурации"
	}

	var selectedPath string
	var err error

	if runtime.GOOS == "windows" {
		psScript := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$f = New-Object System.Windows.Forms.OpenFileDialog
$f.Title = "%s"
$f.Filter = "Все поддерживаемые (*.exe;*.bat;*.cmd;*.ps1;*.sh;*.toml;*.json)|*.exe;*.bat;*.cmd;*.ps1;*.sh;*.toml;*.json|Исполняемые файлы (*.exe)|*.exe|Все файлы (*.*)|*.*"
$f.RestoreDirectory = $true
$f.ShowHelp = $true
if ($f.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
    [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
    Write-Output $f.FileName
}
`, strings.ReplaceAll(req.Title, `"`, `\"`))

		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
		out, runErr := cmd.Output()
		if runErr != nil {
			err = runErr
		} else {
			selectedPath = strings.TrimSpace(string(out))
		}
	} else if runtime.GOOS == "darwin" {
		cmd := exec.Command("osascript", "-e", `POSIX path of (choose file with prompt "`+req.Title+`")`)
		out, runErr := cmd.Output()
		if runErr == nil {
			selectedPath = strings.TrimSpace(string(out))
		}
	} else {
		cmd := exec.Command("zenity", "--file-selection", "--title="+req.Title)
		out, runErr := cmd.Output()
		if runErr == nil {
			selectedPath = strings.TrimSpace(string(out))
		}
	}

	if err != nil && selectedPath == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"path":    "",
			"message": "Диалог отменен или недоступен: " + err.Error(),
		})
		return
	}

	if selectedPath == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success":   true,
			"cancelled": true,
			"path":      "",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"cancelled": false,
		"path":      selectedPath,
	})
}

func (a *AdminAPI) handleUpdateServiceFull(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID             string   `json:"id"`
		Name           string   `json:"name"`
		Slug           string   `json:"slug"`
		Description    string   `json:"description"`
		Port           int      `json:"port"`
		ExecutablePath string   `json:"executable_path"`
		ExecArgs       []string `json:"exec_args"`
		WorkingDir     string   `json:"working_dir"`
		AutoStart      bool     `json:"auto_start"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.services.UpdateFull(req.ID, req.Name, req.Slug, req.Description, req.Port, req.ExecutablePath, req.ExecArgs, req.WorkingDir, req.AutoStart); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.LogAudit(domain.AuditActionServiceUpdated, "admin", req.ID, "Full service configuration updated")
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (a *AdminAPI) handleDeleteService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceID string `json:"service_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_ = a.procManager.StopService(req.ServiceID)

	if err := a.services.Unregister(req.ServiceID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	a.LogAudit(domain.AuditActionServiceDisabled, "admin", req.ServiceID, "Service deleted from registry")
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *AdminAPI) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServerName            string                  `json:"server_name"`
		ListenAddr            string                  `json:"listen_addr"`
		AdminAddr             string                  `json:"admin_addr"`
		ProxyTimeoutSecs      int                     `json:"proxy_timeout_seconds"`
		TelegramBotToken      string                  `json:"telegram_bot_token"`
		TelegramBotEnabled    bool                    `json:"telegram_bot_enabled"`
		TelegramAdminChatIDs  []int64                 `json:"telegram_admin_chat_ids"`
		TelegramAPIEndpoint   string                  `json:"telegram_api_endpoint"`
		TelegramMaxFileSizeMB int                     `json:"telegram_max_file_size_mb"`
		RelayNodes            []config.RelayNodeEntry `json:"relay_nodes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.ServerName != "" {
		a.cfg.ServerName = req.ServerName
	}
	if req.ListenAddr != "" {
		a.cfg.ListenAddr = req.ListenAddr
	}
	if req.AdminAddr != "" {
		a.cfg.AdminAddr = req.AdminAddr
	}
	if req.ProxyTimeoutSecs > 0 {
		a.cfg.ProxyTimeoutSecs = req.ProxyTimeoutSecs
	}
	if req.TelegramBotToken != "" {
		a.cfg.TelegramBotToken = req.TelegramBotToken
		a.adminBot.SetBotToken(req.TelegramBotToken)
	}
	a.cfg.TelegramBotEnabled = req.TelegramBotEnabled
	if len(req.TelegramAdminChatIDs) > 0 {
		a.cfg.TelegramAdminChatIDs = req.TelegramAdminChatIDs
		a.adminBot.SetAuthorizedAdmins(req.TelegramAdminChatIDs)
	}
	if req.TelegramAPIEndpoint != "" {
		a.cfg.TelegramAPIEndpoint = req.TelegramAPIEndpoint
	}
	if req.TelegramMaxFileSizeMB > 0 {
		a.cfg.TelegramMaxFileSizeMB = req.TelegramMaxFileSizeMB
	}
	if len(req.RelayNodes) > 0 {
		a.cfg.RelayNodes = req.RelayNodes
	}

	a.LogAudit(domain.AuditActionServiceUpdated, "admin", "config", "Server and Telegram settings updated: "+a.cfg.ServerName)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "updated",
		"config": a.cfg,
	})
}

func (a *AdminAPI) handleEnrollDevice(w http.ResponseWriter, r *http.Request) {
	var dev domain.Device
	if err := json.NewDecoder(r.Body).Decode(&dev); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if dev.Algorithm == "" {
		dev.Algorithm = "Ed25519"
	}
	if dev.Platform == "" {
		dev.Platform = domain.PlatformWindows
	}
	if dev.Architecture == "" {
		dev.Architecture = domain.ArchX86_64
	}

	if err := a.devices.Enroll(&dev); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	a.LogAudit(domain.AuditActionDeviceEnrolled, "admin", dev.ID, "Device enrolled: "+dev.Label)
	writeJSON(w, http.StatusCreated, dev)
}

func (a *AdminAPI) handleUpdateDeviceStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string              `json:"device_id"`
		Status   domain.DeviceStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := a.devices.UpdateStatus(req.DeviceID, req.Status); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	a.LogAudit(domain.AuditActionDeviceActivated, "admin", req.DeviceID, "Device status updated to "+string(req.Status))
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (a *AdminAPI) handleServeDashboardUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(dashboardHTML)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

