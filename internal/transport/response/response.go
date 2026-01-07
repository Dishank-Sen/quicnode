package response

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Dishank-Sen/quicnode/internal/parser"
	"github.com/Dishank-Sen/quicnode/types"
)

func WriteResponse(w io.Writer, resp *types.Response) error {
	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}
	resp.Headers["Content-Length"] = fmt.Sprintf("%d", len(resp.Body))

	if _, err := fmt.Fprintf(w, "%d %s\r\n", resp.StatusCode, resp.Message); err != nil {
		return err
	}

	for k, v := range resp.Headers {
		if _, err := fmt.Fprintf(w, "%s: %s\r\n", k, v); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprint(w, "\r\n"); err != nil {
		return err
	}

	_, err := w.Write(resp.Body)
	return err
}

func ReadResponse(r io.Reader) (*types.Response, error) {
	reader := bufio.NewReader(r)

	rawHeaders, err := parser.ReadUntilDelimiter(reader, []byte("\r\n\r\n"))
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(rawHeaders), "\r\n")
	status := strings.SplitN(lines[0], " ", 2)

	resp := &types.Response{
		StatusCode: atoi(status[0]),
		Message:    status[1],
		Headers:    make(map[string]string),
	}

	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		kv := strings.SplitN(line, ":", 2)
		if len(kv) == 2 {
			resp.Headers[strings.TrimSpace(kv[0])] =
				strings.TrimSpace(kv[1])
		}
	}

	if cl, ok := resp.Headers["Content-Length"]; ok {
		n, err := strconv.Atoi(cl)
		if err != nil {
			return nil, err
		}

		resp.Body = make([]byte, n)
		if _, err := io.ReadFull(reader, resp.Body); err != nil {
			return nil, err
		}
	}

	return resp, nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}