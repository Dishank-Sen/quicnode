package node

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/Dishank-Sen/quicnode/internal/client"
	"github.com/Dishank-Sen/quicnode/internal/router"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/quic-go/quic-go"
)

type Node struct{
	cfg Config
	ctx context.Context
	cancel context.CancelFunc
	listener *quic.Listener
	transport *quic.Transport
	router *router.Router
	udpConn *net.UDPConn
	once sync.Once
	connsMu sync.Mutex
    conns   map[*quic.Conn]struct{}
	client *client.Client
}

func NewNode(ctx context.Context, cfg Config) (*Node, error){
	if ctx == nil{
		return nil, fmt.Errorf("context is nil")
	}
	ctx, cancel := context.WithCancel(ctx)
	if err := checkConfig(cfg); err != nil{
		cancel()
		return nil, err
	}
	r := router.NewRouter()
	n := &Node{
		cfg: cfg,
		ctx: ctx,
		cancel: cancel,
		router: r,
		conns: make(map[*quic.Conn]struct{}),
		client: client.NewClient(),
	}
	go func(ctx context.Context) {
		<- ctx.Done()
		n.once.Do(n.shutdown)
	}(ctx)

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

func (n *Node) Start() error{
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

	go n.acceptLoop()
	return nil
}

func (n *Node) Stop() error{
	n.once.Do(n.shutdown)
	return nil
}

func (n *Node) acceptLoop(){
	for{
		log.Println("waiting for connection...")
		conn, err := n.listener.Accept(n.ctx)
		if err != nil{
			log.Println(err)
			if n.handleConnError(err){
				log.Printf("error in listening: %v", err)
				n.once.Do(n.shutdown)
				return
			}
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
			return
		}

		log.Println("handshake complete")

		n.connsMu.Lock()
		n.conns[conn] = struct{}{}
		n.connsMu.Unlock()

		go n.handleSession(conn)
	}
}

func (n *Node) Handle(route string, h router.HandlerFunc){
	n.router.AddRoute(route, h)
}

func (n *Node) Dial(addr string, route string, headers map[string]string, body []byte) (*types.Response, error){
	if err := validateListenAddr(addr); err != nil{
		return nil,err
	}

	desAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil{
		return n.errorRes(), err
	}

	req := &types.Request{
		Route: route,
		DestinationAddr: desAddr,
		Headers: headers,
		Body: body,
	}

	return n.client.Dial(n.ctx, n.transport, n.cfg.TlsConfig, n.cfg.QuicConfig, req)
}

func (n *Node) DialConn(addr string, conn *quic.Conn, route string, headers map[string]string, body []byte) (*types.Response, error){
	desAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil{
		return n.errorRes(), err
	}

	req := &types.Request{
		Route: route,
		DestinationAddr: desAddr,
		Headers: headers,
		Body: body,
	}

	return n.client.DialConn(n.ctx, conn, req)
}

func (n *Node) errorRes() *types.Response{
	return &types.Response{
		StatusCode: 500,
		Message:    "Error",
		Body:       []byte("Internal Server Error"),
	}
}

func (n *Node) shutdown(){
	log.Println("node shutting down")
	// important: as listener error still keeps the udp socket open 
	// and it may cause socket leak or port already in use issue later on.
	n.cancel()

	if n.listener != nil {
        _ = n.listener.Close()
    }

	n.connsMu.Lock()
    for c := range n.conns {
        _ = c.CloseWithError(0, "node shutdown (server)")
    }
    n.connsMu.Unlock()

	if n.udpConn != nil {
        _ = n.udpConn.Close()
    }

	n.client.Shutdown()
}