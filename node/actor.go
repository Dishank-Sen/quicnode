package node

import (
	"log"

	"github.com/Dishank-Sen/quicnode/types"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/quic-go/quic-go"
)

type ConnOpened struct {
    ConnID types.ConnID
    PID    *actor.PID
}

type ConnClosed struct {
    ConnID types.ConnID
    PID    *actor.PID
    Err    error
}

type StreamMsg struct {
    Stream quic.Stream
}

type ConnPool struct{
	conns map[types.ConnID]*actor.PID
	events chan types.Event
}

func NewConnPool(events chan types.Event) actor.Actor{
	return &ConnPool{
		conns: map[types.ConnID]*actor.PID{},
		events: events,
	}
}

func (c *ConnPool) Receive(ctx actor.Context){
	switch msg := ctx.Message().(type) {

    case *ConnOpened:
		log.Println("actor.go - conn opened:", msg.ConnID)
        c.conns[msg.ConnID] = msg.PID
		c.events <- types.Event{
            Type:   types.EventConnOpened,
            ConnID: msg.ConnID,
        }

    case *ConnClosed:
        log.Println("actor.go - conn closed:", msg.ConnID, "err:", msg.Err)
        delete(c.conns, msg.ConnID)
		c.events <- types.Event{
            Type:   types.EventConnClosed,
            ConnID: msg.ConnID,
            Err:    msg.Err,
        }
    }
}

type ConnActor struct {
    conn *quic.Conn
    connID types.ConnID
    poolPID *actor.PID
}

func NewConnActor(conn *quic.Conn, connID types.ConnID, poolPID *actor.PID) actor.Actor {
    return &ConnActor{
		conn: conn,
		connID: connID,
		poolPID: poolPID,
	}
}

func (c *ConnActor) Receive(ctx actor.Context){

}
