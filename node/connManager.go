package node

import (
	"fmt"
	"log"
	"sync"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/quic-go/quic-go"
)

type ConnManager struct {
    mu     sync.RWMutex
    byID   map[types.ConnID]*ConnMeta
    byAddr map[string]*ConnMeta
    events chan types.Event
}

type ConnMeta struct {
	ConnID types.ConnID
	Conn   *quic.Conn
	Addr   string
}

func newConnManage(e chan types.Event) *ConnManager{
    return &ConnManager{
        byID: make(map[types.ConnID]*ConnMeta),
        byAddr: make(map[string]*ConnMeta),
        events: e,
    }
}

func (c *ConnManager) newConn(cm *ConnMeta) {
	c.mu.Lock()
	defer c.mu.Unlock()

	existing, ok := c.byAddr[cm.Addr]
	if ok {
		// duplicate connection detected
		if shouldReplace(existing, cm) {
			existing.Conn.CloseWithError(0, "duplicate connection")
			delete(c.byID, existing.ConnID)
		} else {
			cm.Conn.CloseWithError(0, "duplicate connection")
			return
		}
	}

	c.byID[cm.ConnID] = cm
	c.byAddr[cm.Addr] = cm

	// emit event safely
	select {
	case c.events <- types.Event{
		Type:   types.EventConnOpened,
		ConnID: cm.ConnID,
	}:
	default:
		log.Println("event dropped")
	}
}

func shouldReplace(old, new *ConnMeta) bool {
	return new.ConnID < old.ConnID
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

func (c *ConnManager) getConn(addr string) (*quic.Conn, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	meta, ok := c.byAddr[addr]
	if !ok {
		return nil, false
	}
	return meta.Conn, true
}

func (c *ConnManager) getAllConn() []*quic.Conn{
	c.mu.RLock()
	defer c.mu.RUnlock()
	var conns []*quic.Conn

	for _, v := range c.byID{
		conns = append(conns, v.Conn)
	}
	return conns
}