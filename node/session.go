package node

import (
	"log"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/quic-go/quic-go"
)

func (n *Node) handleSession(conn *quic.Conn, connID types.ConnID, pid *actor.PID) error {
    defer func() {
		log.Printf("session.go - connection closing : %s", connID)

        n.root.Send(n.poolPID, &ConnClosed{
            ConnID: connID,
            PID: pid,
            Err: conn.Context().Err(),
        })
		
        n.connsMu.Lock()
        delete(n.conns, conn)
        n.connsMu.Unlock()

        n.root.Stop(pid)
    }()

    for {
        stream, err := conn.AcceptStream(conn.Context())
        if err != nil {
            log.Printf("session.go - %v", err)
            return nil
        }

        go n.handleStream(stream, conn, connID)
    }
}