package parser

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/Dishank-Sen/quicnode/types"
)

func ParseRequest(stream io.Reader) (*types.Request, error) {
	var routeLenBuf [2]byte
	if _, err := io.ReadFull(stream, routeLenBuf[:]); err != nil{
		return nil, err
	}
	routeLen := binary.BigEndian.Uint16(routeLenBuf[:])

	routeBuf := make([]byte, routeLen)
	if _, err := io.ReadFull(stream, routeBuf[:]); err != nil{
		return nil, err
	}

	route := string(routeBuf)

    var payloadLenBuf [4]byte
    if _, err := io.ReadFull(stream, payloadLenBuf[:]); err != nil {
        return nil, err
    }
    payloadLen := binary.BigEndian.Uint32(payloadLenBuf[:])

    payload := make([]byte, payloadLen)
    if _, err := io.ReadFull(stream, payload); err != nil{
		return nil, err
	}
	
	return &types.Request{Route: route, Payload: payload}, nil
}

func SerializeRequest(route string, payload []byte) ([]byte, error) {
    buf := new(bytes.Buffer)

    if err := binary.Write(buf, binary.BigEndian, uint16(len(route))); err != nil {
        return nil, err
    }
    buf.WriteString(route)

    if err := binary.Write(buf, binary.BigEndian, uint32(len(payload))); err != nil {
        return nil, err
    }
    buf.Write(payload)

    return buf.Bytes(), nil
}

func SerializeResponse(payload []byte) ([]byte, error) {
    buf := new(bytes.Buffer)
    if err := binary.Write(buf, binary.BigEndian, uint32(len(payload))); err != nil {
        return nil, err
    }
    buf.Write(payload)
    return buf.Bytes(), nil
}

func ParseResponse(stream io.Reader) ([]byte, error) {
    var lenBuf [4]byte
    if _, err := io.ReadFull(stream, lenBuf[:]); err != nil {
        return nil, err
    }
    payloadLen := binary.BigEndian.Uint32(lenBuf[:])

    payload := make([]byte, payloadLen) // exact allocation
    if _, err := io.ReadFull(stream, payload); err != nil {
        return nil, err
    }
    return payload, nil
}

func SerializeDatagramFrame(route string, payload []byte) ([]byte, error) {
    if len(route) > 255 {
        return nil, fmt.Errorf("route too long for datagram")
    }
    buf := new(bytes.Buffer)
    buf.WriteByte(byte(len(route)))
    buf.WriteString(route)
    buf.Write(payload)
    return buf.Bytes(), nil
}

func ParseDatagramFrame(data []byte) (*types.Request, error) {
    if len(data) < 1 {
        return nil, fmt.Errorf("datagram too short")
    }

    routeLen := int(data[0])
    data = data[1:]

    if len(data) < routeLen {
        return nil, fmt.Errorf("datagram truncated: expected %d route bytes, got %d", routeLen, len(data))
    }

    route := string(data[:routeLen])
    payload := data[routeLen:]

    return &types.Request{
        Route: route,
        Payload: payload,
    }, nil
}