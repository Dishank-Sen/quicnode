package types

import "net"

type Request struct{
	Route   string
	Headers map[string]string
	SourceAddr    net.Addr
	DestinationAddr net.Addr
	Body    []byte
	Protocol string
}

type Response struct {
	StatusCode int
	Message    string
	Headers    map[string]string
	Body       []byte
}