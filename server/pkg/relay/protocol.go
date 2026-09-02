package relay

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	ProtocolMagic   uint32 = 0x5747524C // "WGRL" (WebGate Relay Link)
	ProtocolVersion uint8  = 1

	FrameTypeAuth        uint8 = 0x01
	FrameTypeAuthResp    uint8 = 0x02
	FrameTypePing        uint8 = 0x03
	FrameTypePong        uint8 = 0x04
	FrameTypeStreamOpen  uint8 = 0x05
	FrameTypeStreamData  uint8 = 0x06
	FrameTypeStreamClose uint8 = 0x07
	FrameTypeStreamReset uint8 = 0x08

	AuthStatusOK     uint8 = 0x00
	AuthStatusDenied uint8 = 0x01

	MaxPayloadSize = 64 * 1024 // 64KB per frame
)

var (
	ErrInvalidMagic    = errors.New("relay: invalid protocol magic header")
	ErrUnsupportedVer  = errors.New("relay: unsupported protocol version")
	ErrPayloadTooLarge = errors.New("relay: frame payload exceeds maximum allowed size")
	ErrAuthFailed      = errors.New("relay: origin authentication failed")
	ErrNoActiveOrigin  = errors.New("relay: no active origin reverse session available")
)

// Frame represents a binary frame transmitted over the reverse tunnel.
// Format:
// [0..3]  Magic (0x5747524C)
// [4]     Version (1)
// [5]     Type (FrameType*)
// [6..7]  Flags (uint16)
// [8..11] StreamID (uint32)
// [12..15] PayloadLength (uint32)
// [16..]  Payload bytes
type Frame struct {
	Type     uint8
	Flags    uint16
	StreamID uint32
	Payload  []byte
}

func WriteFrame(w io.Writer, f *Frame) error {
	if len(f.Payload) > MaxPayloadSize {
		return ErrPayloadTooLarge
	}
	var header [16]byte
	binary.BigEndian.PutUint32(header[0:4], ProtocolMagic)
	header[4] = ProtocolVersion
	header[5] = f.Type
	binary.BigEndian.PutUint16(header[6:8], f.Flags)
	binary.BigEndian.PutUint32(header[8:12], f.StreamID)
	binary.BigEndian.PutUint32(header[12:16], uint32(len(f.Payload)))

	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	if len(f.Payload) > 0 {
		if _, err := w.Write(f.Payload); err != nil {
			return err
		}
	}
	return nil
}

func ReadFrame(r io.Reader) (*Frame, error) {
	var header [16]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	magic := binary.BigEndian.Uint32(header[0:4])
	if magic != ProtocolMagic {
		return nil, fmt.Errorf("%w: received 0x%08X", ErrInvalidMagic, magic)
	}
	version := header[4]
	if version != ProtocolVersion {
		return nil, fmt.Errorf("%w: received %d", ErrUnsupportedVer, version)
	}

	frameType := header[5]
	flags := binary.BigEndian.Uint16(header[6:8])
	streamID := binary.BigEndian.Uint32(header[8:12])
	payloadLen := binary.BigEndian.Uint32(header[12:16])

	if payloadLen > MaxPayloadSize {
		return nil, fmt.Errorf("%w: length %d > max %d", ErrPayloadTooLarge, payloadLen, MaxPayloadSize)
	}

	var payload []byte
	if payloadLen > 0 {
		payload = make([]byte, payloadLen)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
	}

	return &Frame{
		Type:     frameType,
		Flags:    flags,
		StreamID: streamID,
		Payload:  payload,
	}, nil
}
