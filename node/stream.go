package node

import (
	"net"

	"github.com/Dishank-Sen/quicnode/internal/parser"
	"github.com/Dishank-Sen/quicnode/internal/transport/response"
	"github.com/quic-go/quic-go"
)

func (n *Node) handleStream(stream *quic.Stream, addr net.Addr){
	defer stream.Close()

	req, err := parser.ParseRequest(stream)
	req.SourceAddr = addr  // attach peer address to req
	if err != nil {
		return
	}

	resp := n.router.Dispatch(req)
	if err = response.WriteResponse(stream, resp); err != nil{
		stream.Close()
	}
}