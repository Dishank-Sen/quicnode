package node

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/Dishank-Sen/quicnode/internal/connection"
	"github.com/Dishank-Sen/quicnode/internal/datagram"
	"github.com/Dishank-Sen/quicnode/internal/router"
	"github.com/Dishank-Sen/quicnode/internal/stream"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/quic-go/quic-go"
	"golang.org/x/sync/errgroup"
)

// re-export so users only need to import node package
type StreamContext = stream.Context
type DatagramContext = datagram.Context
type StreamHandlerFunc = stream.HandlerFunc
type DatagramHandlerFunc = datagram.HandlerFunc
type Peer = connection.Peer

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
}

// NewNode returns a new node
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
	
	n := &Node{
		cfg: cfg,
		group: g,
		gctx: gctx,
		ctx: ctx,
		cancel: cancel,
		router: r,
		events: events,
		connections: connection.NewConnections(events, r),
	}

	return n, nil
}

func checkConfig(cfg Config) error{
	if err := validateListenAddr(cfg.ListenAddr); err != nil{
		return err
	}

	if cfg.TlsConfig == nil{
		return fmt.Errorf("tls config is required")
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

/* non blocking */
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
	tlsCfg := n.cfg.TlsConfig
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

// HandleStream registers a handler for stream-based requests
// Streams are reliable, ordered, and support request-response patterns
func (n *Node) HandleStream(route string, h stream.HandlerFunc){
	n.router.StreamRoute(route, h)
}

// HandleDatagram registers a handler for datagram-based requests
// Datagrams are unreliable, unordered, and fire-and-forget
// Use for real-time data where loss is acceptable (game updates, telemetry, etc.)
func (n *Node) HandleDatagram(route string, h datagram.HandlerFunc){
	n.router.DatagramRoute(route, h)
}

func (n *Node) OpenConn(ctx context.Context, addr string) (*connection.Peer, error){
	desAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil{
		return nil, err
	}

	return n.connections.OpenConn(ctx, n.transport, n.cfg.TlsConfig, n.cfg.QuicConfig, desAddr)
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

func (n *Node) Wait() error {
    return n.group.Wait()
}

func (n *Node) Events() <-chan types.Event {
    return n.events
}