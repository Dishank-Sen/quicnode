package router

import (
	"strings"
	"sync"

	"github.com/Dishank-Sen/quicnode/internal/datagram"
	"github.com/Dishank-Sen/quicnode/internal/stream"
)

const (
    routeNoiseHandshake = "_noise"      // handshake stream
    routePing           = "_ping"       // internal keepalive
    routePeerExchange   = "_peers"      // peer list exchange (hoppeer)
)

type Router struct{
	streamRoutes map[string]stream.HandlerFunc
	datagramRoutes map[string]datagram.HandlerFunc
	internalStreamRoutes map[string]stream.HandlerFunc
	mu sync.RWMutex
}

func NewRouter() *Router {
	return &Router{
		streamRoutes: make(map[string]stream.HandlerFunc),
		datagramRoutes: make(map[string]datagram.HandlerFunc),
	}
}

func (r *Router) StreamRoute(route string, h stream.HandlerFunc) {
	route = strings.TrimSpace(route)
    if h == nil {
        panic("quicnode: nil handler registered for stream route: " + route)
    }
	if len(route) == 0 {
		panic("quicnode: empty route")
	}
	if strings.HasPrefix(route, "_") {
        panic("quicnode: route prefix _ is reserved for internal use")
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    r.streamRoutes[route] = h
}

func (r *Router) DatagramRoute(route string, h datagram.HandlerFunc) {
	route = strings.TrimSpace(route)
	if h == nil {
		panic("quicnode: nil handler registered for datagram route: " + route)
	}
	if len(route) == 0 {
		panic("quicnode: empty route")
	}
	if strings.HasPrefix(route, "_") {
        panic("quicnode: route prefix _ is reserved for internal use")
    }
	r.mu.Lock()
    defer r.mu.Unlock()
    r.datagramRoutes[route] = h
}

func (r *Router) InternalStreamRoute(route string, h stream.HandlerFunc) {
	route = strings.TrimSpace(route)
    if h == nil {
        panic("quicnode: nil handler registered for stream route: " + route)
    }
	if len(route) == 0 {
		panic("quicnode: empty route")
	}
	if !strings.HasPrefix(route, "_") {
        panic("quicnode: route must have prefix _ for internal use")
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    r.internalStreamRoutes[route] = h
}

func (r *Router) GetStreamHandler(route string) (stream.HandlerFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
    h, ok := r.streamRoutes[route]
    return h, ok
}

func (r *Router) GetInternalStreamHandler(route string) (stream.HandlerFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
    h, ok := r.internalStreamRoutes[route]
    return h, ok
}

func (r *Router) GetDatagramHandler(route string) (datagram.HandlerFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
    h, ok := r.datagramRoutes[route]
    return h, ok
}