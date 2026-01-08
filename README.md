# quicnode

`quicnode` is a **minimal QUIC-based networking library** that provides **both server and client capabilities** with **route-based message handling**.  
It is designed to be embedded in **CLI tools, daemons, bootstrap services, or any Go application** that needs long-lived, bidirectional QUIC communication.

This library intentionally **does not handle identity, discovery, or decentralization logic**.  
It focuses purely on **transport + routing**, so it can be reused across multiple applications.

---

## Features

- QUIC server (listener) and client (dialer)
- Long-lived QUIC connections
- Route-based request handling (similar to Express routing)
- Context-aware lifecycle management
- Clean start/stop semantics
- TLS-required, QUIC-native design
- Minimal, embeddable API

---

## Installation

```bash
go get github.com/Dishank-Sen/quicnode@latest
```

## How To Use

1. Create TLS and QUIC Config
```bash
cert, _ := tls.LoadX509KeyPair(certFilePath, keyFilePath)

tlsConfig := &tls.Config{
    InsecureSkipVerify: true,   // for development only
    Certificates: []tls.Certificate{cert},
    NextProtos:   []string{"quicnode"},
}

quicConfig := &quic.Config{}
```

TLS is mandatory. QUIC will not work without it.

2. Create a Node
```bash
ctx := context.Background()

cfg := node.Config{
    ListenAddr: ":4001",   // port where node will listen
    TlsConfig:  tlsConfig,
    QuicConfig: quicConfig,
}

n, err := node.NewNode(ctx, cfg)
if err != nil {
    log.Fatal(err)
}
```

3. Register Route Handlers
```bash
n.Handle("ping", func(req *types.Request) *types.Response {
    return &types.Response{
        StatusCode: 200,
        Message:    "OK",
        Body:       []byte("pong"),
    }
})
```

Each handler:
- receives a *types.Request
- returns a *types.Response

4. Start the Node
```bash
if err := n.Start(); err != nil {
    log.Fatal(err)
}
```

Start() is non-blocking and starts the QUIC listener internally.

5. Send a Request (Client Side)
```bash
resp, err := n.Dial(
    "127.0.0.1:4001",
    "ping",
    map[string]string{
        "x-client": "example",
    },
    []byte("hello"),
)
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(resp.Body)) // pong
```

6. Stop the Node
```bash
if err := n.Stop(); err != nil {
    log.Println(err)
}
```

Stops the listener and cancels the internal context.