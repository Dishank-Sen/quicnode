package auth

import (
	"fmt"
	"io"

	"github.com/flynn/noise"
)

// HandshakeRole indicates whether this node initiates or responds to the handshake.
type HandshakeRole int

const (
	// HandshakeInitiator is the client that opens the connection and starts the handshake.
	HandshakeInitiator HandshakeRole = iota
	// HandshakeResponder is the server that accepts the connection and responds to the handshake.
	HandshakeResponder
)

// Handshake represents a Noise XX handshake session.
// XX provides mutual authentication without prior key knowledge.
type Handshake struct {
	handshakeState *noise.HandshakeState
	cipherState    *noise.CipherState
	remoteKey      []byte // Remote peer's static public key (32 bytes)
	role           HandshakeRole
}

// NewHandshake creates a new Noise XX handshake session.
// localKeypair is this node's static keypair.
// role determines if this is the initiator or responder.
func NewHandshake(localKeypair noise.DHKey, role HandshakeRole) (*Handshake, error) {
	// Noise XX pattern: full mutual authentication, no prior keys
	config := noise.Config{
		CipherSuite: noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashSHA256),
		Pattern:     noise.HandshakeXX,
		Initiator:   role == HandshakeInitiator,
		StaticKeypair: localKeypair,
	}

	hs, err := noise.NewHandshakeState(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create handshake state: %w", err)
	}

	return &Handshake{
		handshakeState: hs,
		role:           role,
	}, nil
}

// WriteMessage generates the next handshake message.
// For initiator: call this 2 times (message 1 and 3).
// For responder: call this 2 times (message 2 and 4).
// Returns the message bytes to send to the peer.
func (h *Handshake) WriteMessage(payload []byte) ([]byte, error) {
	if h.handshakeState == nil {
		return nil, fmt.Errorf("handshake already completed")
	}

	msg, _, _, err := h.handshakeState.WriteMessage(nil, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to write handshake message: %w", err)
	}

	// Check if handshake is complete after this write
	// XX pattern has 3 messages total (indices 0, 1, 2)
	// MessageIndex reaches 3 when all messages are exchanged
	if h.handshakeState.MessageIndex() >= 3 {
		h.finalizeHandshake()
	}

	return msg, nil
}

// ReadMessage processes an incoming handshake message.
// For initiator: call this 2 times (message 2 and 4).
// For responder: call this 2 times (message 1 and 3).
// Returns the decrypted payload (if any).
func (h *Handshake) ReadMessage(message []byte) ([]byte, error) {
	if h.handshakeState == nil {
		return nil, fmt.Errorf("handshake already completed")
	}

	payload, _, _, err := h.handshakeState.ReadMessage(nil, message)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake message: %w", err)
	}

	// Extract remote static key after it's been transmitted
	// In XX pattern, keys are transmitted in messages 2 and 3
	if h.handshakeState.PeerStatic() != nil && len(h.remoteKey) == 0 {
		h.remoteKey = h.handshakeState.PeerStatic()
	}

	// Check if handshake is complete after this read
	// XX pattern has 3 messages total (indices 0, 1, 2)
	// MessageIndex reaches 3 when all messages are exchanged
	if h.handshakeState.MessageIndex() >= 3 {
		h.finalizeHandshake()
	}

	return payload, nil
}

// finalizeHandshake transitions from handshake to transport mode.
func (h *Handshake) finalizeHandshake() {
	// XX handshake complete - cipher states would be used for post-handshake encryption
	// For now, QUIC already provides transport encryption, so we only need the authentication

	// Clear handshake state to free memory
	h.handshakeState = nil
}

// IsComplete returns true if the handshake has finished.
func (h *Handshake) IsComplete() bool {
	return h.handshakeState == nil
}

// RemotePublicKey returns the authenticated peer's static public key.
// Only valid after handshake completes.
func (h *Handshake) RemotePublicKey() []byte {
	return h.remoteKey
}

// PerformHandshake executes the complete Noise XX handshake over the given stream.
// For initiator: writes msg1, reads msg2, writes msg3, reads msg4.
// For responder: reads msg1, writes msg2, reads msg3, writes msg4.
// Returns the authenticated remote public key on success.
func PerformHandshake(rw io.ReadWriter, localKeypair noise.DHKey, role HandshakeRole) ([]byte, error) {
	hs, err := NewHandshake(localKeypair, role)
	if err != nil {
		return nil, err
	}

	if role == HandshakeInitiator {
		// Initiator sends first
		return performInitiatorHandshake(rw, hs)
	}
	return performResponderHandshake(rw, hs)
}

func performInitiatorHandshake(rw io.ReadWriter, hs *Handshake) ([]byte, error) {
	// Message 1: -> e
	msg1, err := hs.WriteMessage(nil)
	if err != nil {
		return nil, fmt.Errorf("initiator msg1: %w", err)
	}
	if err := writeFrame(rw, msg1); err != nil {
		return nil, fmt.Errorf("send msg1: %w", err)
	}

	// Message 2: <- e, ee, s, es
	msg2, err := readFrame(rw)
	if err != nil {
		return nil, fmt.Errorf("receive msg2: %w", err)
	}
	if _, err := hs.ReadMessage(msg2); err != nil {
		return nil, fmt.Errorf("initiator msg2: %w", err)
	}

	// Message 3: -> s, se
	msg3, err := hs.WriteMessage(nil)
	if err != nil {
		return nil, fmt.Errorf("initiator msg3: %w", err)
	}
	if err := writeFrame(rw, msg3); err != nil {
		return nil, fmt.Errorf("send msg3: %w", err)
	}

	if !hs.IsComplete() {
		return nil, fmt.Errorf("handshake incomplete after msg3")
	}

	return hs.RemotePublicKey(), nil
}

func performResponderHandshake(rw io.ReadWriter, hs *Handshake) ([]byte, error) {
	// Message 1: -> e
	msg1, err := readFrame(rw)
	if err != nil {
		return nil, fmt.Errorf("receive msg1: %w", err)
	}
	if _, err := hs.ReadMessage(msg1); err != nil {
		return nil, fmt.Errorf("responder msg1: %w", err)
	}

	// Message 2: <- e, ee, s, es
	msg2, err := hs.WriteMessage(nil)
	if err != nil {
		return nil, fmt.Errorf("responder msg2: %w", err)
	}
	if err := writeFrame(rw, msg2); err != nil {
		return nil, fmt.Errorf("send msg2: %w", err)
	}

	// Message 3: -> s, se
	msg3, err := readFrame(rw)
	if err != nil {
		return nil, fmt.Errorf("receive msg3: %w", err)
	}
	if _, err := hs.ReadMessage(msg3); err != nil {
		return nil, fmt.Errorf("responder msg3: %w", err)
	}

	if !hs.IsComplete() {
		return nil, fmt.Errorf("handshake incomplete after msg3")
	}

	return hs.RemotePublicKey(), nil
}

// writeFrame writes a length-prefixed frame to the writer.
// Format: [4 bytes big-endian length][payload]
func writeFrame(w io.Writer, data []byte) error {
	lenBuf := [4]byte{}
	lenBuf[0] = byte(len(data) >> 24)
	lenBuf[1] = byte(len(data) >> 16)
	lenBuf[2] = byte(len(data) >> 8)
	lenBuf[3] = byte(len(data))

	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}

// readFrame reads a length-prefixed frame from the reader.
func readFrame(r io.Reader) ([]byte, error) {
	lenBuf := [4]byte{}
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}

	length := int(lenBuf[0])<<24 | int(lenBuf[1])<<16 | int(lenBuf[2])<<8 | int(lenBuf[3])
	if length > 65535 {
		return nil, fmt.Errorf("frame too large: %d bytes", length)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}
