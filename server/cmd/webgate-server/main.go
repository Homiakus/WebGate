package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/admin"
	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/config"
	"github.com/Homiakus/WebGate/server/pkg/delivery"
	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/gateway"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func main() {
	configPath := flag.String("config", "", "Path to server configuration file (.toml / .json)")
	flag.StringVar(configPath, "c", "", "Path to server configuration file (shorthand)")
	flag.Parse()

	log.Println("───────────────────────────────────────────────────────────────────────────")
	log.Println(" 01 / WEBGATE СЕРВЕРНЫЙ ШЛЮЗ И ПАНЕЛЬ УПРАВЛЕНИЯ")
	log.Println("───────────────────────────────────────────────────────────────────────────")

	// Initialize registries
	svcReg := registry.NewServiceRegistry()
	devReg := registry.NewDeviceRegistry()
	relReg := registry.NewReleaseRegistry()

	// Load configuration
	var serverCfg *config.ServerConfig
	if *configPath != "" {
		log.Printf("[Конфиг] Привязка внешнего файла настроек: %s\n", *configPath)
		loaded, err := config.LoadConfigFile(*configPath)
		if err != nil {
			log.Printf("[Конфиг] Предупреждение: Ошибка загрузки из %s: %v. Применяются настройки по умолчанию.\n", *configPath, err)
			serverCfg = config.DefaultServerConfig()
		} else {
			serverCfg = loaded
			log.Printf("[Конфиг] Успешно загружен профиль: '%s' с %d маршрутами upstream\n", serverCfg.ServerName, len(serverCfg.Services))
		}
	} else {
		serverCfg = config.DefaultServerConfig()
	}

	// Apply configuration services to registry
	if err := serverCfg.ApplyToRegistries(svcReg); err != nil {
		log.Printf("[Конфиг] Предупреждение при применении настроек: %v\n", err)
	}

	// Seed initial devices
	_ = devReg.Enroll(&domain.Device{
		ID:           "dev_station_alpha",
		UserID:       "usr_admin_dave",
		Label:        "Primary Operations Station (Windows 11)",
		Platform:     domain.PlatformWindows,
		Architecture: domain.ArchX86_64,
		PublicKeyHex: "e0a4f5b210c4987163ef908123abcdef",
		Algorithm:    "Ed25519",
	})
	_ = devReg.Enroll(&domain.Device{
		ID:           "dev_field_pixel8",
		UserID:       "usr_engineer_mark",
		Label:        "Field Station Tablet (Android 15)",
		Platform:     domain.PlatformAndroid,
		Architecture: domain.ArchArm64,
		PublicKeyHex: "38bc49df92019ab48172938174abcdef",
		Algorithm:    "Ed25519",
	})

	// Seed initial release v1.0.0
	_ = relReg.AddDraft(&domain.Release{
		Version:      "1.0.0",
		SourceCommit: "7a4c36b",
		Channel:      "stable",
		Artifacts: []domain.PlatformArtifact{
			{
				Platform:     domain.PlatformWindows,
				Architecture: domain.ArchX86_64,
				FileName:     "webgate-app.exe",
				SHA256Hex:    "d3b07384d113edec49eaa6238ad5ff00",
				SizeBytes:    24500000,
			},
			{
				Platform:     domain.PlatformAndroid,
				Architecture: domain.ArchArm64,
				FileName:     "webgate-android.apk",
				SHA256Hex:    "c7b01984d113edec49eaa6238ad5ff11",
				SizeBytes:    18200000,
			},
		},
	})
	_ = relReg.Verify("1.0.0")
	_ = relReg.Promote("1.0.0")

	authorizer := auth.NewSecureAccessAuthorizer()
	delSvc := delivery.NewTelegramDeliveryService(relReg)

	// Admin API & Dashboard
	adminAPI := admin.NewAdminAPI(svcReg, devReg, relReg, delSvc, authorizer)
	adminAPI.SetConfig(serverCfg)

	// Seed audit events
	adminAPI.LogAudit(domain.AuditActionServiceCreated, "system", "config", "Bound server config: "+serverCfg.ServerName)
	adminAPI.LogAudit(domain.AuditActionReleasePromoted, "system", "1.0.0", "Auto-promoted release 1.0.0 to active fleet")

	// Server Gateway
	gw := gateway.NewServerGateway(svcReg, devReg, authorizer, gateway.GatewayConfig{
		ProxyTimeout: time.Duration(serverCfg.ProxyTimeoutSecs) * time.Second,
	})

	mux := http.NewServeMux()
	adminAPI.RegisterRoutes(mux)

	// Gateway traffic handler on /svc/
	mux.Handle("/svc/", gw)

	listenAddr := serverCfg.ListenAddr
	if listenAddr == "" {
		listenAddr = ":8787"
	}

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	log.Printf("WebGate Server listening on http://127.0.0.1%s (Admin Dashboard: /admin, Gateway: /svc/{slug}/)\n", listenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
