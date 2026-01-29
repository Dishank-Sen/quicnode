package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"

	"github.com/Dishank-Sen/quicnode/constants"
	"github.com/Dishank-Sen/quicnode/internal/transport/request"
	"github.com/Dishank-Sen/quicnode/internal/transport/response"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/quic-go/quic-go"
)

type Client struct{
	connections map[string]*quic.Conn
	connMu sync.Mutex
}

func NewClient() *Client{
	return &Client{
		connections: make(map[string]*quic.Conn),
	}
}

func (c *Client) Dial(ctx context.Context, tr *quic.Transport, tlsCfg *tls.Config, quicCfg *quic.Config, req *types.Request) (*types.Response, error){
	// IMPORTANT: use context with timeout for dial
	dialCtx, dialCancel := context.WithTimeout(ctx, constants.QuicDialTimeout)
	defer dialCancel()

	var conn *quic.Conn
	key := addrKey(req.DestinationAddr)
	c.connMu.Lock()
	existingConn, ok := c.connections[key]
	c.connMu.Unlock()
	if !ok{
		newConn, err := tr.Dial(
			dialCtx,
			req.DestinationAddr,
			tlsCfg,
			quicCfg,
		)
		if err != nil{
			log.Println(err)
			return c.errorRes(), err
		}
		c.connMu.Lock()
		if existing, ok := c.connections[key]; ok {
			c.connMu.Unlock()
			_ = newConn.CloseWithError(0, "duplicate dial")
			conn = existing
		} else {
			c.connections[key] = newConn
			c.connMu.Unlock()
			conn = newConn
			go c.handleConnClose(conn, key)
		}
	}else{
		conn = existingConn
	}

	streamCtx, streamCancel := context.WithTimeout(ctx, constants.QuicStreamTimeout)
	defer streamCancel()

	stream, err := conn.OpenStreamSync(streamCtx)
	if err != nil {
		return c.errorRes(), err
	}
	defer stream.Close()

	if err := request.WriteRequest(stream, req); err != nil {
		log.Println("write failed:", err)
		return c.errorRes(), err
	}

	resp, err := response.ReadResponse(stream)
	if err != nil {
		log.Println("read failed:", err)
		return c.errorRes(), err
	}

	return resp, nil

}

func (c *Client) DialConn(ctx context.Context, conn *quic.Conn, req *types.Request) (*types.Response, error){
	if conn == nil{
		return c.errorRes(), fmt.Errorf("connection object is nil")
	}
	streamCtx, streamCancel := context.WithTimeout(ctx, constants.QuicStreamTimeout)
	defer streamCancel()

	stream, err := conn.OpenStreamSync(streamCtx)
	if err != nil {
		return c.errorRes(), err
	}
	defer stream.Close()

	if err := request.WriteRequest(stream, req); err != nil {
		log.Println("write failed:", err)
		return c.errorRes(), err
	}

	resp, err := response.ReadResponse(stream)
	if err != nil {
		log.Println("read failed:", err)
		return c.errorRes(), err
	}

	return resp, nil
}

func (c *Client) errorRes() *types.Response{
	return &types.Response{
		StatusCode: 500,
		Message:    "Error",
		Body:       []byte("Internal Server Error"),
	}
}

func (c *Client) Shutdown(){
	log.Println("client connection closing....")
	c.connMu.Lock()
	defer c.connMu.Unlock()
	for i, conn := range c.connections{
		log.Printf("connection %v closing", i)
		_ = conn.CloseWithError(0, "node shutdown (client)")
	}
}

func (c *Client) handleConnClose(conn *quic.Conn, addr string){
	<-conn.Context().Done()
	log.Println("connection closed (client)")
	c.connMu.Lock()
	delete(c.connections, addr)
	c.connMu.Unlock()
}

func (c *Client) printConn(){
	c.connMu.Lock()
	defer c.connMu.Unlock()
	log.Println("connections:")
	for addr := range c.connections{
		log.Println(addr)
	}
}

func addrKey(addr net.Addr) string {
    if ua, ok := addr.(*net.UDPAddr); ok {
        return ua.IP.String() + ":" + strconv.Itoa(ua.Port)
    }
    return addr.String()
}
