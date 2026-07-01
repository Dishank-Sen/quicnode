package auth

import (
	"os"
	"path/filepath"
)

func getKeyDir() string {
    if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
        return filepath.Join(xdg, "quicnode")
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".local", "share", "quicnode")
}

func getKnownPeersDir() string {
    return filepath.Join(getKeyDir(), "known_peers")
}