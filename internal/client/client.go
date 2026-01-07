package client

import (
	"context"
	"crypto/tls"

	"github.com/Dishank-Sen/quicnode/constants"
	"github.com/Dishank-Sen/quicnode/internal/transport/request"
	"github.com/Dishank-Sen/quicnode/internal/transport/response"
	"github.com/Dishank-Sen/quicnode/types"
	"github.com/quic-go/quic-go"
)

func Dial(ctx context.Context, tlsCfg *tls.Config, quicCfg *quic.Config, req *types.Request) (*types.Response, error){
	// IMPORTANT: use context with timeout for dial
	dialCtx, dialCancel := context.WithTimeout(context.Background(), constants.QuicDialTimeout)
	defer dialCancel()
	conn, err := quic.DialAddr(
		dialCtx,
		req.DestinationAddr.String(),
		tlsCfg,
		quicCfg,
	)
	if err != nil{
		dialCancel()
		return errorRes(), err
	}

	streamCtx, streamCancel := context.WithTimeout(ctx, constants.QuicStreamTimeout)
	defer streamCancel()
	stream, err := conn.OpenStreamSync(streamCtx)
	if err != nil {
		return errorRes(), err
	}
	defer stream.Close()

	if err := request.WriteRequest(stream, req); err != nil {
		return errorRes(), err
	}

	resp, err := response.ReadResponse(stream)
	if err != nil {
		return errorRes(), err
	}

	return resp, nil

}

func errorRes() *types.Response{
	return &types.Response{
		StatusCode: 500,
		Message:    "Error",
		Body:       []byte("Internal Server Error"),
	}
}