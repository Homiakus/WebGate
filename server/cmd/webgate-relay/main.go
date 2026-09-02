package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/relay"
)

var (
	Version   = "1.0.0"
	GitCommit = "unknown"
)

func main() {
	controlAddr := flag.String("control-addr", "0.0.0.0:43211", "Listen address for Origin reverse connections")
	clientAddr := flag.String("client-addr", "0.0.0.0:43111", "Listen address for client transit connections")
	clusterToken := flag.String("cluster-token", os.Getenv("WEBGATE_RELAY_CLUSTER_TOKEN"), "Secret cluster token for Origin authentication")
	flag.Parse()

	if *clusterToken == "" {
		log.Fatal("[Relay] Error: -cluster-token or WEBGATE_RELAY_CLUSTER_TOKEN environment variable is required")
	}

	log.Println("───────────────────────────────────────────────────────────────────────────")
	log.Printf(" WEBGATE TRANSIT RELAY NODE (v%s, commit: %s)\n", Version, GitCommit)
	log.Println("───────────────────────────────────────────────────────────────────────────")

	srv, err := relay.NewRelayServer(relay.Config{
		ControlAddr:  *controlAddr,
		ClientAddr:   *clientAddr,
		ClusterToken: *clusterToken,
		IdleTimeout:  30 * time.Second,
	})
	if err != nil {
		log.Fatalf("[Relay] Ошибка инициализации: %v", err)
	}

	if err := srv.Start(); err != nil {
		log.Fatalf("[Relay] Ошибка запуска: %v", err)
	}

	log.Printf("[Relay] Control listener (для Origin): %s", srv.ControlAddr())
	log.Printf("[Relay] Client listener (для Клиентов): %s", srv.ClientAddr())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println()
	log.Println("[Relay] Завершение работы...")
	srv.Stop()
	log.Println("[Relay] Узел остановлен.")
}
