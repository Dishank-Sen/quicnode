package node

import (
	"fmt"
	"log"
	"github.com/Dishank-Sen/quicnode/internal/parser"
	"github.com/Dishank-Sen/quicnode/internal/transport/response"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/quic-go/quic-go"
)

func (n *Node) handleStream(stream *quic.Stream, conn *quic.Conn){
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

	resp := n.router.Dispatch(req)
	if err = response.WriteResponse(stream, resp); err != nil{
		log.Println(fmt.Errorf("error in writing response: %v", err))
		return
	}
}