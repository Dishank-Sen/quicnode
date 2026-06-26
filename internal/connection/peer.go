package connection

import (
	"github.com/Dishank-Sen/quicnode/internal/parser"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/quic-go/quic-go"
)

type Peer struct {
    conn *quic.Conn
    Addr string
    ID   types.ConnID
}

func newPeer(conn *quic.Conn, addr string, id types.ConnID) *Peer {
    return &Peer{
        conn: conn,
        Addr: addr,
        ID: id,
    }
}

func (p *Peer) Send(route string, payload []byte) (<-chan []byte, error) {
    stream, err := p.conn.OpenStream()
    if err != nil {
        return nil, err
    }

    frame, err := parser.SerializeRequest(route, payload)
    if err != nil {
        stream.Close()
        return nil, err
    }
    if _, err := stream.Write(frame); err != nil {
        stream.Close()
        return nil, err
    }

    // CRITICAL: Close the write side after sending request
    // This signals EOF to the server that we're done sending
    // But we can still read the response (half-close)
    // Without this, streams accumulate and hit "too many open streams"
    if err := stream.Close(); err != nil {
        return nil, err
    }

    ch := make(chan []byte)
    go func() {
        defer close(ch)
        defer stream.Close()
        for {
            payload, err := parser.ParseResponse(stream) // exact allocation
            if len(payload) > 0 {
                select{
                case ch <- payload:
                case <- p.conn.Context().Done():
                    return
                }
            }
            if err != nil {
                return
            }
        }
    }()

    return ch, nil
}

// SendDatagram sends an unreliable, unordered datagram over the QUIC connection
// Datagrams are fire-and-forget: no acknowledgment, no retransmission, no ordering
// Returns error if datagram couldn't be sent (but NOT if it was lost in transit)
//
// Use cases:
//   - Real-time game state updates
//   - Telemetry/metrics
//   - Heartbeats/pings
//   - Any data where loss is acceptable
func (p *Peer) SendDatagram(route string, payload []byte) error {
	frame, err := parser.SerializeDatagramFrame(route, payload)
	if err != nil {
		return err
	}

	// SendDatagram returns error if the datagram queue is full
	// It does NOT guarantee delivery - datagrams can be lost
	return p.conn.SendDatagram(frame)
}

func (p *Peer) Close(){
	p.conn.CloseWithError(0, "peer closed the connection")
}