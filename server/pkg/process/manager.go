package process

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

var (
	ErrServiceAlreadyRunning = errors.New("service process is already running")
	ErrServiceNotRunning     = errors.New("service process is not running")
	ErrNoExecutableDefined   = errors.New("no executable path configured for service")
)

type ProcessInstance struct {
	ServiceID      string    `json:"service_id"`
	Slug           string    `json:"slug"`
	Port           int       `json:"port"`
	ExecutablePath string    `json:"executable_path"`
	PID            int       `json:"pid"`
	State          string    `json:"state"`
	StartedAt      time.Time `json:"started_at"`
	cmd            *exec.Cmd
	cancelFunc     func()
}

type ProcessManager struct {
	mu         sync.RWMutex
	services   *registry.ServiceRegistry
	processes  map[string]*ProcessInstance
	logs       map[string][]string
	onExitHook func(serviceID string, pid int, exitCode int, err error)
	mockPIDSeq int
}

func NewProcessManager(services *registry.ServiceRegistry) *ProcessManager {
	return &ProcessManager{
		services:   services,
		processes:  make(map[string]*ProcessInstance),
		logs:       make(map[string][]string),
		mockPIDSeq: 54000,
	}
}

func (pm *ProcessManager) SetOnExitHook(hook func(serviceID string, pid int, exitCode int, err error)) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.onExitHook = hook
}

func (pm *ProcessManager) AppendLog(serviceID string, line string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.appendLogLocked(serviceID, line)
}

func (pm *ProcessManager) appendLogLocked(serviceID string, line string) {
	entry := fmt.Sprintf("[%s] %s", time.Now().UTC().Format("15:04:05"), line)
	pm.logs[serviceID] = append(pm.logs[serviceID], entry)
	if len(pm.logs[serviceID]) > 200 {
		pm.logs[serviceID] = pm.logs[serviceID][len(pm.logs[serviceID])-200:]
	}
}

func (pm *ProcessManager) GetRecentLogs(serviceID string, limit int) []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	lines := pm.logs[serviceID]
	if len(lines) == 0 {
		return []string{"(Записи в журнале отсутствуют)"}
	}
	if limit > 0 && len(lines) > limit {
		return lines[len(lines)-limit:]
	}
	return lines
}

func (pm *ProcessManager) StartService(serviceID string) (*ProcessInstance, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	svc, err := pm.services.GetByID(serviceID)
	if err != nil {
		return nil, fmt.Errorf("service not found: %w", err)
	}
	if strings.TrimSpace(svc.ExecutablePath) == "" {
		return nil, ErrNoExecutableDefined
	}
	if existing, running := pm.processes[serviceID]; running && existing.State == string(domain.ProcessStateRunning) {
		return nil, ErrServiceAlreadyRunning
	}

	now := time.Now().UTC()
	var pid int
	var cmd *exec.Cmd

	if svc.ExecutablePath == ":mock" || strings.HasPrefix(svc.ExecutablePath, "sim:") {
		pm.mockPIDSeq++
		pid = pm.mockPIDSeq
	} else {
		cmd = exec.Command(svc.ExecutablePath, svc.ExecArgs...)
		cmd.Env = sanitizedChildEnvironment(os.Environ())
		if svc.WorkingDir != "" {
			cmd.Dir = svc.WorkingDir
		}
		if err := cmd.Start(); err != nil {
			_ = pm.services.UpdateProcessRuntime(serviceID, domain.ProcessStateStopped, 0, nil)
			pm.appendLogLocked(serviceID, "Ошибка запуска процесса: "+err.Error())
			return nil, fmt.Errorf("failed to start service process: %w", err)
		}
		pid = cmd.Process.Pid
	}

	inst := &ProcessInstance{
		ServiceID:      svc.ID,
		Slug:           svc.Slug,
		Port:           svc.Port,
		ExecutablePath: svc.ExecutablePath,
		PID:            pid,
		State:          string(domain.ProcessStateRunning),
		StartedAt:      now,
		cmd:            cmd,
	}

	if err := pm.services.UpdateProcessRuntime(serviceID, domain.ProcessStateRunning, pid, &now); err != nil {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		pm.appendLogLocked(serviceID, "Не удалось зафиксировать состояние запущенного процесса: "+err.Error())
		return nil, fmt.Errorf("failed to record service runtime state: %w", err)
	}

	pm.processes[serviceID] = inst
	pm.appendLogLocked(serviceID, fmt.Sprintf("Процесс запущен на порту %d (PID %d)", svc.Port, pid))

	if cmd != nil {
		go pm.waitForProcess(serviceID, pid, cmd)
	}

	return inst, nil
}

func sanitizedChildEnvironment(parent []string) []string {
	filtered := make([]string, 0, len(parent))
	for _, entry := range parent {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if isWebGateControlSecret(key) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func isWebGateControlSecret(key string) bool {
	return strings.EqualFold(key, "WEBGATE_AUTHORITY_TOKEN") ||
		strings.EqualFold(key, "WEBGATE_ADMIN_TOKEN")
}

func (pm *ProcessManager) waitForProcess(serviceID string, pid int, cmd *exec.Cmd) {
	waitErr := cmd.Wait()
	exitCode := 0
	if waitErr != nil {
		exitCode = 1
	}

	pm.mu.Lock()
	if inst, ok := pm.processes[serviceID]; ok && inst.PID == pid && inst.State == string(domain.ProcessStateRunning) {
		state := domain.ProcessStateStopped
		if waitErr != nil {
			state = domain.ProcessStateCrashed
		}
		inst.State = string(state)
		inst.PID = 0
		_ = pm.services.UpdateProcessRuntime(serviceID, state, 0, nil)
		pm.appendLogLocked(serviceID, fmt.Sprintf("Процесс завершился (PID %d, exit=%d)", pid, exitCode))
	}
	hook := pm.onExitHook
	pm.mu.Unlock()

	if hook != nil {
		hook(serviceID, pid, exitCode, waitErr)
	}
}

func (pm *ProcessManager) StopService(serviceID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	inst, exists := pm.processes[serviceID]
	if !exists || inst.State != string(domain.ProcessStateRunning) {
		return ErrServiceNotRunning
	}

	if inst.cmd != nil && inst.cmd.Process != nil {
		_ = inst.cmd.Process.Kill()
	}

	inst.State = string(domain.ProcessStateStopped)
	inst.PID = 0
	if err := pm.services.UpdateProcessRuntime(serviceID, domain.ProcessStateStopped, 0, nil); err != nil {
		return fmt.Errorf("failed to record stopped service runtime state: %w", err)
	}
	pm.appendLogLocked(serviceID, "Процесс остановлен оператором")

	return nil
}

func (pm *ProcessManager) RestartService(serviceID string) (*ProcessInstance, error) {
	_ = pm.StopService(serviceID)
	return pm.StartService(serviceID)
}

func (pm *ProcessManager) GetProcess(serviceID string) (*ProcessInstance, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	inst, exists := pm.processes[serviceID]
	return inst, exists
}

func (pm *ProcessManager) ListAll() map[string]*ProcessInstance {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	res := make(map[string]*ProcessInstance, len(pm.processes))
	for k, v := range pm.processes {
		res[k] = v
	}
	return res
}
