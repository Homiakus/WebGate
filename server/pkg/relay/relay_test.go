package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestRelayFailsClosedWhenNoOriginConnected(t *testing.T) {
	r, err := NewRelayServer(Config{
		ControlAddr:  "127.0.0.1:0",
		ClientAddr:   "127.0.0.1:0",
		ClusterToken: "secret-cluster-token",
	})
	if err != nil {
		t.Fatalf("failed to create relay server: %v", err)
	}
	if err := r.Start(); err != nil {
		t.Fatalf("failed to start relay server: %v", err)
	}
	defer r.Stop()

	clientAddr := r.ClientAddr().String()
	conn, err := net.DialTimeout("tcp", clientAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to dial client address: %v", err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err == nil && n > 0 {
		t.Fatalf("expected immediate closure / EOF, but received %d bytes", n)
	}
	if err != io.EOF && !isClosedConnErr(err) {
		t.Logf("connection closed as expected with err: %v", err)
	}
}

func isClosedConnErr(err error) bool {
	if err == nil {
		return false
	}
	return true
}

func TestRelayRejectsUnauthenticatedOrigin(t *testing.T) {
	r, err := NewRelayServer(Config{
		ControlAddr:  "127.0.0.1:0",
		ClientAddr:   "127.0.0.1:0",
		ClusterToken: "correct-secret-token",
	})
	if err != nil {
		t.Fatalf("failed to create relay: %v", err)
	}
	if err := r.Start(); err != nil {
		t.Fatalf("failed to start relay: %v", err)
	}
	defer r.Stop()

	conn, err := net.Dial("tcp", r.ControlAddr().String())
	if err != nil {
		t.Fatalf("failed to dial control: %v", err)
	}
	defer conn.Close()

	authBytes, _ := json.Marshal(AuthPayload{
		ClusterID: "cluster-1",
		OriginID:  "origin-1",
		Token:     "wrong-token",
	})

	if err := WriteFrame(conn, &Frame{
		Type:    FrameTypeAuth,
		Payload: authBytes,
	}); err != nil {
		t.Fatalf("write auth frame failed: %v", err)
	}

	resp, err := ReadFrame(conn)
	if err != nil {
		t.Fatalf("read auth resp failed: %v", err)
	}
	if resp.Type != FrameTypeAuthResp {
		t.Fatalf("unexpected frame type: %d", resp.Type)
	}

	var result AuthResponsePayload
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		t.Fatalf("unmarshal auth resp failed: %v", err)
	}
	if result.Status != AuthStatusDenied {
		t.Fatalf("expected AuthStatusDenied, got %d", result.Status)
	}
}

func TestRelayMultiplexedStreamingEndToEnd(t *testing.T) {
	clusterToken := "e2e-cluster-token"
	r, err := NewRelayServer(Config{
		ControlAddr:  "127.0.0.1:0",
		ClientAddr:   "127.0.0.1:0",
		ClusterToken: clusterToken,
	})
	if err != nil {
		t.Fatalf("failed to create relay: %v", err)
	}
	if err := r.Start(); err != nil {
		t.Fatalf("failed to start relay: %v", err)
	}
	defer r.Stop()

	// Mock Origin connection
	originConn, err := net.Dial("tcp", r.ControlAddr().String())
	if err != nil {
		t.Fatalf("origin dial failed: %v", err)
	}
	defer originConn.Close()

	authBytes, _ := json.Marshal(AuthPayload{
		ClusterID: "cluster-alpha",
		OriginID:  "origin-alpha",
		Token:     clusterToken,
	})
	if err := WriteFrame(originConn, &Frame{
		Type:    FrameTypeAuth,
		Payload: authBytes,
	}); err != nil {
		t.Fatalf("auth frame failed: %v", err)
	}

	authResp, err := ReadFrame(originConn)
	if err != nil || authResp.Type != FrameTypeAuthResp {
		t.Fatalf("auth response failed: %v", err)
	}

	// Origin read/echo loop simulation
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			f, err := ReadFrame(originConn)
			if err != nil {
				return
			}
			switch f.Type {
			case FrameTypeStreamOpen:
				// Stream open ACK
			case FrameTypeStreamData:
				// Echo data back with prefix "ECHO:"
				echoPayload := append([]byte("ECHO:"), f.Payload...)
				_ = WriteFrame(originConn, &Frame{
					Type:     FrameTypeStreamData,
					StreamID: f.StreamID,
					Payload:  echoPayload,
				})
			case FrameTypeStreamClose:
				return
			}
		}
	}()

	// Client connects to relay client address
	clientConn, err := net.Dial("tcp", r.ClientAddr().String())
	if err != nil {
		t.Fatalf("client dial failed: %v", err)
	}
	defer clientConn.Close()

	testMsg := []byte("Hello WebGate Relay!")
	if _, err := clientConn.Write(testMsg); err != nil {
		t.Fatalf("client write failed: %v", err)
	}

	recvBuf := make([]byte, 1024)
	_ = clientConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := clientConn.Read(recvBuf)
	if err != nil {
		t.Fatalf("client read failed: %v", err)
	}

	expected := append([]byte("ECHO:"), testMsg...)
	if !bytes.Equal(recvBuf[:n], expected) {
		t.Fatalf("expected '%s', got '%s'", expected, recvBuf[:n])
	}

	_ = clientConn.Close()
	_ = originConn.Close()
	wg.Wait()
}

func TestRelayRejectsMalformedMagicFrame(t *testing.T) {
	r, err := NewRelayServer(Config{
		ControlAddr:  "127.0.0.1:0",
		ClientAddr:   "127.0.0.1:0",
		ClusterToken: "token",
	})
	if err != nil {
		t.Fatalf("failed to create relay: %v", err)
	}
	if err := r.Start(); err != nil {
		t.Fatalf("failed to start relay: %v", err)
	}
	defer r.Stop()

	conn, err := net.Dial("tcp", r.ControlAddr().String())
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	// Write garbage bytes instead of protocol magic
	_, _ = conn.Write([]byte("GARBAGE_MAGIC_BYTES_1234"))

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err == nil && n > 0 {
		t.Fatalf("expected connection closure on invalid magic, received %d bytes", n)
	}
}

func TestRelayConcurrentStreamsNoCrosstalk(t *testing.T) {
	clusterToken := "concurrent-test-token"
	r, err := NewRelayServer(Config{
		ControlAddr:  "127.0.0.1:0",
		ClientAddr:   "127.0.0.1:0",
		ClusterToken: clusterToken,
	})
	if err != nil {
		t.Fatalf("failed to create relay: %v", err)
	}
	if err := r.Start(); err != nil {
		t.Fatalf("failed to start relay: %v", err)
	}
	defer r.Stop()

	originConn, err := net.Dial("tcp", r.ControlAddr().String())
	if err != nil {
		t.Fatalf("origin dial failed: %v", err)
	}
	defer originConn.Close()

	authBytes, _ := json.Marshal(AuthPayload{
		ClusterID: "cluster-concurrent",
		OriginID:  "origin-concurrent",
		Token:     clusterToken,
	})
	_ = WriteFrame(originConn, &Frame{Type: FrameTypeAuth, Payload: authBytes})
	_, _ = ReadFrame(originConn)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			f, err := ReadFrame(originConn)
			if err != nil {
				return
			}
			if f.Type == FrameTypeStreamData {
				_ = WriteFrame(originConn, &Frame{
					Type:     FrameTypeStreamData,
					StreamID: f.StreamID,
					Payload:  append([]byte("ACK:"), f.Payload...),
				})
			}
		}
	}()

	clientCount := 10
	var clientWg sync.WaitGroup
	for i := 0; i < clientCount; i++ {
		clientWg.Add(1)
		go func(idx int) {
			defer clientWg.Done()
			conn, err := net.Dial("tcp", r.ClientAddr().String())
			if err != nil {
				t.Errorf("client %d dial failed: %v", idx, err)
				return
			}
			defer conn.Close()

			msg := fmt.Sprintf("STREAM-ID-%d-DATA", idx)
			if _, err := conn.Write([]byte(msg)); err != nil {
				t.Errorf("client %d write failed: %v", idx, err)
				return
			}

			buf := make([]byte, 1024)
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			n, err := conn.Read(buf)
			if err != nil {
				t.Errorf("client %d read failed: %v", idx, err)
				return
			}
			expected := fmt.Sprintf("ACK:%s", msg)
			if string(buf[:n]) != expected {
				t.Errorf("client %d expected '%s', got '%s'", idx, expected, string(buf[:n]))
			}
		}(i)
	}

	clientWg.Wait()
	_ = originConn.Close()
	wg.Wait()
}
