package node

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/Dishank-Sen/quicnode/internal/auth"
	"github.com/Dishank-Sen/quicnode/internal/connection"
	"github.com/Dishank-Sen/quicnode/internal/datagram"
	"github.com/Dishank-Sen/quicnode/internal/router"
	"github.com/Dishank-Sen/quicnode/internal/stream"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/flynn/noise"
	"github.com/quic-go/quic-go"
	"golang.org/x/sync/errgroup"
)

// StreamContext provides access to an incoming stream request.
// Available in StreamHandlerFunc callbacks registered via HandleStream.
type StreamContext = stream.Context

// DatagramContext provides access to an incoming datagram.
// Available in DatagramHandlerFunc callbacks registered via HandleDatagram.
type DatagramContext = datagram.Context

// StreamHandlerFunc is called when a stream request arrives on a registered route.
// The handler can read from ctx.Payload() and write responses via ctx.Write().
type StreamHandlerFunc = stream.HandlerFunc

// DatagramHandlerFunc is called when a datagram arrives on a registered route.
// The handler can read from ctx.Payload(). Datagrams are one-way (no response).
type DatagramHandlerFunc = datagram.HandlerFunc

// Peer represents a connection to a remote node.
// Obtained via OpenConn. Use Send() for reliable streams or SendDatagram() for unreliable messages.
type Peer = connection.Peer

// Node is a QUIC-based network node that can both accept connections and dial out to peers.
// A node listens on a local address and maintains a connection pool to remote peers.
type Node struct{
	cfg Config
	group  *errgroup.Group
    gctx   context.Context
	ctx context.Context
	cancel context.CancelFunc
	listener *quic.Listener
	transport *quic.Transport
	router *router.Router
	udpConn *net.UDPConn
	once sync.Once
	events chan types.Event
	connections *connection.Connections

	// Authentication (only initialized if RequireAuth is true)
	localKeypair noise.DHKey
}

// NewNode creates a new QUIC node with the given configuration.
// The provided context controls the lifetime of the node - when cancelled, the node shuts down.
// Config must include a valid ListenAddr, TlsConfig, and QuicConfig.
// Returns error if configuration validation fails.
//
// Example:
//   node, err := NewNode(ctx, Config{
//       ListenAddr: "127.0.0.1:4242",
//       TlsConfig:  tlsCfg,
//       QuicConfig: &quic.Config{},
//   })
func NewNode(ctx context.Context, cfg Config) (*Node, error){
	if ctx == nil{
		return nil, fmt.Errorf("context is nil")
	}

	g, gctx := errgroup.WithContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	if err := checkConfig(cfg); err != nil{
		cancel()
		return nil, err
	}
	r := router.NewRouter()

	events := make(chan types.Event, 100)

	// Always load or generate keypair (needed for TLS certificate)
	keypair, err := auth.LoadOrGenerateKeypair()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to load keypair: %w", err)
	}

	if cfg.RequireAuth {
		log.Printf("Noise authentication enabled (public key: %x...)", keypair.Public[:8])
	}

	n := &Node{
		cfg: cfg,
		group: g,
		gctx: gctx,
		ctx: ctx,
		cancel: cancel,
		router: r,
		events: events,
		connections: connection.NewConnections(events, r, cfg.RequireAuth, keypair),
		localKeypair: keypair,
	}

	return n, nil
}

func checkConfig(cfg Config) error{
	if err := validateListenAddr(cfg.ListenAddr); err != nil{
		return err
	}

	if cfg.QuicConfig == nil{
		return fmt.Errorf("quic config is required")
	}

	return nil
}

func validateListenAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid ListenAddr: %w", err)
	}

	p, err := strconv.Atoi(port)
	if err != nil || p <= 0 || p > 65535 {
		return fmt.Errorf("invalid port: %s", port)
	}

	_ = host // host can be "", "0.0.0.0", "::", etc.
	return nil
}

// Start begins listening for incoming QUIC connections on the configured address.
// This method is non-blocking - it spawns goroutines for the accept loop and returns immediately.
// Returns error if the listener cannot be created (e.g., port already in use).
// The node will continue running until Stop() is called or the context is cancelled.
func (n *Node) Start() error{
	n.group.Go(func() error {
		<-n.ctx.Done()
		n.once.Do(n.shutdown)
		return fmt.Errorf("node context cancelled")
	})
	addr := n.cfg.ListenAddr
	host, portstr, err := net.SplitHostPort(addr)
	port, err := strconv.Atoi(portstr)
	if err != nil{
		return err
	}

	// Generate TLS config from the node's keypair
	tlsCfg, err := auth.GetTLSConfig(n.localKeypair)
	if err != nil {
		return fmt.Errorf("failed to generate TLS config: %w", err)
	}

	quicCfg := n.cfg.QuicConfig

	udpConn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP: net.ParseIP(host),
		Port: port,
	})
	if err != nil{
		return err
	}
	n.udpConn = udpConn

	n.transport = &quic.Transport{Conn: udpConn}

	listener, err := n.transport.Listen(tlsCfg, quicCfg)
	if err != nil {
		n.once.Do(n.shutdown)
		return err
	}

	n.listener = listener

	n.group.Go(func() error{
		return n.acceptLoop()
	})
	return nil
}

// Stop gracefully shuts down the node.
// Closes the listener, cancels the node context, and closes all active peer connections.
// Safe to call multiple times - only the first call takes effect.
// Returns nil (error exists for interface compatibility).
func (n *Node) Stop() error{
	n.once.Do(n.shutdown)
	return nil
}

func (n *Node) acceptLoop() error{
	for{
		log.Println("waiting for connection...")
		conn, err := n.listener.Accept(n.ctx)
		if err != nil{
			if n.ctx.Err() != nil {
				return fmt.Errorf("acceptLoop stopped: %w", err)
			}
			log.Println("accept error:", err)
			continue
		}

		log.Println("waiting for handshake")
		select {
		case <-conn.HandshakeComplete():
			// ok
		case <-conn.Context().Done():
			// handshake failed / connection died early
			continue
		case <-n.ctx.Done():
			return fmt.Errorf("acceptLoop - node context cancelled")
		}

		log.Println("handshake complete")
		n.connections.CreateConn(conn)
	}
}

// HandleStream registers a handler function for the given route.
// When a remote peer sends a stream request to this route, the handler is called.
// Streams are reliable and ordered - use for request-response patterns.
// The handler receives a StreamContext which provides access to the request payload
// and a Write() method for sending responses back to the requester.
//
// Example:
//   node.HandleStream("echo", func(ctx node.StreamContext) {
//       ctx.Write([]byte("echo: " + string(ctx.Payload())))
//   })
func (n *Node) HandleStream(route string, h stream.HandlerFunc){
	n.router.StreamRoute(route, h)
}

// HandleDatagram registers a handler function for the given route.
// When a remote peer sends a datagram to this route, the handler is called.
// Datagrams are unreliable and may be lost or arrive out of order.
// Use for real-time data where loss is acceptable (game position updates, telemetry).
// The handler receives a DatagramContext which provides read-only access to the payload.
// Datagrams are one-way - there is no response mechanism.
//
// Requires EnableDatagrams: true in QuicConfig.
//
// Example:
//   node.HandleDatagram("telemetry", func(ctx node.DatagramContext) {
//       metrics := parseMetrics(ctx.Payload())
//       recordMetrics(metrics)
//   })
func (n *Node) HandleDatagram(route string, h datagram.HandlerFunc){
	n.router.DatagramRoute(route, h)
}

// OpenConn opens a connection to the remote address and returns a Peer handle.
// If a connection to this address already exists, it is reused and the existing Peer is returned.
// The returned Peer can be used to send stream requests via Send() or datagrams via SendDatagram().
// The connection remains open until explicitly closed via Peer.Close() or node shutdown.
//
// The addr must be in "host:port" format (e.g., "127.0.0.1:4242" or "example.com:443").
// Context can be used to timeout the connection attempt.
//
// Example:
//   peer, err := node.OpenConn(ctx, "127.0.0.1:4242")
//   if err != nil {
//       return err
//   }
//   respCh, _ := peer.Send("echo", []byte("hello"))
func (n *Node) OpenConn(ctx context.Context, addr string) (*connection.Peer, error){
	desAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil{
		return nil, err
	}

	// Generate TLS config from the node's keypair
	tlsCfg, err := auth.GetTLSConfig(n.localKeypair)
	if err != nil {
		return nil, fmt.Errorf("failed to generate TLS config: %w", err)
	}

	return n.connections.OpenConn(ctx, n.transport, tlsCfg, n.cfg.QuicConfig, desAddr)
}

func (n *Node) shutdown(){
	log.Println("node shutting down")
	// important: as listener error still keeps the udp socket open 
	// and it may cause socket leak or port already in use issue later on.
	n.cancel()
	
	// close all connections
	n.connections.CloseAll()

	if n.listener != nil {
        _ = n.listener.Close()
    }

	if n.udpConn != nil {
        _ = n.udpConn.Close()
    }
}

// Wait blocks until the node's context is cancelled or an error occurs in a background goroutine.
// Typically called after Start() to keep the main goroutine alive.
// Returns the first error encountered by any managed goroutine, or context cancellation error.
//
// Example:
//   node.Start()
//   if err := node.Wait(); err != nil {
//       log.Printf("Node stopped: %v", err)
//   }
func (n *Node) Wait() error {
    return n.group.Wait()
}

// Events returns a receive-only channel of connection lifecycle events.
// Events are sent when connections are opened (EventConnOpened) or closed (EventConnClosed).
// The channel is buffered with capacity 100 - if not consumed, events may be dropped.
// Useful for monitoring connection state or implementing custom connection management logic.
//
// Example:
//   for event := range node.Events() {
//       switch event.Type {
//       case types.EventConnOpened:
//           log.Printf("Connection opened: %s", event.ConnID)
//       case types.EventConnClosed:
//           log.Printf("Connection closed: %s (err: %v)", event.ConnID, event.Err)
//       }
//   }
func (n *Node) Events() <-chan types.Event {
    return n.events
}