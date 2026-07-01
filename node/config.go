package node

import (
	"github.com/quic-go/quic-go"
)

// Config holds the configuration for a Node.
type Config struct {
	// ListenAddr is the local address to bind to (e.g., "127.0.0.1:4242" or ":4242").
	ListenAddr string

	// QuicConfig provides QUIC transport settings (required).
	QuicConfig *quic.Config

	// RequireAuth enables Noise XX authentication for all connections.
	// When true, peers must complete a Noise handshake before any application streams are accepted.
	// When false (default), authentication is skipped entirely for backward compatibility.
	// If enabled, the node's keypair is loaded from ~/.local/share/quicnode/node.key
	// or generated on first run.
	// TLS certificates are automatically derived from the node's keypair - user never manages them.
	RequireAuth bool
}