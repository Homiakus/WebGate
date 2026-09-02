package relay

import (
	"bytes"
	"testing"
)

func FuzzReadFrame(f *testing.F) {
	// Seed valid frames
	var buf bytes.Buffer
	_ = WriteFrame(&buf, &Frame{
		Type:     FrameTypePing,
		StreamID: 1,
		Payload:  nil,
	})
	f.Add(buf.Bytes())

	buf.Reset()
	_ = WriteFrame(&buf, &Frame{
		Type:     FrameTypeAuth,
		StreamID: 0,
		Payload:  []byte("test-cluster-token-123"),
	})
	f.Add(buf.Bytes())

	buf.Reset()
	_ = WriteFrame(&buf, &Frame{
		Type:     FrameTypeStreamData,
		StreamID: 42,
		Payload:  []byte("GET / HTTP/1.1\r\nHost: localhost\r\n\r\n"),
	})
	f.Add(buf.Bytes())

	// Seed invalid inputs
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x01, 0x02, 0x03})
	f.Add([]byte("NOT_A_VALID_RELAY_FRAME"))

	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		frame, err := ReadFrame(r)
		if err != nil {
			// Expected for invalid inputs; ensure no panic occurred
			return
		}
		if frame == nil {
			t.Fatalf("expected non-nil frame when err is nil")
		}
		// Invariant: frame payload size must not exceed MaxPayloadSize
		if len(frame.Payload) > MaxPayloadSize {
			t.Fatalf("frame payload length %d exceeds MaxPayloadSize %d", len(frame.Payload), MaxPayloadSize)
		}
	})
}
