package node

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (n *Node) handleConnError(err error) (shouldExit bool) {
	// 1. Context cancelled → normal shutdown
	if errors.Is(err, context.Canceled) {
		fmt.Println("accept loop exiting: context cancelled")
		return true
	}

	// 2. Listener closed → normal shutdown
	msg := err.Error()
	if strings.Contains(msg, "closed") ||
		strings.Contains(msg, "use of closed network connection") {
		fmt.Println("accept loop exiting: listener closed")
		return true
	}

	// 3. Temporary / retryable error
	type temporary interface {
		Temporary() bool
	}
	if t, ok := err.(temporary); ok && t.Temporary() {
		fmt.Println("temporary accept error:", err)
		time.Sleep(time.Second) // prevent busy loop
		return false
	}

	// 4. Unknown / fatal error → stop node
	fmt.Println("fatal accept error:", err)

	// Stop asynchronously to avoid deadlock
	go n.Stop()

	return true
}

func (n *Node) handleStreamError(err error) (shouldExit bool) {
	if err == nil {
		return false
	}

	// 1. Context cancelled → normal shutdown
	if errors.Is(err, context.Canceled) {
		return true
	}

	// 2. Connection closed by peer or locally → session ends
	msg := err.Error()
	if strings.Contains(msg, "closed") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "connection closed") {
		return true
	}

	// 3. Temporary / retryable stream error
	type temporary interface {
		Temporary() bool
	}
	if t, ok := err.(temporary); ok && t.Temporary() {
		time.Sleep(50 * time.Millisecond) // avoid busy loop
		return false
	}
	
	return true
}
