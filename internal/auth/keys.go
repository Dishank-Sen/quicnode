package auth

import (
	"crypto/rand"
	"os"
	"path/filepath"

	"github.com/flynn/noise"
)

// LoadOrGenerateKeypair loads the node's Noise keypair from disk or generates a new one.
// Keys are stored at ~/.local/share/quicnode/node.key (private) and node.pub (public).
// Private key has 0600 permissions. This is called once per node initialization.
func LoadOrGenerateKeypair() (noise.DHKey, error) {
    keyDir := getKeyDir()
    privPath := filepath.Join(keyDir, "node.key")
    pubPath  := filepath.Join(keyDir, "node.pub")

    // check if already exists
    priv, err := os.ReadFile(privPath)
    if err == nil {
        pub, err := os.ReadFile(pubPath)
        if err == nil {
            return noise.DHKey{
                Private: priv,
                Public:  pub,
            }, nil
        }
    }

    // generate fresh keypair
    keypair, err := noise.DH25519.GenerateKeypair(rand.Reader)
    if err != nil {
        return noise.DHKey{}, err
    }

    // save to disk
    if err := os.MkdirAll(keyDir, 0700); err != nil {
        return noise.DHKey{}, err
    }
    os.WriteFile(privPath, keypair.Private, 0600) // private — owner read only
    os.WriteFile(pubPath, keypair.Public, 0644)   // public — readable by anyone

    return keypair, nil
}