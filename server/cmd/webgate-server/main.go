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
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

const defaultAuthorityEndpoint = "http://127.0.0.1:8790"

func main() {
	configPath := flag.String("config", "", "Path to server configuration file (.toml / .json)")
	flag.StringVar(configPath, "c", "", "Path to server configuration file (shorthand)")
	flag.Parse()

	log.Println("───────────────────────────────────────────────────────────────────────────")
	log.Println(" 01 / WEBGATE СЕРВЕРНЫЙ ШЛЮЗ И ПАНЕЛЬ УПРАВЛЕНИЯ")
	log.Println("───────────────────────────────────────────────────────────────────────────")

	svcReg := registry.NewServiceRegistry()
	devReg := registry.NewDeviceRegistry()
	relReg := registry.NewReleaseRegistry()

	serverCfg := config.DefaultServerConfig()
	if *configPath != "" {
		log.Printf("[Конфиг] Привязка внешнего файла настроек: %s\n", *configPath)
		loaded, err := config.LoadConfigFile(*configPath)
		if err != nil {
			log.Fatalf("[Конфиг] Ошибка загрузки %s: %v", *configPath, err)
		}
		serverCfg = loaded
		log.Printf("[Конфиг] Успешно загружен профиль: '%s' с %d маршрутами upstream\n", serverCfg.ServerName, len(serverCfg.Services))
	}

	if err := config.HardenRuntimeAddresses(serverCfg); err != nil {
		log.Fatalf("[Безопасность] Небезопасная конфигурация listener: %v", err)
	}
	if err := serverCfg.ApplyToRegistries(svcReg); err != nil {
		log.Fatalf("[Конфиг] Не удалось применить реестр сервисов: %v", err)
	}

	// No synthetic production devices are seeded here. Every real device must be
	// enrolled with a valid public key and activated through proof-of-possession.

	// Release registry starts empty. Only verified build pipeline outputs may be promoted.

	delSvc := delivery.NewTelegramDeliveryService(relReg)
	// The historical admin prototype still carries an unused legacy-authorizer
	// parameter. T-051 removes it when management authorization is requalified.
	adminAPI := admin.NewAdminAPI(svcReg, devReg, relReg, delSvc, nil)
	adminAPI.SetConfig(serverCfg)
	adminAPI.LogAudit(domain.AuditActionServiceCreated, "system", "config", "Bound server config: "+serverCfg.ServerName)

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
	adminToken := os.Getenv("WEBGATE_ADMIN_TOKEN")
	adminHandler, err := admin.RequireAdminToken(adminMux, adminToken)
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
