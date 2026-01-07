package node

import "github.com/quic-go/quic-go"

func (n *Node) handleSession(conn *quic.Conn){
	for{
		stream, err := conn.AcceptStream(n.ctx)
		if err != nil{
			if n.handleStreamError(err){
				return
			}
			continue
		}
		go n.handleStream(stream, conn.RemoteAddr())
	}
}