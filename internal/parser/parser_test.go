package parser

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestParseRequest_ValidFrame(t *testing.T) {
    route := "ping"
    payload := "hello"

    // construct the frame manually
    buf := new(bytes.Buffer)
    binary.Write(buf, binary.BigEndian, uint16(len(route)))
    buf.WriteString(route)
    binary.Write(buf, binary.BigEndian, uint32(len(payload)))
    buf.WriteString(payload)

    req, err := ParseRequest(buf)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if req.Route != route {
        t.Errorf("expected route %q, got %q", route, req.Route)
    }
    if string(req.Payload) != payload {
        t.Errorf("expected payload %q, got %q", payload, string(req.Payload))
    }
}

func TestParseRequest_EmptyPayload(t *testing.T) {
    route := "ping"
    payload := ""

    // construct the frame manually
    buf := new(bytes.Buffer)
    binary.Write(buf, binary.BigEndian, uint16(len(route)))
    buf.WriteString(route)
    binary.Write(buf, binary.BigEndian, uint32(len(payload)))
    buf.WriteString(payload)

    req, err := ParseRequest(buf)

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if req.Route != route {
        t.Errorf("expected route %q, got %q", route, req.Route)
    }
    if string(req.Payload) != payload {
        t.Errorf("expected payload %q, got %q", payload, string(req.Payload))
    }
}

func TestParseRequest_TruncatedFrame(t *testing.T) {
    buf := new(bytes.Buffer)
    // write route length of 4 but don't write the actual route bytes
    binary.Write(buf, binary.BigEndian, uint16(4))

    _, err := ParseRequest(buf)
    if err == nil {
        t.Fatal("expected error for truncated frame, got nil")
    }
}

func TestSerializeRequest_ValidRequest(t *testing.T) {
    route := "ping"
    payload := "hii it ping"

    data, err := SerializeRequest(route, []byte(payload))
    if err != nil {
        t.Fatalf("unexpected error in serializing: %v", err)
    }

    reader := bytes.NewReader(data)
    req, err := ParseRequest(reader)
    if err != nil{
        t.Fatalf("unexpected error in parsing: %v", err)
    }
    if req.Route != route {
        t.Errorf("expected route %q, got %q", route, req.Route)
    }
    if string(req.Payload) != payload {
        t.Errorf("expected payload %q, got %q", payload, string(req.Payload))
    }
}

func TestSerializeRequest_EmptyRequest(t *testing.T) {
    route := ""
    payload := ""

    data, err := SerializeRequest(route, []byte(payload))
    if err != nil {
        t.Fatalf("unexpected error in serializing: %v", err)
    }

    reader := bytes.NewReader(data)
    req, err := ParseRequest(reader)
    if err != nil{
        t.Fatalf("unexpected error in parsing: %v", err)
    }
    if req.Route != route {
        t.Errorf("expected route %q, got %q", route, req.Route)
    }
    if string(req.Payload) != payload {
        t.Errorf("expected payload %q, got %q", payload, string(req.Payload))
    }
}

func TestParseResponse_ValidFrame(t *testing.T) {
    // response should be like this - [4byte payload length][byte payload]

    payload := "sample payload"
    buf := new(bytes.Buffer)
    binary.Write(buf, binary.BigEndian, uint32(len(payload)))
    buf.WriteString(payload)

    data, err := ParseResponse(buf)
    if err != nil {
        t.Fatalf("unexpected error in parsing response: %v", err)
    }
    if string(data) != payload {
        t.Errorf("expected payload %q, got %q", payload, string(data))
    }
}

func TestParseResponse_EmptyFrame(t *testing.T) {
    // response should be like this - [4byte payload length][byte payload]

    payload := ""
    buf := new(bytes.Buffer)
    binary.Write(buf, binary.BigEndian, uint32(len(payload)))
    buf.WriteString(payload)

    data, err := ParseResponse(buf)
    if err != nil {
        t.Fatalf("unexpected error in parsing response: %v", err)
    }
    if string(data) != payload {
        t.Errorf("expected payload %q, got %q", payload, string(data))
    }
}

func TestParseResponse_TruncatedFrame(t *testing.T) {
    // response should be like this - [4byte payload length][byte payload]

    buf := new(bytes.Buffer)
    binary.Write(buf, binary.BigEndian, uint32(4))

    _, err := ParseResponse(buf)
    if err == nil {
        t.Fatal("expected error for truncated frame, got nil")
    }
}