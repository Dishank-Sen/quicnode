package request

import (
	"fmt"
	"io"

	"github.com/Dishank-Sen/quicnode/types"
)

func WriteRequest(w io.Writer, req *types.Request) error{
	// Request line
	if _, err := fmt.Fprintf(w, "%s %s\r\n", req.Route, req.Protocol); err != nil{
		return err
	}

	// Body framing
	if len(req.Body) > 0 {
		if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n", len(req.Body)); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "Destination-Address: %d\r\n", req.DestinationAddr); err != nil {
		return err
	}

	// headers
	for k, v := range req.Headers {
		if _, err := fmt.Fprintf(w, "%s: %s\r\n", k, v); err != nil {
			return err
		}
	}

	// header/body delimiter
	if _, err := fmt.Fprint(w, "\r\n"); err != nil {
		return err
	}

	// Body
	if len(req.Body) > 0 {
		if _, err := w.Write(req.Body); err != nil {
			return err
		}
	}
	return nil
}