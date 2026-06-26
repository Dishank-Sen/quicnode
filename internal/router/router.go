package router

import (
	"strings"
	"sync"

	"github.com/Dishank-Sen/quicnode/internal/datagram"
	"github.com/Dishank-Sen/quicnode/internal/stream"
)

type Router struct{
	streamRoutes map[string]stream.HandlerFunc
	datagramRoutes map[string]datagram.HandlerFunc
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
	r.mu.Lock()
    defer r.mu.Unlock()
    r.datagramRoutes[route] = h
}

func (r *Router) GetStreamHandler(route string) (stream.HandlerFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
    h, ok := r.streamRoutes[route]
    return h, ok
}

func (r *Router) GetDatagramHandler(route string) (datagram.HandlerFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
    h, ok := r.datagramRoutes[route]
    return h, ok
}