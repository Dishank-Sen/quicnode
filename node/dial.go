package node

import (
	"context"
	"crypto/tls"
	"log"

	"github.com/Dishank-Sen/quicnode/constants"
	"github.com/Dishank-Sen/quicnode/internal/transport/request"
	"github.com/Dishank-Sen/quicnode/internal/transport/response"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
)

func (n *Node) dial(tr *quic.Transport, tlsCfg *tls.Config, quicCfg *quic.Config, req *types.Request) (*types.Response, error){
	// Don't try to close connecetion here, it is managed by node
	// IMPORTANT: use context with timeout for dial
	dialCtx, dialCancel := context.WithTimeout(n.ctx, constants.QuicDialTimeout)
	defer dialCancel()

	var conn *quic.Conn
	retrievedConn, ok := n.connManager.getConn(req.DestinationAddr.String())
	if ok{
		conn = retrievedConn
	}else{
		newConn, err := tr.Dial(
			dialCtx,
			req.DestinationAddr,
			tlsCfg,
			quicCfg,
		)
		if err != nil{
			log.Println(err)
			return errorRes(), err
		}
		conn = newConn
		connID := types.ConnID(uuid.New().String())
		cm := &ConnMeta{
			ConnID: connID,
			Conn: conn,
			Addr: req.DestinationAddr.String(),
		}
		n.connManager.newConn(cm)
		n.group.Go(func() error{
			return n.handleConnClose(conn, connID)
		})
	}
		
	streamCtx, streamCancel := context.WithTimeout(n.ctx, constants.QuicStreamTimeout)
	defer streamCancel()

	stream, err := conn.OpenStreamSync(streamCtx)
	if err != nil {
		return errorRes(), err
	}
	defer stream.Close()

	if err := request.WriteRequest(stream, req); err != nil {
		log.Println("write failed:", err)
		return errorRes(), err
	}

	resp, err := response.ReadResponse(stream)
	if err != nil {
		log.Println("read failed:", err)
		return errorRes(), err
	}

	return resp, nil

}

func (n *Node) handleConnClose(conn *quic.Conn, connID types.ConnID) error{
	<-conn.Context().Done()
	log.Printf("dial.go - connection context cancelled : %v", conn.Context().Err())
	n.connManager.removeEntry(connID)
	return nil
}

func errorRes() *types.Response{
	return &types.Response{
		StatusCode: 500,
		Message:    "Error",
		Body:       []byte("Internal Server Error"),
	}
}