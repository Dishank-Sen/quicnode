package node

import (
	"context"
	"fmt"
	"net"
	"strconv"

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
	router *router.Router
}

func NewNode(ctx context.Context, cfg Config) (*Node, error){
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

func (n *Node) Start() error{
	addr := n.cfg.ListenAddr
	tlsCfg := n.cfg.TlsConfig
	quicCfg := n.cfg.QuicConfig

	listener, err := quic.ListenAddr(addr, tlsCfg, quicCfg)
	if err != nil {
		n.cancel()
		return err
	}

	n.listener = listener

	go n.acceptLoop()
	return nil
}

func (n *Node) Stop() error{
	n.cancel()
	if n.listener != nil{
		return n.listener.Close()
	}
	return nil
}

func (n *Node) acceptLoop(){
	for{
		// fmt.Println("waiting for connection...")
		conn, err := n.listener.Accept(n.ctx)
		if err != nil{
			if n.handleConnError(err){
				return
			}
			continue
		}

		// fmt.Println("connection received")
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

	return client.Dial(n.ctx, n.cfg.TlsConfig, n.cfg.QuicConfig, req)
}

func (n *Node) errorRes() *types.Response{
	return &types.Response{
		StatusCode: 500,
		Message:    "Error",
		Body:       []byte("Internal Server Error"),
	}
}