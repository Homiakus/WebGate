package origin

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Homiakus/WebGate/server/pkg/relay"
)

var (
	ErrNonLoopbackTarget = errors.New("origin: target data gateway must be a loopback address")
	ErrNoRelays          = errors.New("origin: no relay endpoints configured")
)

type RelayTarget struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	Token   string `json:"token,omitempty"`
}

type AgentConfig struct {
	ClusterID         string
	OriginID          string
	DefaultToken      string
	TargetDataAddr    string // Must be literal loopback (e.g. "127.0.0.1:8788")
	Relays            []RelayTarget
	ReconnectInterval time.Duration
	HeartbeatInterval time.Duration
}

type relayWorker struct {
	target       RelayTarget
	agent        *OriginReverseAgent
	connected    int32
	closed       chan struct{}
	closeOnce    sync.Once
	conn         net.Conn
	writeMu      sync.Mutex
	localStreams map[uint32]*localStream
	streamsMu    sync.RWMutex
}

type localStream struct {
	id        uint32
	localConn net.Conn
	dataCh    chan []byte
	closed    chan struct{}
	closeOnce sync.Once
}

func (s *localStream) close() {
	s.closeOnce.Do(func() {
		close(s.closed)
	})
}

type OriginReverseAgent struct {
	cfg       AgentConfig
	workers   []*relayWorker
	closed    chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

func validateLoopback(addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.Trim(host, "[]")
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrNonLoopbackTarget, addr)
}

func NewOriginReverseAgent(cfg AgentConfig) (*OriginReverseAgent, error) {
	if err := validateLoopback(cfg.TargetDataAddr); err != nil {
		return nil, err
	}
	if len(cfg.Relays) == 0 {
		return nil, ErrNoRelays
	}
	if cfg.OriginID == "" {
		cfg.OriginID = "webgate-origin-default"
	}
	if cfg.ClusterID == "" {
		cfg.ClusterID = "webgate-cluster"
	}
	if cfg.ReconnectInterval <= 0 {
		cfg.ReconnectInterval = 2 * time.Second
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 10 * time.Second
	}

	agent := &OriginReverseAgent{
		cfg:    cfg,
		closed: make(chan struct{}),
	}

	for _, target := range cfg.Relays {
		w := &relayWorker{
			target:       target,
			agent:        agent,
			closed:       make(chan struct{}),
			localStreams: make(map[uint32]*localStream),
		}
		agent.workers = append(agent.workers, w)
	}

	return agent, nil
}

func (a *OriginReverseAgent) Start() error {
	for _, w := range a.workers {
		a.wg.Add(1)
		go w.run()
	}
	return nil
}

func (a *OriginReverseAgent) Stop() {
	a.closeOnce.Do(func() {
		close(a.closed)
		for _, w := range a.workers {
			w.stop()
		}
		a.wg.Wait()
	})
}

func (a *OriginReverseAgent) ActiveRelayCount() int {
	count := 0
	for _, w := range a.workers {
		if atomic.LoadInt32(&w.connected) == 1 {
			count++
		}
	}
	return count
}

func (a *OriginReverseAgent) IsRelayConnected(relayID string) bool {
	for _, w := range a.workers {
		if w.target.ID == relayID {
			return atomic.LoadInt32(&w.connected) == 1
		}
	}
	return false
}

func (w *relayWorker) stop() {
	w.closeOnce.Do(func() {
		close(w.closed)
		if w.conn != nil {
			_ = w.conn.Close()
		}
		w.streamsMu.Lock()
		for _, s := range w.localStreams {
			s.close()
		}
		w.localStreams = make(map[uint32]*localStream)
		w.streamsMu.Unlock()
	})
}

func (w *relayWorker) sendFrame(f *relay.Frame) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	if w.conn == nil {
		return errors.New("not connected")
	}
	return relay.WriteFrame(w.conn, f)
}

func (w *relayWorker) run() {
	defer w.agent.wg.Done()
	addr := fmt.Sprintf("%s:%d", w.target.Address, w.target.Port)

	for {
		select {
		case <-w.closed:
			return
		case <-w.agent.closed:
			return
		default:
		}

		err := w.connectAndServe(addr)
		atomic.StoreInt32(&w.connected, 0)
		if err != nil {
			select {
			case <-w.closed:
				return
			case <-w.agent.closed:
				return
			case <-time.After(w.agent.cfg.ReconnectInterval):
			}
		}
	}
}

func (w *relayWorker) connectAndServe(addr string) error {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return err
	}
	w.conn = conn
	defer func() {
		_ = conn.Close()
		atomic.StoreInt32(&w.connected, 0)
		w.streamsMu.Lock()
		for _, s := range w.localStreams {
			s.close()
		}
		w.localStreams = make(map[uint32]*localStream)
		w.streamsMu.Unlock()
	}()

	token := w.target.Token
	if token == "" {
		token = w.agent.cfg.DefaultToken
	}

	authPayload, _ := json.Marshal(relay.AuthPayload{
		ClusterID: w.agent.cfg.ClusterID,
		OriginID:  w.agent.cfg.OriginID,
		Token:     token,
	})

	if err := relay.WriteFrame(conn, &relay.Frame{
		Type:    relay.FrameTypeAuth,
		Payload: authPayload,
	}); err != nil {
		return err
	}

	authResp, err := relay.ReadFrame(conn)
	if err != nil {
		return err
	}
	if authResp.Type != relay.FrameTypeAuthResp {
		return errors.New("invalid auth response frame")
	}

	var authResult relay.AuthResponsePayload
	if err := json.Unmarshal(authResp.Payload, &authResult); err != nil {
		return err
	}
	if authResult.Status != relay.AuthStatusOK {
		return fmt.Errorf("relay auth denied: %s", authResult.Message)
	}

	atomic.StoreInt32(&w.connected, 1)

	// Heartbeat goroutine
	heartbeatStop := make(chan struct{})
	defer close(heartbeatStop)
	go func() {
		ticker := time.NewTicker(w.agent.cfg.HeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := w.sendFrame(&relay.Frame{Type: relay.FrameTypePing}); err != nil {
					_ = conn.Close()
					return
				}
			case <-heartbeatStop:
				return
			case <-w.closed:
				return
			case <-w.agent.closed:
				return
			}
		}
	}()

	// Frame read loop
	for {
		f, err := relay.ReadFrame(conn)
		if err != nil {
			return err
		}

		switch f.Type {
		case relay.FrameTypePong:
			// Heartbeat ACK
		case relay.FrameTypeStreamOpen:
			st := &localStream{
				id:     f.StreamID,
				dataCh: make(chan []byte, 64),
				closed: make(chan struct{}),
			}
			w.streamsMu.Lock()
			w.localStreams[f.StreamID] = st
			w.streamsMu.Unlock()
			go w.handleStreamOpen(st)
		case relay.FrameTypeStreamData:
			w.streamsMu.RLock()
			st, exists := w.localStreams[f.StreamID]
			w.streamsMu.RUnlock()
			if exists {
				select {
				case st.dataCh <- f.Payload:
				case <-st.closed:
				}
			}
		case relay.FrameTypeStreamClose, relay.FrameTypeStreamReset:
			w.streamsMu.Lock()
			st, exists := w.localStreams[f.StreamID]
			if exists {
				delete(w.localStreams, f.StreamID)
			}
			w.streamsMu.Unlock()
			if exists {
				st.close()
			}
		}
	}
}

func (w *relayWorker) handleStreamOpen(st *localStream) {
	localConn, err := net.DialTimeout("tcp", w.agent.cfg.TargetDataAddr, 3*time.Second)
	if err != nil {
		w.streamsMu.Lock()
		delete(w.localStreams, st.id)
		w.streamsMu.Unlock()
		st.close()
		_ = w.sendFrame(&relay.Frame{
			Type:     relay.FrameTypeStreamReset,
			StreamID: st.id,
		})
		return
	}
	st.localConn = localConn

	defer func() {
		w.streamsMu.Lock()
		delete(w.localStreams, st.id)
		w.streamsMu.Unlock()
		st.close()
		_ = localConn.Close()
		_ = w.sendFrame(&relay.Frame{
			Type:     relay.FrameTypeStreamClose,
			StreamID: st.id,
		})
	}()

	// Read from dataCh and write to local data gateway
	doneWriting := make(chan struct{})
	go func() {
		defer close(doneWriting)
		for {
			select {
			case data, ok := <-st.dataCh:
				if !ok || len(data) == 0 {
					return
				}
				if _, err := localConn.Write(data); err != nil {
					return
				}
			case <-st.closed:
				return
			case <-w.closed:
				return
			}
		}
	}()

	// Read from local data gateway and forward to relay as FrameStreamData
	buf := make([]byte, 16*1024)
	for {
		n, err := localConn.Read(buf)
		if n > 0 {
			payload := make([]byte, n)
			copy(payload, buf[:n])
			if sendErr := w.sendFrame(&relay.Frame{
				Type:     relay.FrameTypeStreamData,
				StreamID: st.id,
				Payload:  payload,
			}); sendErr != nil {
				break
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			break
		}
	}

	st.close()
	<-doneWriting
}
