package auth

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mr-tron/base58"
)

// Session represents an authenticated peer session after Noise handshake.
type Session struct {
	PublicKey []byte // Raw 32-byte Curve25519 public key
	PeerID    string // base58(sha256(PublicKey))
}

// NewSession creates a session from an authenticated public key.
// Derives the PeerID as base58(sha256(publicKey)).
func NewSession(publicKey []byte) (*Session, error) {
	if len(publicKey) != 32 {
		return nil, fmt.Errorf("invalid public key length: %d (expected 32)", len(publicKey))
	}

	peerID := derivePeerID(publicKey)

	return &Session{
		PublicKey: publicKey,
		PeerID:    peerID,
	}, nil
}

// derivePeerID computes base58(sha256(publicKey)).
func derivePeerID(publicKey []byte) string {
	hash := sha256.Sum256(publicKey)
	return base58.Encode(hash[:])
}

// VerifyOrStorePeer implements TOFU (Trust On First Use) logic.
// On first connection: stores the peer's public key.
// On subsequent connections: verifies the key matches what was stored.
// Returns error if verification fails (MITM detected).
func VerifyOrStorePeer(peerID string, publicKey []byte) error {
	knownPeersDir := getKnownPeersDir()
	peerFile := filepath.Join(knownPeersDir, peerID+".pub")

	// Check if we've seen this peer before
	storedKey, err := os.ReadFile(peerFile)
	if err == nil {
		// Peer known - verify key matches
		if !bytesEqual(storedKey, publicKey) {
			return fmt.Errorf("peer key mismatch: stored key differs from presented key (possible MITM)")
		}
		// Key matches - success
		return nil
	}

	// First contact - store the key
	if err := os.MkdirAll(knownPeersDir, 0700); err != nil {
		return fmt.Errorf("failed to create known_peers directory: %w", err)
	}

	if err := os.WriteFile(peerFile, publicKey, 0644); err != nil {
		return fmt.Errorf("failed to store peer public key: %w", err)
	}

	return nil
}

// GetKnownPeers returns a list of all known peer IDs.
func GetKnownPeers() ([]string, error) {
	knownPeersDir := getKnownPeersDir()

	entries, err := os.ReadDir(knownPeersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var peerIDs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Remove .pub extension
		if len(name) > 4 && name[len(name)-4:] == ".pub" {
			peerID := name[:len(name)-4]
			peerIDs = append(peerIDs, peerID)
		}
	}

	return peerIDs, nil
}

// RemoveKnownPeer removes a peer from the known_peers store.
// Useful for manual trust revocation.
func RemoveKnownPeer(peerID string) error {
	knownPeersDir := getKnownPeersDir()
	peerFile := filepath.Join(knownPeersDir, peerID+".pub")

	err := os.Remove(peerFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove peer %s: %w", peerID, err)
	}
	return nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
