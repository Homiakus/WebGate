package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/admin"
	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/config"
	"github.com/Homiakus/WebGate/server/pkg/delivery"
	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/gateway"
	"github.com/Homiakus/WebGate/server/pkg/persistence"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

const (
	defaultAuthorityEndpoint = "http://127.0.0.1:8790"
	defaultStateDBPath       = "data/webgate-state.db"
)

func main() {
	configPath := flag.String("config", "", "Path to server configuration file (.toml / .json)")
	flag.StringVar(configPath, "c", "", "Path to server configuration file (shorthand)")
	stateDBPath := flag.String("state-db", stateDBPathFromEnvironment(), "Path to durable WebGate SQLite state")
	flag.StringVar(stateDBPath, "state", stateDBPathFromEnvironment(), "Path to durable WebGate SQLite state (shorthand)")
	backupStatePath := flag.String("backup-state", "", "Create a validated SQLite state snapshot at this path and exit")
	restoreStatePath := flag.String("restore-state", "", "Restore a validated SQLite snapshot into --state-db and exit; target must not already exist")
	flag.Parse()

	log.Println("───────────────────────────────────────────────────────────────────────────")
	log.Println(" 01 / WEBGATE СЕРВЕРНЫЙ ШЛЮЗ И ПАНЕЛЬ УПРАВЛЕНИЯ")
	log.Println("───────────────────────────────────────────────────────────────────────────")

	if strings.TrimSpace(*restoreStatePath) != "" {
		if err := persistence.RestoreSQLiteBackup(*restoreStatePath, *stateDBPath); err != nil {
			log.Fatalf("[Состояние] Restore отклонён: %v", err)
		}
		log.Printf("[Состояние] Restore завершён: %s -> %s", *restoreStatePath, *stateDBPath)
		return
	}

	stateDBExisted, err := stateFileExists(*stateDBPath)
	if err != nil {
		log.Fatalf("[Состояние] Не удалось проверить state DB %s: %v", *stateDBPath, err)
	}
	stateStore, err := persistence.OpenSQLiteRegistryStore(*stateDBPath)
	if err != nil {
		log.Fatalf("[Состояние] Не удалось открыть durable state %s: %v", *stateDBPath, err)
	}
	defer stateStore.Close()
	controlStore, err := persistence.OpenSQLiteControlStore(stateStore)
	if err != nil {
		log.Fatalf("[Состояние] Не удалось открыть durable control state: %v", err)
	}
	if strings.TrimSpace(*backupStatePath) != "" {
		if err := controlStore.BackupTo(*backupStatePath); err != nil {
			log.Fatalf("[Состояние] Backup отклонён: %v", err)
		}
		log.Printf("[Состояние] Backup создан: %s", *backupStatePath)
		return
	}

	svcReg := registry.NewServiceRegistryWithPersistence(stateStore)
	devReg := registry.NewDeviceRegistryWithPersistence(stateStore)
	relReg := registry.NewReleaseRegistryWithPersistence(stateStore)
	if err := restoreDurableRegistries(stateStore, svcReg, devReg, relReg); err != nil {
		log.Fatalf("[Состояние] Durable state поврежден или несовместим: %v", err)
	}
	log.Printf("[Состояние] Восстановлено: services=%d devices=%d releases=%d", len(svcReg.List()), len(devReg.List()), len(relReg.List()))

	serverCfg := config.DefaultServerConfig()
	persistedControl, err := controlStore.LoadControlConfig()
	if err != nil {
		log.Fatalf("[Состояние] Durable control config повреждён или несовместим: %v", err)
	}
	if persistedControl != nil {
		config.ApplyDurableSnapshot(serverCfg, persistedControl)
		log.Printf("[Конфиг] Восстановлена durable control-конфигурация: %s", serverCfg.ServerName)
	}
	if *configPath != "" {
		log.Printf("[Конфиг] Привязка внешнего файла настроек: %s\n", *configPath)
		loaded, err := config.LoadConfigFile(*configPath)
		if err != nil {
			log.Fatalf("[Конфиг] Ошибка загрузки %s: %v", *configPath, err)
		}
		serverCfg = loaded
		log.Printf("[Конфиг] Успешно загружен профиль: '%s'", serverCfg.ServerName)
	}
	config.ApplyRuntimeSecrets(serverCfg, os.LookupEnv)

	if err := config.HardenRuntimeAddresses(serverCfg); err != nil {
		log.Fatalf("[Безопасность] Небезопасная конфигурация listener: %v", err)
	}

	// Service definitions bootstrap only a truly new state file. Once a durable
	// database exists, even an intentionally empty service registry is authoritative
	// and must not be silently resurrected from defaults or a config file.
	if shouldBootstrapServices(stateDBExisted, len(svcReg.List())) {
		if err := serverCfg.ApplyToRegistries(svcReg); err != nil {
			log.Fatalf("[Конфиг] Не удалось выполнить initial service bootstrap: %v", err)
		}
		if err := ensureConfiguredServicesPresent(serverCfg, svcReg); err != nil {
			log.Fatalf("[Конфиг] Initial service bootstrap не зафиксирован: %v", err)
		}
	} else if len(serverCfg.Services) > 0 {
		log.Printf("[Конфиг] Durable service registry уже является источником истины; service entries из config не пере-засеиваются")
	}
	if err := controlStore.SaveControlConfig(config.DurableSnapshot(serverCfg)); err != nil {
		log.Fatalf("[Состояние] Не удалось зафиксировать durable control config: %v", err)
	}

	// No synthetic production devices are seeded here. Every real device must be
	// enrolled with a valid public key and activated through proof-of-possession.

	// Release registry starts empty unless a previously qualified release was
	// restored from durable state. Only verified build pipeline outputs may be promoted.

	delSvc := delivery.NewTelegramDeliveryService(relReg)
	// The historical admin prototype still carries an unused legacy-authorizer
	// parameter. T-051 removes it when management authorization is requalified.
	adminAPI := admin.NewAdminAPI(svcReg, devReg, relReg, delSvc, nil)
	adminAPI.InstallConfig(serverCfg)

	serviceAuthorizer, authorityEndpoint, err := serviceAuthorizerFromEnvironment()
	if err != nil {
		log.Fatalf("[Безопасность] Некорректная конфигурация SecureAcces authority: %v", err)
	}
	if authorityEndpoint == "" {
		log.Printf("[Безопасность] SecureAcces provider не настроен: /svc/* работает fail-closed (503)")
	} else {
		log.Printf("[Безопасность] SecureAcces authority подключён через %s", authorityEndpoint)
	}
	gw := gateway.NewServerGateway(svcReg, devReg, serviceAuthorizer, gateway.GatewayConfig{
		ProxyTimeout: time.Duration(serverCfg.ProxyTimeoutSecs) * time.Second,
	})

	dataMux := http.NewServeMux()
	dataMux.Handle("/svc/", gw)
	dataMux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	})

	adminMux := http.NewServeMux()
	adminAPI.RegisterRoutes(adminMux)
	durableAdmin, err := admin.NewDurableAdminHandler(adminAPI, controlStore, adminMux)
	if err != nil {
		log.Fatalf("[Состояние] Не удалось восстановить durable admin state: %v", err)
	}
	if err := durableAdmin.RecordAudit(domain.AuditActionServiceUpdated, "system", "config", "Bound server control config: "+serverCfg.ServerName); err != nil {
		log.Fatalf("[Состояние] Не удалось зафиксировать startup audit: %v", err)
	}
	adminToken := os.Getenv("WEBGATE_ADMIN_TOKEN")
	adminHandler, err := admin.RequireAdminToken(durableAdmin, adminToken)
	if err != nil {
		log.Fatalf("[Безопасность] WEBGATE_ADMIN_TOKEN должен быть случайным секретом не короче 32 байт: %v", err)
	}

	dataListener, err := net.Listen("tcp", serverCfg.ListenAddr)
	if err != nil {
		log.Fatalf("[Data Plane] Не удалось открыть listener %s: %v", serverCfg.ListenAddr, err)
	}
	defer dataListener.Close()

	adminListener, err := net.Listen("tcp", serverCfg.AdminAddr)
	if err != nil {
		log.Fatalf("[Admin Plane] Не удалось открыть listener %s: %v", serverCfg.AdminAddr, err)
	}
	defer adminListener.Close()

	dataServer := hardenedHTTPServer(dataMux)
	adminServer := hardenedHTTPServer(adminHandler)

	log.Printf("[Data Plane] WebGate Gateway слушает http://%s (Gateway: /svc/{slug}/)", dataListener.Addr())
	log.Printf("[Admin Plane] WebGate Admin слушает http://%s/admin (требуется аутентификация)", adminListener.Addr())

	errCh := make(chan error, 2)
	go func() { errCh <- dataServer.Serve(dataListener) }()
	go func() { errCh <- adminServer.Serve(adminListener) }()

	if serveErr := <-errCh; serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		log.Fatalf("WebGate server error: %v", serveErr)
	}
}

func stateDBPathFromEnvironment() string {
	if configured := strings.TrimSpace(os.Getenv("WEBGATE_STATE_DB")); configured != "" {
		return configured
	}
	return defaultStateDBPath
}

func stateFileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func restoreDurableRegistries(
	store *persistence.SQLiteRegistryStore,
	services *registry.ServiceRegistry,
	devices *registry.DeviceRegistry,
	releases *registry.ReleaseRegistry,
) error {
	persistedServices, err := store.LoadServices()
	if err != nil {
		return err
	}
	persistedDevices, err := store.LoadDevices()
	if err != nil {
		return err
	}
	persistedReleases, err := store.LoadReleases()
	if err != nil {
		return err
	}
	if err := services.Restore(persistedServices); err != nil {
		return fmt.Errorf("restore services: %w", err)
	}
	if err := devices.Restore(persistedDevices); err != nil {
		return fmt.Errorf("restore devices: %w", err)
	}
	if err := releases.Restore(persistedReleases); err != nil {
		return fmt.Errorf("restore releases: %w", err)
	}
	return nil
}

func ensureConfiguredServicesPresent(cfg *config.ServerConfig, services *registry.ServiceRegistry) error {
	for _, configured := range cfg.Services {
		if _, err := services.GetByID(configured.ID); err != nil {
			return fmt.Errorf("service %q: %w", configured.ID, err)
		}
	}
	return nil
}

func serviceAuthorizerFromEnvironment() (auth.ServiceAuthorizer, string, error) {
	endpoint := strings.TrimSpace(os.Getenv("WEBGATE_AUTHORITY_URL"))
	bridgeToken := os.Getenv("WEBGATE_AUTHORITY_TOKEN")

	if endpoint == "" && bridgeToken == "" {
		return auth.NewUnavailableServiceAuthorizer(), "", nil
	}
	if bridgeToken == "" {
		return nil, "", fmt.Errorf("WEBGATE_AUTHORITY_TOKEN is required when authority is explicitly configured")
	}
	if endpoint == "" {
		endpoint = defaultAuthorityEndpoint
	}
	authorizer, err := auth.NewRemoteServiceAuthorizer(auth.RemoteServiceAuthorizerConfig{
		Endpoint:    endpoint,
		BridgeToken: bridgeToken,
		Timeout:     2 * time.Second,
	})
	if err != nil {
		return nil, "", err
	}
	return authorizer, endpoint, nil
}

func hardenedHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
}
