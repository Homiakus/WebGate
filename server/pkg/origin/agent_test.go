package origin

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/relay"
)

func TestOriginAgentRejectsNonLoopbackTarget(t *testing.T) {
	_, err := NewOriginReverseAgent(AgentConfig{
		TargetDataAddr: "192.168.1.50:8788",
		Relays: []RelayTarget{
			{ID: "relay-1", Address: "127.0.0.1", Port: 43111},
		},
	})
	if err == nil {
		t.Fatal("expected error for non-loopback target, got nil")
	}
}

func TestOriginAgentDualRelayEndToEnd(t *testing.T) {
	token := "dual-relay-test-token"

	// Start Mock Target Service (Local Data Gateway)
	dataLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start data listener: %v", err)
	}
	defer dataLn.Close()

	go func() {
		for {
			conn, err := dataLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 1024)
				n, err := c.Read(buf)
				if err != nil && err != io.EOF {
					return
				}
				if n > 0 {
					_, _ = c.Write(append([]byte("GW-ACK:"), buf[:n]...))
				}
			}(conn)
		}
	}()

	// Start Relay A
	relayA, err := relay.NewRelayServer(relay.Config{
		ControlAddr:  "127.0.0.1:0",
		ClientAddr:   "127.0.0.1:0",
		ClusterToken: token,
	})
	if err != nil {
		t.Fatalf("relay A init failed: %v", err)
	}
	if err := relayA.Start(); err != nil {
		t.Fatalf("relay A start failed: %v", err)
	}
	defer relayA.Stop()

	// Start Relay B
	relayB, err := relay.NewRelayServer(relay.Config{
		ControlAddr:  "127.0.0.1:0",
		ClientAddr:   "127.0.0.1:0",
		ClusterToken: token,
	})
	if err != nil {
		t.Fatalf("relay B init failed: %v", err)
	}
	if err := relayB.Start(); err != nil {
		t.Fatalf("relay B start failed: %v", err)
	}
	defer relayB.Stop()

	ctrlAPort := relayA.ControlAddr().(*net.TCPAddr).Port
	ctrlBPort := relayB.ControlAddr().(*net.TCPAddr).Port

	// Start Origin Agent connecting to both Relay A and Relay B
	agent, err := NewOriginReverseAgent(AgentConfig{
		ClusterID:         "test-cluster",
		OriginID:          "origin-alpha",
		DefaultToken:      token,
		TargetDataAddr:    dataLn.Addr().String(),
		ReconnectInterval: 500 * time.Millisecond,
		HeartbeatInterval: 1 * time.Second,
		Relays: []RelayTarget{
			{ID: "relay-a", Name: "Relay Alpha", Address: "127.0.0.1", Port: ctrlAPort},
			{ID: "relay-b", Name: "Relay Beta", Address: "127.0.0.1", Port: ctrlBPort},
		},
	})
	if err != nil {
		t.Fatalf("agent init failed: %v", err)
	}
	if err := agent.Start(); err != nil {
		t.Fatalf("agent start failed: %v", err)
	}
	defer agent.Stop()

	// Wait for both relays to be connected
	deadline := time.Now().Add(5 * time.Second)
	for agent.ActiveRelayCount() < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for dual relay connection, active: %d", agent.ActiveRelayCount())
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Test request through Relay A
	testThroughRelay(t, relayA.ClientAddr().String(), "Ping Relay A")

	// Test request through Relay B
	testThroughRelay(t, relayB.ClientAddr().String(), "Ping Relay B")
}

func TestOriginAgentAutoReconnectOnRelayRestart(t *testing.T) {
	token := "reconnect-test-token"

	dataLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start data listener: %v", err)
	}
	defer dataLn.Close()

	// Pick a fixed control port
	tempLn, _ := net.Listen("tcp", "127.0.0.1:0")
	ctrlPort := tempLn.Addr().(*net.TCPAddr).Port
	tempLn.Close()

	relaySrv, err := relay.NewRelayServer(relay.Config{
		ControlAddr:  fmt.Sprintf("127.0.0.1:%d", ctrlPort),
		ClientAddr:   "127.0.0.1:0",
		ClusterToken: token,
	})
	if err != nil {
		t.Fatalf("relay init failed: %v", err)
	}
	if err := relaySrv.Start(); err != nil {
		t.Fatalf("relay start failed: %v", err)
	}

	agent, err := NewOriginReverseAgent(AgentConfig{
		ClusterID:         "test-cluster",
		OriginID:          "origin-reconnect",
		DefaultToken:      token,
		TargetDataAddr:    dataLn.Addr().String(),
		ReconnectInterval: 100 * time.Millisecond,
		HeartbeatInterval: 500 * time.Millisecond,
		Relays: []RelayTarget{
			{ID: "relay-1", Address: "127.0.0.1", Port: ctrlPort},
		},
	})
	if err != nil {
		t.Fatalf("agent init failed: %v", err)
	}
	if err := agent.Start(); err != nil {
		t.Fatalf("agent start failed: %v", err)
	}
	defer agent.Stop()

	// Wait for initial connection
	deadline := time.Now().Add(3 * time.Second)
	for agent.ActiveRelayCount() < 1 {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for initial connection")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Stop relay (simulate crash/restart)
	relaySrv.Stop()
	time.Sleep(200 * time.Millisecond)
	if agent.ActiveRelayCount() != 0 {
		t.Fatalf("expected 0 active relays after server stop, got %d", agent.ActiveRelayCount())
	}

	// Restart relay on same control port
	restartedRelay, err := relay.NewRelayServer(relay.Config{
		ControlAddr:  fmt.Sprintf("127.0.0.1:%d", ctrlPort),
		ClientAddr:   "127.0.0.1:0",
		ClusterToken: token,
	})
	if err != nil {
		t.Fatalf("restart relay init failed: %v", err)
	}
	if err := restartedRelay.Start(); err != nil {
		t.Fatalf("restart relay start failed: %v", err)
	}
	defer restartedRelay.Stop()

	// Agent should auto-reconnect
	reconnectDeadline := time.Now().Add(5 * time.Second)
	for agent.ActiveRelayCount() < 1 {
		if time.Now().After(reconnectDeadline) {
			t.Fatal("timeout waiting for agent to auto-reconnect after relay restart")
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !agent.IsRelayConnected("relay-1") {
		t.Fatal("expected relay-1 to be marked connected")
	}
}

func testThroughRelay(t *testing.T, clientAddr string, msg string) {
	conn, err := net.DialTimeout("tcp", clientAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to dial relay client addr %s: %v", clientAddr, err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(msg)); err != nil {
		t.Fatalf("failed to write to relay: %v", err)
	}

	buf := make([]byte, 1024)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read from relay: %v", err)
	}

	expected := fmt.Sprintf("GW-ACK:%s", msg)
	if !bytes.Equal(buf[:n], []byte(expected)) {
		t.Fatalf("expected '%s', got '%s'", expected, string(buf[:n]))
	}
}
