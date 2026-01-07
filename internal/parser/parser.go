package parser

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/Dishank-Sen/quicnode/types"
)

func ParseRequest(stream io.Reader) (*types.Request, error) {
	r := bufio.NewReader(stream)
	rawHeaders, err := ReadUntilDelimiter(r, []byte("\r\n\r\n"))
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(rawHeaders), "\r\n")
	parts := strings.Split(lines[0], " ")
	if len(parts) < 1 {
		return nil, errors.New("invalid request line")
	}

	req := &types.Request{
		Route: parts[0],
		Headers: make(map[string]string),
		Protocol: parts[1],
	}

	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		kv := strings.SplitN(line, ":", 2)
		if len(kv) == 2 {
			req.Headers[strings.TrimSpace(kv[0])] =
				strings.TrimSpace(kv[1])
		}
	}

	// Body only if Content-Length exists
	if cl, ok := req.Headers["Content-Length"]; ok {
		n, err := strconv.Atoi(cl)
		if err != nil {
			return nil, err
		}
		req.Body = make([]byte, n)
		if _, err := io.ReadFull(r, req.Body); err != nil {
			return nil, err
		}
	}

	return req, nil
}

func ReadUntilDelimiter(r *bufio.Reader, delim []byte) ([]byte, error) {
	var buf []byte
	match := 0

	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}

		buf = append(buf, b)

		switch b {
		case delim[match]:
			match++
			if match == len(delim) {
				return buf, nil
			}
		case delim[0]:
			match = 1
		default:
			match = 0
		}
	}
}
