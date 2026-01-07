package node

import (
	"crypto/tls"

	"github.com/quic-go/quic-go"
)

type Config struct{
	ListenAddr string
	TlsConfig *tls.Config
	QuicConfig *quic.Config
}