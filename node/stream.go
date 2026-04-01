package node

import (
	"context"
	"fmt"
	"log"

	"github.com/Dishank-Sen/quicnode/internal/parser"
	"github.com/Dishank-Sen/quicnode/internal/transport/response"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/quic-go/quic-go"
)

func (n *Node) handleStream(stream *quic.Stream, conn *quic.Conn, connID types.ConnID){
	// can add explicit condition when in ongoing stream connection context is cancelled.
	defer stream.Close()

	req, err := parser.ParseRequest(stream)
	if err != nil {
		log.Println(fmt.Errorf("error in parsing: %v", err))
		// best: send 400 response instead of silent return
		_ = response.WriteResponse(stream, &types.Response{
			StatusCode: 400,
			Message:    "Bad Request",
			Headers:    nil,
			Body:       []byte(err.Error()),
		})
		return
	}
	req.SourceAddr = conn.RemoteAddr()
	req.Conn = conn
	reqCtx := context.WithValue(context.Background(), "connID", connID)

	resp := n.router.Dispatch(reqCtx, req)
	if err = response.WriteResponse(stream, resp); err != nil{
		log.Printf("stream.go - error in writing response: %v", err)
		return
	}
}