package stream

import (
	"context"
	"log"
	"net"
	"strings"

	"github.com/Dishank-Sen/quicnode/internal/parser"
	"github.com/quic-go/quic-go"
)

// Context provides access to an incoming stream request.
// Methods allow reading the request and writing responses.
type Context interface {
    Write([]byte) (int, error)
    Route() string
    Payload() []byte
    PeerAddr() string

    // PeerPublicKey returns the authenticated peer's Noise public key (32 bytes).
    // Returns nil if RequireAuth is disabled or handshake hasn't completed.
    PeerPublicKey() []byte

    // PeerID returns the authenticated peer's stable identifier.
    // Computed as base58(sha256(PublicKey)).
    // Returns empty string if RequireAuth is disabled or handshake hasn't completed.
    PeerID() string
}

type HandlerFunc func(ctx Context)

type Stream struct {
    stream   *quic.Stream
    route    string
    payload  []byte
    peerAddr string
    dispatch func(route string) (HandlerFunc, bool)
	internalDispatch func(route string) (HandlerFunc, bool)
	ctx      context.Context

	// Authentication info (populated from Peer if RequireAuth enabled)
	peerPublicKey []byte
	peerID        string
}

func NewStream(ctx context.Context, stream *quic.Stream, addr net.Addr, dispatch func(route string) (HandlerFunc, bool), internalDispatch func(route string) (HandlerFunc, bool), peerPublicKey []byte, peerID string) *Stream{
	return &Stream{
		stream: stream,
		dispatch: dispatch,
		internalDispatch: internalDispatch,
		peerAddr: addr.String(),
		ctx: ctx,
		peerPublicKey: peerPublicKey,
		peerID: peerID,
	}
}

func (s *Stream) Handle(){
	cancelled := false
    defer func() {
        if !cancelled {
            s.stream.Close()
        }
        log.Println("stream closed")
    }()

	select{
	case <-s.ctx.Done():
		return
	default:
	}

	req, err := parser.ParseRequest(s.stream)
	if err != nil{
		log.Printf("error while parsing req: %v", err)
		s.stream.CancelWrite(1)
		s.stream.CancelRead(1)
		cancelled = true
		return
	}

	route := strings.TrimSpace(req.Route)
	s.route = route
	s.payload = req.Payload

	// check if req is valid
	if(len(route) == 0){
		log.Printf("invalid route")
		s.Write([]byte("error: invalid route"))
		return
	}

	var h HandlerFunc
	var ok bool

	if strings.HasPrefix(route, "_") {
		h, ok = s.internalDispatch(route)
		if !ok {
			log.Printf("no internal handler for that route")
			s.Write([]byte("error: no internal handler registered for this route"))
			return
		}
	}else{
		h, ok = s.dispatch(route)
		if !ok {
			log.Printf("no handler for that route")
			s.Write([]byte("error: no handler registered for this route"))
			return
		}
	}

	h(s)
	// Handler completed - defer will close the stream
}

func (s *Stream) Route() string { return s.route }

func (s *Stream) Payload() []byte { return s.payload }

func (s *Stream) PeerAddr() string { return s.peerAddr }

func (s *Stream) Write(data []byte) (int, error) {
    frame, err := parser.SerializeResponse(data)
    if err != nil {
        return 0, err
    }
    return s.stream.Write(frame)
}

func (s *Stream) PeerPublicKey() []byte {
	return s.peerPublicKey
}

func (s *Stream) PeerID() string {
	return s.peerID
}