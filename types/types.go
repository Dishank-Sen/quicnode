package types

type Request struct{
	Route string
	Payload []byte
}

type Response struct {
	Body       []byte
}

type ConnID string

type EventType int

const (
    EventConnOpened EventType = iota
    EventConnClosed
)

type Event struct {
    Type   EventType
    ConnID ConnID
    Err    error
}