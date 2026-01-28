package node

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
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
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded){
		return true
	}

	// when issue related to handshake
	var unknownAuth x509.UnknownAuthorityError
	if errors.As(err, &unknownAuth) {
		return true
	}

	var certInvalid x509.CertificateInvalidError
	if errors.As(err, &certInvalid) {
		return true
	}

	var hostErr x509.HostnameError
	if errors.As(err, &hostErr) {
		return true
	}

	var appErr *quic.ApplicationError
	if errors.As(err, &appErr){
		return true
	}
	return false
}
