package gateway_test

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/auth"
	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/gateway"
	"github.com/Homiakus/WebGate/server/pkg/origin"
	"github.com/Homiakus/WebGate/server/pkg/registry"
	"github.com/Homiakus/WebGate/server/pkg/relay"
)

// TestRealEndToEndQualification verifies the complete WebGate qualified runtime path:
// Client -> Relay (A/B) -> Persistent Reverse Origin Agent -> WebGate Data Gateway -> Local Protected Service.
func TestRealEndToEndQualification(t *testing.T) {
	clusterToken := "e2e-qualified-cluster-token-2026"

	// 1. Local Protected Backend Service (Loopback HTTP Server)
	backendReceivedHeaders := make(map[string]string)
	var headerMu sync.Mutex

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headerMu.Lock()
		for k, v := range r.Header {
			if len(v) > 0 {
				backendReceivedHeaders[k] = v[0]
			}
		}
		headerMu.Unlock()

		// Internal session token must never leak into protected upstream backend service
		if r.Header.Get("X-WebGate-Session") != "" {
			http.Error(w, "security violation: X-WebGate-Session leaked to upstream", http.StatusBadGateway)
			return
		}

		switch r.URL.Path {
		case "/api/data":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"success","payload":"corporate-e2e-payload-v1"}`))
		case "/api/echo":
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(append([]byte("ECHO:"), body...))
		default:
			http.NotFound(w, r)
		}
	}))
	defer backendServer.Close()

	// 2. Registries & Authorizer Setup
	svcReg := registry.NewServiceRegistry()
	err := svcReg.Register(&domain.ProtectedService{
		ID:          "svc_corp_crm",
		WorkspaceID: "ws_corp",
		Slug:        "corp-crm",
		Name:        "Corporate CRM Service",
		UpstreamURL: backendServer.URL,
		Status:      domain.ServiceStatusActive,
	})
	if err != nil {
		t.Fatalf("failed to register service: %v", err)
	}

	devReg := registry.NewDeviceRegistry()
	enrollAndActivateTestDevice(t, devReg, "dev_qualified_01", "user_alice")

	authorizer := auth.NewSecureAccessAuthorizer()
	authorizer.RegisterSession(&auth.UserSession{
		SessionID: "sess_qualified_alice",
		UserID:    "user_alice",
		DeviceID:  "dev_qualified_01",
		ExpiresAt: time.Now().UTC().Add(2 * time.Hour),
	})
	authorizer.SetMembership("user_alice", "ws_corp", domain.PermView|domain.PermEdit|domain.PermUpload)

	// 3. WebGate Data Gateway Listener
	gw := gateway.NewServerGateway(svcReg, devReg, authorizer, gateway.GatewayConfig{
		ProxyTimeout: 5 * time.Second,
	})

	gwListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create data gateway listener: %v", err)
	}
	defer gwListener.Close()

	httpServer := &http.Server{
		Handler:      gw,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	go func() { _ = httpServer.Serve(gwListener) }()
	defer func() { _ = httpServer.Close() }()

	// 4. Relay A (Primary) and Relay B (Fallback) Transit Nodes
	relayA, err := relay.NewRelayServer(relay.Config{
		ControlAddr:  "127.0.0.1:0",
		ClientAddr:   "127.0.0.1:0",
		ClusterToken: clusterToken,
		IdleTimeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("relay A init error: %v", err)
	}
	if err := relayA.Start(); err != nil {
		t.Fatalf("relay A start error: %v", err)
	}
	defer relayA.Stop()

	relayB, err := relay.NewRelayServer(relay.Config{
		ControlAddr:  "127.0.0.1:0",
		ClientAddr:   "127.0.0.1:0",
		ClusterToken: clusterToken,
		IdleTimeout:  10 * time.Second,
	})
	if err != nil {
		t.Fatalf("relay B init error: %v", err)
	}
	if err := relayB.Start(); err != nil {
		t.Fatalf("relay B start error: %v", err)
	}
	defer relayB.Stop()

	relayAPort := relayA.ControlAddr().(*net.TCPAddr).Port
	relayBPort := relayB.ControlAddr().(*net.TCPAddr).Port

	// 5. Origin Reverse Agent (operating without inbound NAT/ports)
	originAgent, err := origin.NewOriginReverseAgent(origin.AgentConfig{
		ClusterID:         "webgate-e2e-cluster",
		OriginID:          "origin-e2e-node",
		DefaultToken:      clusterToken,
		TargetDataAddr:    gwListener.Addr().String(),
		ReconnectInterval: 200 * time.Millisecond,
		HeartbeatInterval: 500 * time.Millisecond,
		Relays: []origin.RelayTarget{
			{ID: "relay-a", Name: "Relay Alpha", Address: "127.0.0.1", Port: relayAPort},
			{ID: "relay-b", Name: "Relay Beta", Address: "127.0.0.1", Port: relayBPort},
		},
	})
	if err != nil {
		t.Fatalf("origin agent init error: %v", err)
	}
	if err := originAgent.Start(); err != nil {
		t.Fatalf("origin agent start error: %v", err)
	}
	defer originAgent.Stop()

	// Wait briefly for reverse sessions to establish
	time.Sleep(150 * time.Millisecond)
	if relayA.ConnectedOrigins() == 0 || relayB.ConnectedOrigins() == 0 {
		t.Fatalf("expected origin connected to both relays; A=%d, B=%d",
			relayA.ConnectedOrigins(), relayB.ConnectedOrigins())
	}

	clientAddrA := relayA.ClientAddr().String()
	clientAddrB := relayB.ClientAddr().String()

	// Helper to send HTTP request over raw TCP connection to Relay
	sendHTTPRequest := func(relayClientAddr, reqStr string) (string, int, string, error) {
		conn, err := net.DialTimeout("tcp", relayClientAddr, 2*time.Second)
		if err != nil {
			return "", 0, "", fmt.Errorf("dial relay client: %w", err)
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

		if _, err := conn.Write([]byte(reqStr)); err != nil {
			return "", 0, "", fmt.Errorf("write request: %w", err)
		}

		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			return "", 0, "", fmt.Errorf("read response: %w", err)
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", resp.StatusCode, "", fmt.Errorf("read body: %w", err)
		}

		return resp.Status, resp.StatusCode, string(bodyBytes), nil
	}

	t.Run("PrimaryRelay_EndToEnd_AuthorizedRequest_Succeeds", func(t *testing.T) {
		req := "GET /svc/corp-crm/api/data HTTP/1.1\r\n" +
			"Host: 127.0.0.1\r\n" +
			"X-WebGate-Session: sess_qualified_alice\r\n" +
			"X-WebGate-Device: dev_qualified_01\r\n" +
			"Connection: close\r\n\r\n"

		status, code, body, err := sendHTTPRequest(clientAddrA, req)
		if err != nil {
			t.Fatalf("e2e request failed: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected 200 OK, got code %d, status %s, body: %s", code, status, body)
		}
		if !strings.Contains(body, "corporate-e2e-payload-v1") {
			t.Fatalf("unexpected body content: %s", body)
		}
	})

	t.Run("PrimaryRelay_EndToEnd_POST_Payload_Echo_Succeeds", func(t *testing.T) {
		payload := "confidential-report-data-42"
		req := fmt.Sprintf("POST /svc/corp-crm/api/echo HTTP/1.1\r\n"+
			"Host: 127.0.0.1\r\n"+
			"X-WebGate-Session: sess_qualified_alice\r\n"+
			"X-WebGate-Device: dev_qualified_01\r\n"+
			"Content-Length: %d\r\n"+
			"Connection: close\r\n\r\n%s", len(payload), payload)

		status, code, body, err := sendHTTPRequest(clientAddrA, req)
		if err != nil {
			t.Fatalf("echo request failed: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected 200 OK, got code %d, status %s, body: %s", code, status, body)
		}
		if body != "ECHO:"+payload {
			t.Fatalf("expected ECHO:%s, got: %s", payload, body)
		}
	})

	t.Run("FallbackRelay_EndToEnd_AuthorizedRequest_Succeeds", func(t *testing.T) {
		req := "GET /svc/corp-crm/api/data HTTP/1.1\r\n" +
			"Host: 127.0.0.1\r\n" +
			"X-WebGate-Session: sess_qualified_alice\r\n" +
			"X-WebGate-Device: dev_qualified_01\r\n" +
			"Connection: close\r\n\r\n"

		status, code, body, err := sendHTTPRequest(clientAddrB, req)
		if err != nil {
			t.Fatalf("e2e fallback request failed: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected 200 OK on fallback, got code %d, status %s, body: %s", code, status, body)
		}
		if !strings.Contains(body, "corporate-e2e-payload-v1") {
			t.Fatalf("unexpected body on fallback: %s", body)
		}
	})

	t.Run("Unauthorized_MissingSession_FailsClosed_401", func(t *testing.T) {
		req := "GET /svc/corp-crm/api/data HTTP/1.1\r\n" +
			"Host: 127.0.0.1\r\n" +
			"Connection: close\r\n\r\n"

		_, code, _, err := sendHTTPRequest(clientAddrA, req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got: %d", code)
		}
	})

	t.Run("Unauthorized_UnknownDevice_FailsClosed_403", func(t *testing.T) {
		req := "GET /svc/corp-crm/api/data HTTP/1.1\r\n" +
			"Host: 127.0.0.1\r\n" +
			"X-WebGate-Session: sess_qualified_alice\r\n" +
			"X-WebGate-Device: dev_unknown_attacker\r\n" +
			"Connection: close\r\n\r\n"

		_, code, _, err := sendHTTPRequest(clientAddrA, req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden, got: %d", code)
		}
	})

	t.Run("NonExistent_ServiceSlug_Returns_404", func(t *testing.T) {
		req := "GET /svc/non-existent-slug/api/data HTTP/1.1\r\n" +
			"Host: 127.0.0.1\r\n" +
			"X-WebGate-Session: sess_qualified_alice\r\n" +
			"X-WebGate-Device: dev_qualified_01\r\n" +
			"Connection: close\r\n\r\n"

		_, code, _, err := sendHTTPRequest(clientAddrA, req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got: %d", code)
		}
	})

	t.Run("Disabled_Service_FailsClosed_503", func(t *testing.T) {
		_ = svcReg.UpdateStatus("svc_corp_crm", domain.ServiceStatusDisabled)
		defer func() { _ = svcReg.UpdateStatus("svc_corp_crm", domain.ServiceStatusActive) }()

		req := "GET /svc/corp-crm/api/data HTTP/1.1\r\n" +
			"Host: 127.0.0.1\r\n" +
			"X-WebGate-Session: sess_qualified_alice\r\n" +
			"X-WebGate-Device: dev_qualified_01\r\n" +
			"Connection: close\r\n\r\n"

		_, code, _, err := sendHTTPRequest(clientAddrA, req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 Service Unavailable, got: %d", code)
		}
	})

	t.Run("Concurrent_Multiplexed_Traffic_NoCrosstalk", func(t *testing.T) {
		const concurrentCount = 15
		var wg sync.WaitGroup
		errs := make(chan error, concurrentCount)

		for i := 0; i < concurrentCount; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				relayAddr := clientAddrA
				if idx%2 == 1 {
					relayAddr = clientAddrB
				}
				msg := fmt.Sprintf("concurrent-msg-thread-%d", idx)
				req := fmt.Sprintf("POST /svc/corp-crm/api/echo HTTP/1.1\r\n"+
					"Host: 127.0.0.1\r\n"+
					"X-WebGate-Session: sess_qualified_alice\r\n"+
					"X-WebGate-Device: dev_qualified_01\r\n"+
					"Content-Length: %d\r\n"+
					"Connection: close\r\n\r\n%s", len(msg), msg)

				_, code, body, err := sendHTTPRequest(relayAddr, req)
				if err != nil {
					errs <- fmt.Errorf("worker %d error: %w", idx, err)
					return
				}
				if code != http.StatusOK {
					errs <- fmt.Errorf("worker %d returned status %d", idx, code)
					return
				}
				if body != "ECHO:"+msg {
					errs <- fmt.Errorf("worker %d expected 'ECHO:%s', got '%s'", idx, msg, body)
					return
				}
			}(i)
		}

		wg.Wait()
		close(errs)

		for err := range errs {
			t.Fatalf("concurrent traffic error: %v", err)
		}
	})
}
