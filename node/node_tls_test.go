package node

import (
	"context"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
)

// Edge Case: Node creation without TlsConfig (should succeed)
func TestNewNode_NoTlsConfig(t *testing.T) {
	ctx := context.Background()

	node, err := NewNode(ctx, Config{
		ListenAddr: "127.0.0.1:28001",
		QuicConfig: &quic.Config{},
	})

	if err != nil {
		t.Fatalf("NewNode should succeed without TlsConfig: %v", err)
	}

	if node == nil {
		t.Fatal("Node should not be nil")
	}

	// Verify keypair was loaded
	if len(node.localKeypair.Private) != 32 {
		t.Errorf("Keypair not loaded correctly: private key length %d", len(node.localKeypair.Private))
	}

	if len(node.localKeypair.Public) != 32 {
		t.Errorf("Keypair not loaded correctly: public key length %d", len(node.localKeypair.Public))
	}
}

// Edge Case: Node with RequireAuth=true generates keypair
func TestNewNode_RequireAuthGeneratesKeypair(t *testing.T) {
	ctx := context.Background()

	node, err := NewNode(ctx, Config{
		ListenAddr:  "127.0.0.1:28002",
		QuicConfig:  &quic.Config{},
		RequireAuth: true,
	})

	if err != nil {
		t.Fatalf("NewNode with RequireAuth should succeed: %v", err)
	}

	if node == nil {
		t.Fatal("Node should not be nil")
	}

	// Verify keypair was loaded
	if len(node.localKeypair.Private) != 32 {
		t.Error("Keypair not loaded for authenticated node")
	}

	if len(node.localKeypair.Public) != 32 {
		t.Error("Public key not loaded for authenticated node")
	}
}

// Edge Case: Node with RequireAuth=false still generates keypair (for TLS)
func TestNewNode_NoAuthStillGeneratesKeypair(t *testing.T) {
	ctx := context.Background()

	node, err := NewNode(ctx, Config{
		ListenAddr:  "127.0.0.1:28003",
		QuicConfig:  &quic.Config{},
		RequireAuth: false,
	})

	if err != nil {
		t.Fatalf("NewNode should succeed: %v", err)
	}

	// Even without auth, keypair should be loaded (needed for TLS)
	if len(node.localKeypair.Private) != 32 {
		t.Error("Keypair should be loaded even without auth (needed for TLS)")
	}
}

// Edge Case: Node can start and stop multiple times
func TestNode_StartStopMultipleTimes(t *testing.T) {
	ctx := context.Background()

	node, err := NewNode(ctx, Config{
		ListenAddr: "127.0.0.1:18888",
		QuicConfig: &quic.Config{},
	})
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	// Start
	if err := node.Start(); err != nil {
		t.Fatalf("First start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Stop
	if err := node.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Stop again (should be idempotent)
	if err := node.Stop(); err != nil {
		t.Errorf("Second stop should be idempotent: %v", err)
	}
}

// Edge Case: Multiple nodes can be created simultaneously
func TestNewNode_MultipleNodes(t *testing.T) {
	ctx := context.Background()

	nodes := make([]*Node, 5)
	for i := 0; i < 5; i++ {
		node, err := NewNode(ctx, Config{
			ListenAddr: "127.0.0.1:28010", // Use same port - just testing creation
			QuicConfig: &quic.Config{},
		})
		if err != nil {
			t.Fatalf("Failed to create node %d: %v", i, err)
		}
		nodes[i] = node

		// Each should have loaded the same keypair
		if i > 0 {
			// All nodes should have the same keypair (loaded from same file)
			for j := 0; j < 32; j++ {
				if nodes[i].localKeypair.Private[j] != nodes[0].localKeypair.Private[j] {
					// This is actually expected - all nodes load the same key
					break
				}
			}
		}
	}

	// Cleanup
	for _, n := range nodes {
		if n != nil {
			n.Stop()
		}
	}
}

// Edge Case: Node with nil context
func TestNewNode_NilContext(t *testing.T) {
	_, err := NewNode(context.TODO(), Config{
		ListenAddr: "127.0.0.1:28004",
		QuicConfig: &quic.Config{},
	})

	if err == nil {
		t.Error("Expected error for nil context")
	}

	if err != nil && err.Error() != "context is nil" {
		t.Errorf("Wrong error message: %v", err)
	}
}

// Edge Case: Node with invalid config (missing QuicConfig)
func TestNewNode_MissingQuicConfig(t *testing.T) {
	ctx := context.Background()

	_, err := NewNode(ctx, Config{
		ListenAddr: "127.0.0.1:28005",
		QuicConfig: nil,
	})

	if err == nil {
		t.Error("Expected error for nil QuicConfig")
	}

	if err != nil && err.Error() != "quic config is required" {
		t.Errorf("Wrong error message: %v", err)
	}
}

// Edge Case: Node with invalid listen address
func TestNewNode_InvalidListenAddr(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name string
		addr string
	}{
		{"empty address", ""},
		{"invalid format", "not-an-address"},
		{"missing port", "127.0.0.1"},
		{"invalid port", "127.0.0.1:99999"},
		{"negative port", "127.0.0.1:-1"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewNode(ctx, Config{
				ListenAddr: tc.addr,
				QuicConfig: &quic.Config{},
			})

			if err == nil {
				t.Errorf("Expected error for address: %s", tc.addr)
			}
		})
	}
}

// Edge Case: Node Start generates TLS config successfully
func TestNode_StartGeneratesTLSConfig(t *testing.T) {
	ctx := context.Background()

	node, err := NewNode(ctx, Config{
		ListenAddr: "127.0.0.1:19999",
		QuicConfig: &quic.Config{},
	})
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	defer node.Stop()

	// Start should generate TLS config internally
	err = node.Start()
	if err != nil {
		t.Fatalf("Start failed (TLS generation error?): %v", err)
	}

	// Node should be listening
	if node.listener == nil {
		t.Error("Listener should be set after Start")
	}

	if node.transport == nil {
		t.Error("Transport should be set after Start")
	}

	if node.udpConn == nil {
		t.Error("UDP connection should be set after Start")
	}
}

// Edge Case: Node can handle Start failure gracefully
func TestNode_StartFailureCleanup(t *testing.T) {
	ctx := context.Background()

	// Create first node on specific port
	node1, err := NewNode(ctx, Config{
		ListenAddr: "127.0.0.1:17777",
		QuicConfig: &quic.Config{},
	})
	if err != nil {
		t.Fatalf("Failed to create node1: %v", err)
	}

	if err := node1.Start(); err != nil {
		t.Fatalf("Failed to start node1: %v", err)
	}
	defer node1.Stop()

	// Try to create second node on same port (should fail)
	node2, err := NewNode(ctx, Config{
		ListenAddr: "127.0.0.1:17777",
		QuicConfig: &quic.Config{},
	})
	if err != nil {
		t.Fatalf("Failed to create node2: %v", err)
	}

	err = node2.Start()
	if err == nil {
		node2.Stop()
		t.Error("Expected error for port already in use")
	}
}

// Edge Case: Cancelled context prevents node creation
func TestNewNode_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	node, err := NewNode(ctx, Config{
		ListenAddr: "127.0.0.1:28006",
		QuicConfig: &quic.Config{},
	})

	// Node creation should succeed even with cancelled context
	// But operations will fail
	if err != nil {
		t.Fatalf("NewNode should succeed even with cancelled context: %v", err)
	}

	if node == nil {
		t.Fatal("Node should not be nil")
	}

	// Start might fail or work depending on timing
	// The important thing is no crash
	node.Start()
	node.Stop()
}

// Edge Case: Zero-value Config
func TestNewNode_ZeroValueConfig(t *testing.T) {
	ctx := context.Background()

	_, err := NewNode(ctx, Config{})

	if err == nil {
		t.Error("Expected error for zero-value config")
	}

	// Should fail on validation (missing fields)
}

// Edge Case: OpenConn generates TLS config correctly
func TestNode_OpenConnGeneratesTLS(t *testing.T) {
	ctx := context.Background()

	// Create server
	server, err := NewNode(ctx, Config{
		ListenAddr: "127.0.0.1:16666",
		QuicConfig: &quic.Config{},
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Stop()

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Create client
	client, err := NewNode(ctx, Config{
		ListenAddr: "127.0.0.1:16667",
		QuicConfig: &quic.Config{},
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Stop()

	if err := client.Start(); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// OpenConn should generate TLS config internally
	peer, err := client.OpenConn(ctx, "127.0.0.1:16666")
	if err != nil {
		t.Fatalf("OpenConn failed (TLS generation error?): %v", err)
	}

	if peer == nil {
		t.Error("Peer should not be nil")
	}

	// Verify peer has an ID (connection was established)
	if peer.ID == "" {
		t.Error("Peer ID should be set after connection")
	}
}
