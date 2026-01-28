package node

import (
	"log"
	"github.com/quic-go/quic-go"
)

func (n *Node) handleSession(conn *quic.Conn){
	for{
		select {
		case <-conn.Context().Done():
			log.Println("conn ended:", conn.Context().Err())
			n.connsMu.Lock()
			delete(n.conns, conn)
			n.connsMu.Unlock()
			return
		case <-n.ctx.Done():
			return
		default:
		}
		stream, err := conn.AcceptStream(n.ctx)
		if err != nil{
			if n.handleStreamError(err){
				return
			}
			continue
		}
		go n.handleStream(stream, conn)
	}
}