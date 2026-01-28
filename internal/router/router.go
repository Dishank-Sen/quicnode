package router

import (
	"github.com/Dishank-Sen/quicnode/types"
)

type HandlerFunc func(req *types.Request) *types.Response

type routeKey struct {
	route string
}

type Router struct{
	routes map[routeKey]HandlerFunc
}

func NewRouter() *Router {
	return &Router{
		routes: make(map[routeKey]HandlerFunc),
	}
}

func (r *Router) AddRoute(route string, h HandlerFunc){
	r.routes[routeKey{route: route}] = h
}

func (r *Router) Dispatch(req *types.Request) *types.Response {
	h, ok := r.routes[routeKey{route: req.Route}]
	if !ok {
		return &types.Response{
			StatusCode: 404,
			Message:    "Not Found",
			Body:       []byte("route not found"),
		}
	}
	return h(req)
}
