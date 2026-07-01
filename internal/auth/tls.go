package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"

	"github.com/flynn/noise"
)

// DeriveTLSCertificate generates a self-signed TLS certificate from a Noise Curve25519 keypair.
// The certificate is deterministic for a given keypair and regenerated on every startup.
// It is used for QUIC transport encryption, while Noise XX provides the actual authentication.
func DeriveTLSCertificate(noiseKeypair noise.DHKey) (tls.Certificate, error) {
	// Derive Ed25519 key from the first 32 bytes of Noise private key
	// Curve25519 and Ed25519 both use 32-byte seeds, and this provides a deterministic mapping
	if len(noiseKeypair.Private) != 32 {
		return tls.Certificate{}, fmt.Errorf("invalid noise private key length: %d", len(noiseKeypair.Private))
	}

	// Create Ed25519 private key from the Noise private key bytes
	edPrivKey := ed25519.NewKeyFromSeed(noiseKeypair.Private)
	edPubKey := edPrivKey.Public().(ed25519.PublicKey)

	// Generate PeerID for use as CommonName
	session, err := NewSession(noiseKeypair.Public)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to derive peer ID: %w", err)
	}

	// Create certificate template
	now := time.Now()
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: session.PeerID, // Use the node's PeerID as identity
		},
		NotBefore:             now,
		NotAfter:              now.Add(10 * 365 * 24 * time.Hour), // 10 years
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Self-sign the certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, edPubKey, edPrivKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Construct tls.Certificate
	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  edPrivKey,
	}, nil
}

// GetTLSConfig creates a TLS config using a certificate derived from the Noise keypair.
// InsecureSkipVerify is true because certs are self-signed - actual authentication is via Noise XX.
func GetTLSConfig(noiseKeypair noise.DHKey) (*tls.Config, error) {
	cert, err := DeriveTLSCertificate(noiseKeypair)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, // Self-signed certs, authentication is via Noise XX
		NextProtos:         []string{"quicnode"},
	}, nil
}
