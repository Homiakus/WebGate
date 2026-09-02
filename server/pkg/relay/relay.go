package relay

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	ControlAddr  string        // Address to listen for Origin reverse connections (e.g. ":43211")
	ClientAddr   string        // Address to listen for incoming client transit connections (e.g. ":43111")
	ClusterToken string        // Shared secret token required for Origin authentication
	IdleTimeout  time.Duration // Timeout for inactive streams
}

type AuthPayload struct {
	ClusterID string `json:"cluster_id"`
	OriginID  string `json:"origin_id"`
	Token     string `json:"token"`
}

type AuthResponsePayload struct {
	Status  uint8  `json:"status"`
	Message string `json:"message"`
}

type originSession struct {
	id         string
	clusterID  string
	conn       net.Conn
	writeMu    sync.Mutex
	streams    map[uint32]*relayStream
	streamsMu  sync.RWMutex
	closed     chan struct{}
	closeOnce  sync.Once
	lastActive int64
}

func (s *originSession) sendFrame(f *Frame) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return WriteFrame(s.conn, f)
}

func (s *originSession) close() {
	s.closeOnce.Do(func() {
		close(s.closed)
		_ = s.conn.Close()
		s.streamsMu.Lock()
		for _, stream := range s.streams {
			stream.close()
		}
		s.streams = make(map[uint32]*relayStream)
		s.streamsMu.Unlock()
	})
}

type relayStream struct {
	id         uint32
	clientConn net.Conn
	dataCh     chan []byte
	closed     chan struct{}
	closeOnce  sync.Once
}

func (s *relayStream) close() {
	s.closeOnce.Do(func() {
		close(s.closed)
	})
}

type RelayServer struct {
	cfg          Config
	controlLn    net.Listener
	clientLn     net.Listener
	origins      map[string]*originSession
	originsMu    sync.RWMutex
	nextStreamID uint32
	closed       chan struct{}
	closeOnce    sync.Once
	wg           sync.WaitGroup
}

func NewRelayServer(cfg Config) (*RelayServer, error) {
	if cfg.ControlAddr == "" {
		return nil, errors.New("relay: control listen address cannot be empty")
	}
	if cfg.ClientAddr == "" {
		return nil, errors.New("relay: client listen address cannot be empty")
	}
	if cfg.ClusterToken == "" {
		return nil, errors.New("relay: cluster token cannot be empty")
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 30 * time.Second
	}

	return &RelayServer{
		cfg:     cfg,
		origins: make(map[string]*originSession),
		closed:  make(chan struct{}),
	}, nil
}

func (r *RelayServer) Start() error {
	controlLn, err := net.Listen("tcp", r.cfg.ControlAddr)
	if err != nil {
		return fmt.Errorf("relay: failed to bind control listener: %w", err)
	}
	r.controlLn = controlLn

	clientLn, err := net.Listen("tcp", r.cfg.ClientAddr)
	if err != nil {
		_ = controlLn.Close()
		return fmt.Errorf("relay: failed to bind client listener: %w", err)
	}
	r.clientLn = clientLn

	r.wg.Add(2)
	go r.acceptControlLoop()
	go r.acceptClientLoop()

	return nil
}

func (r *RelayServer) ControlAddr() net.Addr {
	if r.controlLn != nil {
		return r.controlLn.Addr()
	}
	return nil
}

func (r *RelayServer) ClientAddr() net.Addr {
	if r.clientLn != nil {
		return r.clientLn.Addr()
	}
	return nil
}

func (r *RelayServer) ConnectedOrigins() int {
	r.originsMu.RLock()
	defer r.originsMu.RUnlock()
	return len(r.origins)
}

func (r *RelayServer) Stop() {
	r.closeOnce.Do(func() {
		close(r.closed)
		if r.controlLn != nil {
			_ = r.controlLn.Close()
		}
		if r.clientLn != nil {
			_ = r.clientLn.Close()
		}

		r.originsMu.Lock()
		for _, session := range r.origins {
			session.close()
		}
		r.origins = make(map[string]*originSession)
		r.originsMu.Unlock()

		r.wg.Wait()
	})
}

func (r *RelayServer) acceptControlLoop() {
	defer r.wg.Done()
	for {
		conn, err := r.controlLn.Accept()
		if err != nil {
			select {
			case <-r.closed:
				return
			default:
				return
			}
		}
		r.wg.Add(1)
		go r.handleControlConn(conn)
	}
}

func (r *RelayServer) handleControlConn(conn net.Conn) {
	defer r.wg.Done()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	frame, err := ReadFrame(conn)
	if err != nil {
		_ = conn.Close()
		return
	}
	if frame.Type != FrameTypeAuth {
		_ = conn.Close()
		return
	}

	var authReq AuthPayload
	if err := json.Unmarshal(frame.Payload, &authReq); err != nil {
		_ = conn.Close()
		return
	}

	tokenMatch := subtle.ConstantTimeCompare([]byte(authReq.Token), []byte(r.cfg.ClusterToken)) == 1
	if !tokenMatch || authReq.OriginID == "" {
		respBytes, _ := json.Marshal(AuthResponsePayload{
			Status:  AuthStatusDenied,
			Message: "invalid credentials or origin id",
		})
		_ = WriteFrame(conn, &Frame{
			Type:    FrameTypeAuthResp,
			Payload: respBytes,
		})
		_ = conn.Close()
		return
	}

	respBytes, _ := json.Marshal(AuthResponsePayload{
		Status:  AuthStatusOK,
		Message: "authenticated",
	})
	if err := WriteFrame(conn, &Frame{
		Type:    FrameTypeAuthResp,
		Payload: respBytes,
	}); err != nil {
		_ = conn.Close()
		return
	}

	// Disable deadline for active long-lived reverse session
	_ = conn.SetDeadline(time.Time{})

	session := &originSession{
		id:         authReq.OriginID,
		clusterID:  authReq.ClusterID,
		conn:       conn,
		streams:    make(map[uint32]*relayStream),
		closed:     make(chan struct{}),
		lastActive: time.Now().UnixNano(),
	}

	r.originsMu.Lock()
	if existing, ok := r.origins[session.id]; ok {
		existing.close()
	}
	r.origins[session.id] = session
	r.originsMu.Unlock()

	defer func() {
		r.originsMu.Lock()
		if current, ok := r.origins[session.id]; ok && current == session {
			delete(r.origins, session.id)
		}
		r.originsMu.Unlock()
		session.close()
	}()

	for {
		f, err := ReadFrame(conn)
		if err != nil {
			return
		}
		atomic.StoreInt64(&session.lastActive, time.Now().UnixNano())

		switch f.Type {
		case FrameTypePing:
			_ = session.sendFrame(&Frame{Type: FrameTypePong})
		case FrameTypeStreamData:
			session.streamsMu.RLock()
			st, exists := session.streams[f.StreamID]
			session.streamsMu.RUnlock()
			if exists {
				select {
				case st.dataCh <- f.Payload:
				case <-st.closed:
				case <-session.closed:
					return
				}
			}
		case FrameTypeStreamClose, FrameTypeStreamReset:
			session.streamsMu.Lock()
			st, exists := session.streams[f.StreamID]
			if exists {
				delete(session.streams, f.StreamID)
			}
			session.streamsMu.Unlock()
			if exists {
				st.close()
			}
		}
	}
}

func (r *RelayServer) acceptClientLoop() {
	defer r.wg.Done()
	for {
		conn, err := r.clientLn.Accept()
		if err != nil {
			select {
			case <-r.closed:
				return
			default:
				return
			}
		}
		r.wg.Add(1)
		go r.handleClientConn(conn)
	}
}

func (r *RelayServer) getActiveOrigin() *originSession {
	r.originsMu.RLock()
	defer r.originsMu.RUnlock()
	for _, session := range r.origins {
		select {
		case <-session.closed:
			continue
		default:
			return session
		}
	}
	return nil
}

func (r *RelayServer) handleClientConn(clientConn net.Conn) {
	defer r.wg.Done()

	session := r.getActiveOrigin()
	if session == nil {
		// Fail closed when no origin reverse session is available
		_ = clientConn.Close()
		return
	}

	streamID := atomic.AddUint32(&r.nextStreamID, 1)
	st := &relayStream{
		id:         streamID,
		clientConn: clientConn,
		dataCh:     make(chan []byte, 32),
		closed:     make(chan struct{}),
	}

	session.streamsMu.Lock()
	session.streams[streamID] = st
	session.streamsMu.Unlock()

	defer func() {
		session.streamsMu.Lock()
		delete(session.streams, streamID)
		session.streamsMu.Unlock()
		st.close()
		_ = clientConn.Close()
		_ = session.sendFrame(&Frame{
			Type:     FrameTypeStreamClose,
			StreamID: streamID,
		})
	}()

	// Notify origin of new inbound stream
	if err := session.sendFrame(&Frame{
		Type:     FrameTypeStreamOpen,
		StreamID: streamID,
	}); err != nil {
		return
	}

	// Goroutine 1: Read data received from Origin and write to Client
	doneWriting := make(chan struct{})
	go func() {
		defer close(doneWriting)
		for {
			select {
			case data, ok := <-st.dataCh:
				if !ok || len(data) == 0 {
					return
				}
				if _, err := clientConn.Write(data); err != nil {
					return
				}
			case <-st.closed:
				return
			case <-session.closed:
				return
			case <-r.closed:
				return
			}
		}
	}()

	// Goroutine 2: Read data from Client and send as FrameStreamData to Origin
	buf := make([]byte, 16*1024)
	for {
		n, err := clientConn.Read(buf)
		if n > 0 {
			frameData := make([]byte, n)
			copy(frameData, buf[:n])
			if sendErr := session.sendFrame(&Frame{
				Type:     FrameTypeStreamData,
				StreamID: streamID,
				Payload:  frameData,
			}); sendErr != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}

	st.close()
	<-doneWriting
}
