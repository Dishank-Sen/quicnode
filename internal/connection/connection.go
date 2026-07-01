package connection

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/Dishank-Sen/quicnode/constants"
	"github.com/Dishank-Sen/quicnode/internal/auth"
	"github.com/Dishank-Sen/quicnode/internal/datagram"
	"github.com/Dishank-Sen/quicnode/internal/router"
	"github.com/Dishank-Sen/quicnode/internal/stream"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/flynn/noise"
	"github.com/quic-go/quic-go"
)

type Connections struct {
	router *router.Router
	numConn int
	connManager *ConnManager
	mu    sync.RWMutex

	// Authentication
	requireAuth  bool
	localKeypair noise.DHKey
}

func NewConnections(e chan types.Event, r *router.Router, requireAuth bool, localKeypair noise.DHKey) *Connections {
	return &Connections{
		router: r,
		connManager: newConnManage(e),
		numConn: 0,
		requireAuth: requireAuth,
		localKeypair: localKeypair,
	}
}

func (c *Connections) CreateConn(conn *quic.Conn) {
	c.mu.Lock()
	c.numConn++
	c.mu.Unlock()

	// add connection to connection manager
	addr := conn.RemoteAddr()
	peer := c.connManager.newConn(conn, addr.String())

	// If authentication required, perform handshake before accepting user streams
	if c.requireAuth {
		go c.authenticateAndHandle(peer)
	} else {
		// No auth - proceed directly to stream/datagram handling
		go c.handleStream(peer)
		go c.handleDatagram(peer)
	}
}

// authenticateAndHandle performs Noise XX handshake then starts stream/datagram handling.
// This is only called when RequireAuth is true.
func (c *Connections) authenticateAndHandle(peer *Peer) {
	defer func() {
		log.Println("connection closed")
		c.connManager.removeEntry(peer.ID)
		log.Println("entry removed")
	}()

	log.Printf("Performing Noise XX handshake with %s...", peer.Addr)

	// Wait for the peer to open the _noise handshake stream
	conn := peer.conn
	noiseStream, err := conn.AcceptStream(conn.Context())
	if err != nil {
		log.Printf("Failed to accept handshake stream from %s: %v", peer.Addr, err)
		return
	}

	// Perform XX handshake (we are responder since we accepted the connection)
	remotePublicKey, err := auth.PerformHandshake(noiseStream, c.localKeypair, auth.HandshakeResponder)
	if err != nil {
		log.Printf("Handshake failed with %s: %v", peer.Addr, err)
		noiseStream.Close()
		return
	}
	noiseStream.Close()

	// Create session and verify TOFU
	session, err := auth.NewSession(remotePublicKey)
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		return
	}

	if err := auth.VerifyOrStorePeer(session.PeerID, session.PublicKey); err != nil {
		log.Printf("TOFU verification failed for %s: %v", session.PeerID, err)
		return
	}

	// Store authenticated identity in peer
	peer.SetAuthenticated(session.PublicKey, session.PeerID)
	log.Printf("Authenticated peer %s (%s)", session.PeerID, peer.Addr)

	// Now proceed to normal stream/datagram handling
	go c.handleStream(peer)
	go c.handleDatagram(peer)
}

func (c *Connections) handleStream(peer *Peer) {
	conn := peer.conn
	for {
		log.Println("waiting for stream...")
        s, err := conn.AcceptStream(conn.Context())
        if err != nil {
            log.Printf("connection.go - %v", err)
            return
        }
		log.Println("got a stream")

		// Get peer identity for context
		publicKey, peerID, _ := peer.GetAuthInfo()

		stream := stream.NewStream(conn.Context(), s, conn.RemoteAddr(), c.router.GetStreamHandler, c.router.GetInternalStreamHandler, publicKey, peerID)
		go stream.Handle()
    }
}

func (c *Connections) handleDatagram(peer *Peer) {
	conn := peer.conn
	ctx := conn.Context()
	for{
		msg, err := conn.ReceiveDatagram(ctx)
		if err != nil {
			log.Printf("error in datagram: %v", err)
			return
		}
		d := datagram.NewDatagram(ctx, msg, conn.RemoteAddr(), c.router.GetDatagramHandler)
		go d.Handle()
	}
}

func (c *Connections) OpenConn(ctx context.Context, tr *quic.Transport, tlsCfg *tls.Config, quicCfg *quic.Config, addr net.Addr) (*Peer, error) {
	// validate addr
	if ok := isValidAddr(addr); !ok {
		return nil, fmt.Errorf("invalid address")
	}

	peer, ok := c.connManager.getPeer(addr.String())
	if ok {
		log.Println("peer already exist")
		return peer, nil
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, constants.QuicDialTimeout)
	defer dialCancel()

	log.Println("opening conn...")
	newConn, err := tr.Dial(
			dialCtx,
			addr,
			tlsCfg,
			quicCfg,
		)

	if err != nil {
		log.Println("error in open conn - 1")
		return nil, err
	}
	log.Println("conn opened")

	// add connection to connection manager
	peer = c.connManager.newConn(newConn, addr.String())
	c.mu.Lock()
	c.numConn++
	c.mu.Unlock()

	// If authentication required, perform handshake as initiator
	if c.requireAuth {
		if err := c.performClientAuth(peer); err != nil {
			peer.Close()
			c.connManager.removeEntry(peer.ID)
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
	}

	go c.handleStream(peer)
	go c.handleDatagram(peer)

	return peer, nil
}

// performClientAuth initiates a Noise XX handshake as the client.
func (c *Connections) performClientAuth(peer *Peer) error {
	log.Printf("Initiating Noise XX handshake with %s...", peer.Addr)

	// Open the _noise handshake stream
	noiseStream, err := peer.conn.OpenStream()
	if err != nil {
		return fmt.Errorf("failed to open handshake stream: %w", err)
	}
	defer noiseStream.Close()

	// Perform XX handshake (we are initiator since we dialed)
	remotePublicKey, err := auth.PerformHandshake(noiseStream, c.localKeypair, auth.HandshakeInitiator)
	if err != nil {
		return fmt.Errorf("handshake failed: %w", err)
	}

	// Create session and verify TOFU
	session, err := auth.NewSession(remotePublicKey)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	if err := auth.VerifyOrStorePeer(session.PeerID, session.PublicKey); err != nil {
		return fmt.Errorf("TOFU verification failed: %w", err)
	}

	// Store authenticated identity
	peer.SetAuthenticated(session.PublicKey, session.PeerID)
	log.Printf("Authenticated peer %s (%s)", session.PeerID, peer.Addr)

	return nil
}

func isValidAddr(addr net.Addr) bool {
    host, _, err := net.SplitHostPort(addr.String())
    if err != nil {
        return false
    }
    return net.ParseIP(host) != nil
}

func (c *Connections) Count() int {
	return c.numConn
}

func (c *Connections) CloseAll() {
	for _, c := range c.connManager.getAllPeers() {
		_ = c.conn.CloseWithError(0, "node shutting down...")
	}
}