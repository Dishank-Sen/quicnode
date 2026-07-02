package connection

import (
	"fmt"
	"log"
	"sync"

	"github.com/Dishank-Sen/quicnode/types"
	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
)

type ConnManager struct {
    mu     sync.RWMutex
    byID   map[types.ConnID]*ConnMeta
    byAddr map[string]*ConnMeta
    events chan types.Event
}

func newConnManage(e chan types.Event) *ConnManager{
    return &ConnManager{
        byID: make(map[types.ConnID]*ConnMeta),
        byAddr: make(map[string]*ConnMeta),
        events: e,
    }
}

// connmanager.go
type ConnMeta struct {
    ConnID types.ConnID
    Conn   *quic.Conn
    Addr   string
    Peer   *Peer
}

func (c *ConnManager) newConn(conn *quic.Conn, addr string) *Peer {
    c.mu.Lock()
    defer c.mu.Unlock()

    connID := types.ConnID(uuid.New().String())
    peer := newPeer(conn, addr, connID)
    cm := &ConnMeta{
        ConnID: connID,
        Conn:   conn,
        Addr:   addr,
        Peer:   peer,
    }

    // Check if there's an existing connection from this address
    existing, ok := c.byAddr[addr]
    if ok {
        // Check if existing connection is still alive
        select {
        case <-existing.Conn.Context().Done():
            // Existing connection is dead, remove it
            log.Printf("Replacing dead connection from %s", addr)
            delete(c.byID, existing.ConnID)
            delete(c.byAddr, addr)
        default:
            // Existing connection is alive
            // Close the OLD connection and use the NEW one
            log.Printf("Duplicate connection from %s - closing old connection", addr)
            existing.Conn.CloseWithError(0, "replaced by new connection")
            delete(c.byID, existing.ConnID)
            // Will be replaced below with new connection
        }
    }

    c.byID[cm.ConnID] = cm
    c.byAddr[cm.Addr] = cm

    select {
    case c.events <- types.Event{
        Type:   types.EventConnOpened,
        ConnID: cm.ConnID,
    }:
    default:
        log.Println("event dropped")
    }

    return peer
}

func (c *ConnManager) getPeer(addr string) (*Peer, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    meta, ok := c.byAddr[addr]
    if !ok {
        return nil, false
    }
    return meta.Peer, true
}

func (c *ConnManager) getAllPeers() []*Peer {
    c.mu.RLock()
    defer c.mu.RUnlock()

    var peers []*Peer
    for _, v := range c.byID {
        peers = append(peers, v.Peer)
    }
    return peers
}

func (c *ConnManager) removeEntry(connID types.ConnID){
	c.mu.Lock()
	defer c.mu.Unlock()
	cm, ok := c.byID[connID]
	if !ok{
		// not exist in id map
		log.Println("connection id not exist in connection manager")
		return
	}
	delete(c.byID, connID)
	delete(c.byAddr, cm.Addr)

	event := types.Event{
		Type: types.EventConnClosed,
		ConnID: connID,
		Err: fmt.Errorf("connection context cancelled"),
	}
	select {
	case c.events <- event:
	default:
		log.Println("event dropped")
	}
}