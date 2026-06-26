package datagram

import (
	"context"
	"log"
	"net"
	"strings"

	"github.com/Dishank-Sen/quicnode/internal/parser"
)

type Context interface {
    Route() string
    Payload() []byte
    PeerAddr() string
}

type HandlerFunc func(ctx Context)

type Datagram struct {
    payload []byte
    route string
    peerAddr string
    dispatch func(route string) (HandlerFunc, bool)
	ctx      context.Context
}

func NewDatagram(ctx context.Context, payload []byte, addr net.Addr, dispatch func(route string) (HandlerFunc, bool)) *Datagram{
    return &Datagram{
        ctx: ctx,
        payload: payload,
        peerAddr: addr.String(),
        dispatch: dispatch,
    }
}

func (d *Datagram) Handle() {
    select{
	case <-d.ctx.Done():
		return
	default:
	}

    req, err := parser.ParseDatagramFrame(d.payload)
    if err != nil {
        log.Printf("error in parsing datagram: %v", err)
        return
    }
    route := strings.TrimSpace(req.Route)
	d.route = route
	d.payload = req.Payload

	// check if req is valid
	if(len(route) == 0){
		log.Printf("invalid route")
		return
	}

	// handler
	h, ok := d.dispatch(route)
	if !ok {
		log.Printf("no handler for that route")
		return
	}

	h(d)
}

func (d *Datagram) Route() string { return d.route }

func (d *Datagram) Payload() []byte { return d.payload }

func (d *Datagram) PeerAddr() string { return d.peerAddr }