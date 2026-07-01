# Noise XX Authentication

QuicNode supports optional peer authentication using the Noise XX protocol with Trust On First Use (TOFU).

## Overview

When `Config.RequireAuth` is enabled:
- All connections perform a Noise XX handshake before accepting application streams
- Peers are authenticated by their Curve25519 static public key
- Public keys are verified using TOFU (Trust On First Use)
- Authenticated peer identity is available to handler functions

## Enabling Authentication

```go
node, err := node.NewNode(ctx, node.Config{
    ListenAddr:  "127.0.0.1:4242",
    QuicConfig:  &quic.Config{},
    RequireAuth: true, // Enable Noise XX authentication
})
```

**Note:** TLS certificates are automatically derived from the node's Noise keypair. No TLS configuration needed from the user.

**Default:** `RequireAuth: false` (authentication disabled, backward compatible)

## Keypair Management

### Automatic Key Generation

On first run with `RequireAuth: true`, a Curve25519 keypair is automatically generated and saved:

```
~/.local/share/quicnode/
├── node.key (private, 0600 permissions)
└── node.pub (public, 0644 permissions)
```

On subsequent runs, the keypair is loaded from disk. **Never regenerate if keys exist.**

### Key Security

- Private key has `0600` permissions (owner read/write only)
- Keys are stored in `$XDG_DATA_HOME/quicnode` or `~/.local/share/quicnode`
- If you lose your private key, you must generate a new identity (delete both files)

## Trust On First Use (TOFU)

QuicNode uses TOFU for peer verification:

### First Connection

1. Peer connects and completes Noise XX handshake
2. Remote public key is extracted
3. Peer ID computed as `base58(sha256(publicKey))`
4. Public key is stored in `~/.local/share/quicnode/known_peers/<peerID>.pub`

### Subsequent Connections

1. Peer connects and completes handshake
2. Presented public key is compared against stored key
3. If keys match: connection accepted
4. If keys differ: **connection rejected** (possible MITM or key rotation)

### Known Peers Directory

```
~/.local/share/quicnode/known_peers/
├── 8kN9H7Xv2QmP5zY...s4a.pub
├── 3bT6K2Pq8JvM1nX...r9b.pub
└── ...
```

Each file contains the 32-byte Curve25519 public key of an authenticated peer.

## Handshake Protocol

### Noise XX Pattern

QuicNode uses the **XX** pattern:
- **No prior key knowledge required** - peers can be completely unknown
- **Mutual authentication** - both sides prove their identity
- **Full forward secrecy** - ephemeral keys in every handshake

### Handshake Flow

```
Client                              Server
  |                                    |
  |--- QUIC Connection --------------->|
  |                                    |
  |--- Stream "_noise" --------------->|
  |--- Noise msg 1: -> e ------------->|
  |<-- Noise msg 2: <- e, ee, s, es ---|
  |--- Noise msg 3: -> s, se --------->|
  |                                    |
  [✓ Authenticated]              [✓ Authenticated]
  |                                    |
  |--- Application streams allowed --->|
```

**Messages:**
- `e` = ephemeral public key
- `s` = static public key (identity)
- `ee`, `es`, `se` = Diffie-Hellman operations

### Handshake Stream

- Route: `_noise` (reserved internal route)
- Initiated by client immediately after QUIC connection
- Not visible to user handlers
- **Application streams are blocked** until handshake completes

## Accessing Peer Identity

### In Stream Handlers

```go
node.HandleStream("greet", func(ctx node.StreamContext) {
    // Access authenticated identity
    peerID := ctx.PeerID()           // e.g., "8kN9H7Xv2QmP5zYs4a"
    publicKey := ctx.PeerPublicKey() // 32 bytes Curve25519 key

    if peerID == "" {
        // RequireAuth is false or handshake not complete
    }

    log.Printf("Request from peer %s", peerID)
    ctx.Write([]byte("hello"))
})
```

### On Peer Object

```go
peer, err := node.OpenConn(ctx, "127.0.0.1:4242")

publicKey, peerID, authenticated := peer.GetAuthInfo()
if authenticated {
    log.Printf("Connected to peer %s", peerID)
    log.Printf("Public key: %x", publicKey)
}
```

## Peer ID Format

```
PeerID = base58(sha256(PublicKey))
```

**Properties:**
- **Stable:** Same public key always produces same Peer ID
- **Human-readable:** Base58 encoding (no confusing characters)
- **Collision-resistant:** SHA-256 provides 256-bit security
- **Length:** Approximately 43-44 characters

**Example:** `8kN9H7Xv2QmP5zYr3tK6Ws4aBc1Fn9Lm7Pq5Gh2Jx3Yz`

## Trust Management

### List Known Peers

```go
import "github.com/Dishank-Sen/quicnode/internal/auth"

peerIDs, err := auth.GetKnownPeers()
for _, peerID := range peerIDs {
    fmt.Printf("Known peer: %s\n", peerID)
}
```

### Remove a Peer (Revoke Trust)

```go
err := auth.RemoveKnownPeer(peerID)
```

After removal, the peer must go through TOFU again on next connection.

### Manual Key Verification

Before trusting a peer on first connection, verify their public key out-of-band:

```bash
# On peer's machine
cat ~/.local/share/quicnode/node.pub | base64

# Compare with what you received
cat ~/.local/share/quicnode/known_peers/<peerID>.pub | base64
```

## Security Considerations

### TOFU Limitations

- **First connection is unverified** - vulnerable to MITM on initial connection
- **Key rotation is not supported** - changing keys requires manual trust re-establishment
- **No revocation mechanism** - you must manually delete keys from `known_peers/`

### Mitigations

1. **Out-of-band verification:** Exchange and verify public keys before first connection
2. **Private networks:** Use TOFU on trusted networks where MITM is unlikely
3. **Multi-factor trust:** Combine TOFU with other trust signals (IP allowlists, etc.)

### Attack Scenarios

**MITM on first connection:**
- Attacker intercepts first connection
- Attacker's key is stored in `known_peers/`
- Solution: Verify public key out-of-band before trusting

**Key rotation:**
- Peer generates new keypair
- TOFU rejects connection (key mismatch)
- Solution: Manually remove old key from `known_peers/`, re-establish trust

## Backward Compatibility

### RequireAuth: false (Default)

- **No handshake** - application streams accepted immediately
- **No authentication** - `ctx.PeerID()` returns `""`
- **No TOFU** - `known_peers/` directory not used
- **No performance impact** - authentication code path skipped entirely

### RequireAuth: true

- **Breaking change** - unauthenticated peers cannot connect
- **Handshake required** - adds ~50-100ms latency to connection establishment
- **Storage overhead** - one 32-byte file per known peer

## Performance

### Handshake Overhead

- **Latency:** ~50-100ms (3 round-trips over QUIC)
- **CPU:** Minimal (Curve25519 operations are fast)
- **Memory:** ~1KB per handshake session
- **One-time cost:** Handshake only on new connections

### After Handshake

- **Zero overhead** - QUIC already provides transport encryption
- **No per-message cost** - authentication is connection-level

## Troubleshooting

### "Handshake failed"

**Cause:** Network error, incompatible Noise implementation, or version mismatch  
**Solution:** Check logs for specific error, verify both nodes have compatible quicnode versions

### "TOFU verification failed"

**Cause:** Peer's public key changed since last connection  
**Solution:**
```bash
# Remove old key
rm ~/.local/share/quicnode/known_peers/<peerID>.pub

# Reconnect (will re-establish trust)
```

### "Failed to load keypair"

**Cause:** Corrupted or missing key files  
**Solution:**
```bash
# Backup and regenerate
mv ~/.local/share/quicnode ~/.local/share/quicnode.backup
# Restart node (new keys generated)
```

### Handler receives empty PeerID

**Cause:** `RequireAuth: false` or handshake not complete  
**Check:**
```go
if ctx.PeerID() == "" {
    log.Println("Peer not authenticated")
}
```

## Complete Example

See `/mnt/data/quicnode-testing/auth_test/main.go` for a working example demonstrating:
- Server with `RequireAuth: true`
- Client connecting and performing handshake
- Handler accessing authenticated peer identity
- TOFU key storage and verification
- Subsequent connection reusing stored key

Run:
```bash
cd /mnt/data/quicnode-testing/auth_test
go run main.go
```

## Implementation Details

### Reserved Routes

The following routes are reserved for internal use and will panic if registered by users:

- `_noise` - Noise XX handshake stream
- `_ping` - Internal keepalive
- `_peers` - Peer list exchange (reserved for future use)

Any user route starting with `_` will panic on registration.

### Noise Library

QuicNode uses [flynn/noise](https://github.com/flynn/noise) for Noise protocol implementation.

### Dependencies

- `github.com/flynn/noise` - Noise protocol
- `github.com/mr-tron/base58` - Base58 encoding for Peer IDs
