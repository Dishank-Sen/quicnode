# quicnode

A Go library for building peer-to-peer systems over QUIC. Provides connection pooling, stream multiplexing, and unreliable datagram support with a route-based handler API. Handles the transport layer so you can focus on application logic.

## What This Library Does NOT Do

- **Identity**: No public keys, peer IDs, or cryptographic identity
- **Discovery**: No DHT, mDNS, or peer discovery mechanisms  
- **Decentralization**: No blockchain, consensus, or distributed hash tables

This is a transport library. Bring your own identity layer and discovery mechanism.

## Wire Format

### Stream Request/Response

```
Stream Request:
┌──────────────────┬─────────────────┬──────────────────┬──────────────┐
│ Route Length (2) │ Route (variable)│ Payload Len (4)  │ Payload      │
└──────────────────┴─────────────────┴──────────────────┴──────────────┘

Stream Response:
┌──────────────────┬──────────────┐
│ Payload Len (4)  │ Payload      │
└──────────────────┴──────────────┘
```

### Datagram

```
┌──────────────────┬─────────────────┬──────────────┐
│ Route Length (1) │ Route (variable)│ Payload      │
└──────────────────┴─────────────────┴──────────────┘
```

All integers are big-endian. Route is UTF-8. Payload is opaque bytes.

## Installation

```bash
go get github.com/Dishank-Sen/quicnode
```

Requires Go 1.25+

## Quick Start

```go
package main

import (
    "context"
    "crypto/tls"
    "fmt"
    "log"
    "time"

    "github.com/Dishank-Sen/quicnode/node"
    "github.com/quic-go/quic-go"
)

func main() {
    ctx := context.Background()
    tlsCfg := &tls.Config{ /* your TLS config */ }

    // Node A - listens and handles requests
    nodeA, _ := node.NewNode(ctx, node.Config{
        ListenAddr: "127.0.0.1:4242",
        TlsConfig:  tlsCfg,
        QuicConfig: &quic.Config{},
    })
    nodeA.Start()

    nodeA.HandleStream("echo", func(c node.StreamContext) {
        log.Printf("Received: %s", string(c.Payload()))
        c.Write([]byte("echo: " + string(c.Payload())))
    })

    // Node B - connects and sends request
    nodeB, _ := node.NewNode(ctx, node.Config{
        ListenAddr: "127.0.0.1:4243",
        TlsConfig:  tlsCfg,
        QuicConfig: &quic.Config{},
    })
    nodeB.Start()

    peer, _ := nodeB.OpenConn(ctx, "127.0.0.1:4242")
    
    respCh, _ := peer.Send("echo", []byte("hello"))
    response := <-respCh
    fmt.Printf("Got: %s\n", string(response))
}
```

## API Reference

### Node

```go
func NewNode(ctx context.Context, cfg Config) (*Node, error)
```
Creates a new node. Context controls lifetime. Returns error if config is invalid.

```go
func (n *Node) Start() error
```
Starts listening for connections. Non-blocking. Returns error if listener fails.

```go
func (n *Node) Stop() error
```
Gracefully shuts down the node and closes all connections.

```go
func (n *Node) Wait() error
```
Blocks until node context is cancelled or an error occurs.

```go
func (n *Node) HandleStream(route string, h StreamHandlerFunc)
```
Registers a handler for reliable stream-based requests on the given route.

```go
func (n *Node) HandleDatagram(route string, h DatagramHandlerFunc)
```
Registers a handler for unreliable datagram-based requests on the given route.

```go
func (n *Node) OpenConn(ctx context.Context, addr string) (*Peer, error)
```
Opens a connection to the remote address. Reuses existing connection if available.

```go
func (n *Node) Events() <-chan types.Event
```
Returns channel of connection lifecycle events (opened, closed).

### Peer

```go
func (p *Peer) Send(route string, payload []byte) (<-chan []byte, error)
```
Sends a stream-based request. Returns channel that receives response chunks. Reliable, ordered.

```go
func (p *Peer) SendDatagram(route string, payload []byte) error
```
Sends an unreliable datagram. Fire-and-forget. No response channel. May be lost or reordered.

```go
func (p *Peer) Close()
```
Closes the connection to this peer.

### StreamContext (Handler Argument)

```go
Route() string
```
Returns the route string from the request.

```go
Payload() []byte
```
Returns the request payload bytes.

```go
PeerAddr() string
```
Returns the remote peer's address.

```go
Write([]byte) (int, error)
```
Writes response data back to the requester. Can be called multiple times.

### DatagramContext (Handler Argument)

```go
Route() string
```
Returns the route string from the datagram.

```go
Payload() []byte
```
Returns the datagram payload bytes.

```go
PeerAddr() string
```
Returns the remote peer's address.

Note: No Write method. Datagrams are one-way.

### Config

```go
type Config struct {
    ListenAddr string         // IP:port to bind to
    TlsConfig  *tls.Config    // TLS configuration (required)
    QuicConfig *quic.Config   // QUIC transport config (required)
}
```

For datagrams, set `quicConfig.EnableDatagrams = true`.

## Streams vs Datagrams

### Streams: Reliable Request-Response

Use streams when:
- Data must arrive (financial transactions, commands)
- You need a response
- Order matters
- Payload is large (>1KB)

**Example:**
```go
// Server
node.HandleStream("transfer", func(ctx node.StreamContext) {
    amount := parseAmount(ctx.Payload())
    result := processTransfer(amount)
    ctx.Write(result)
})

// Client
peer, _ := node.OpenConn(ctx, addr)
respCh, _ := peer.Send("transfer", []byte("100"))
result := <-respCh
```

**Characteristics:**
- Guaranteed delivery
- In-order arrival
- Retransmission on loss
- ~5-50ms latency
- Request-response pattern

### Datagrams: Unreliable Fire-and-Forget

Use datagrams when:
- Loss is acceptable (telemetry, game updates)
- Low latency is critical (<1ms)
- Data is frequent and redundant
- Payload is small (<1KB)

**Example:**
```go
// Server
node.HandleDatagram("position", func(ctx node.DatagramContext) {
    x, y, z := parsePosition(ctx.Payload())
    updatePlayerPosition(ctx.PeerAddr(), x, y, z)
    // No response - one-way only
})

// Client (game loop at 60 FPS)
peer, _ := node.OpenConn(ctx, addr)
for range time.Tick(16 * time.Millisecond) {
    peer.SendDatagram("position", serializePosition(player))
}
```

**Characteristics:**
- May be lost (95-99% delivery typical)
- May arrive out of order
- No retransmission
- <1ms latency
- One-way only (no response)

## Architecture

```
┌─────────────────────────────────────────┐
│              Application                │
│  (Your handlers & business logic)       │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│              node.Node                  │
│  • Listen for connections               │
│  • Route registration                   │
│  • Connection pooling                   │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│         connection.Peer                 │
│  • Per-peer connection handle           │
│  • Stream multiplexing                  │
│  • Datagram sending                     │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│          stream.Stream                  │
│  • Parse incoming requests              │
│  • Handler dispatch                     │
│  • Response serialization               │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│          parser.Parser                  │
│  • Binary framing (route + payload)     │
│  • Length-prefix encoding               │
│  • Big-endian integers                  │
└─────────────────────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│           quic-go/quic-go               │
│  • QUIC protocol implementation         │
│  • UDP transport                        │
└─────────────────────────────────────────┘
```

Incoming requests flow bottom-up. Outgoing requests flow top-down. Connection manager pools connections by remote address and reuses existing connections when available.
