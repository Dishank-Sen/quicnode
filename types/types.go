package types

import (
	"net"

	"github.com/quic-go/quic-go"
)

type Request struct{
	Route   string
	Headers map[string]string
	SourceAddr    net.Addr
	DestinationAddr net.Addr
	Body    []byte
	Protocol string
	Conn *quic.Conn
}

type Response struct {
	StatusCode int
	Message    string
	Headers    map[string]string
	Body       []byte
}