package node

import (
	"log"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/quic-go/quic-go"
)

func (n *Node) handleSession(conn *quic.Conn, connID types.ConnID) error {
    defer func() {
		log.Printf("session.go - connection closing : %s", connID)
        n.connManager.removeEntry(connID)
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