package connection

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/Dishank-Sen/quicnode/constants"
	"github.com/Dishank-Sen/quicnode/internal/datagram"
	"github.com/Dishank-Sen/quicnode/internal/router"
	"github.com/Dishank-Sen/quicnode/internal/stream"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/quic-go/quic-go"
)

type Connections struct {
	router *router.Router
	numConn int
	connManager *ConnManager
	mu    sync.RWMutex
}

func NewConnections(e chan types.Event, r *router.Router) *Connections {
	return &Connections{
		router: r,
		connManager: newConnManage(e),
		numConn: 0,
	}
}

func (c *Connections) CreateConn(conn *quic.Conn) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.numConn++

	// add connection to connection manager
	addr := conn.RemoteAddr()
	peer := c.connManager.newConn(conn, addr.String())

	// handle the new connection - goroutine
	go c.handleStream(peer)
	go c.handleDatagram(peer)
}

func (c *Connections) handleStream(peer *Peer) {
	defer func() {
		log.Println("connection closed")
		c.connManager.removeEntry(peer.ID)
		log.Println("entry removed")
	}()
	
	conn := peer.conn
	for {
		log.Println("waiting for stream...")
        s, err := conn.AcceptStream(conn.Context())
        if err != nil {
            log.Printf("connection.go - %v", err)
            return
        }
		log.Println("got a stream")
		stream := stream.NewStream(conn.Context(), s, conn.RemoteAddr(), c.router.GetStreamHandler)
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
	/* it depends on the default handshake ideal timeout - 5sec 
	we can change the default handshake time*/
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
	defer c.mu.Unlock()
	c.numConn++

	go c.handleStream(peer)
	go c.handleDatagram(peer)

	return peer, nil
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