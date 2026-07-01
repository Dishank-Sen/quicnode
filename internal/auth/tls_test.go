package auth

import (
	"crypto/x509"
	"testing"

	"github.com/flynn/noise"
)

func TestDeriveTLSCertificate(t *testing.T) {
	// Generate a test keypair
	keypair, err := noise.DH25519.GenerateKeypair(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Derive TLS certificate
	tlsCert, err := DeriveTLSCertificate(keypair)
	if err != nil {
		t.Fatalf("Failed to derive TLS certificate: %v", err)
	}

	// Parse the certificate
	cert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Derive expected PeerID
	session, err := NewSession(keypair.Public)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Verify CommonName matches PeerID
	if cert.Subject.CommonName != session.PeerID {
		t.Errorf("Certificate CommonName mismatch: got %s, want %s", cert.Subject.CommonName, session.PeerID)
	}

	// Verify key usage
	if cert.KeyUsage != x509.KeyUsageDigitalSignature {
		t.Errorf("Unexpected KeyUsage: %v", cert.KeyUsage)
	}

	// Verify extended key usage
	if len(cert.ExtKeyUsage) != 2 {
		t.Errorf("Expected 2 ExtKeyUsage entries, got %d", len(cert.ExtKeyUsage))
	}

	hasServerAuth := false
	hasClientAuth := false
	for _, usage := range cert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageServerAuth {
			hasServerAuth = true
		}
		if usage == x509.ExtKeyUsageClientAuth {
			hasClientAuth = true
		}
	}

	if !hasServerAuth {
		t.Error("Certificate missing ExtKeyUsageServerAuth")
	}
	if !hasClientAuth {
		t.Error("Certificate missing ExtKeyUsageClientAuth")
	}

	t.Logf("Certificate CommonName: %s", cert.Subject.CommonName)
	t.Logf("Certificate valid from %v to %v", cert.NotBefore, cert.NotAfter)
}

func TestGetTLSConfig(t *testing.T) {
	keypair, err := noise.DH25519.GenerateKeypair(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	tlsConfig, err := GetTLSConfig(keypair)
	if err != nil {
		t.Fatalf("Failed to get TLS config: %v", err)
	}

	if len(tlsConfig.Certificates) != 1 {
		t.Errorf("Expected 1 certificate, got %d", len(tlsConfig.Certificates))
	}

	if !tlsConfig.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be true")
	}

	if len(tlsConfig.NextProtos) != 1 || tlsConfig.NextProtos[0] != "quicnode" {
		t.Errorf("Expected NextProtos=[quicnode], got %v", tlsConfig.NextProtos)
	}
}

func TestDeterministicCertificate(t *testing.T) {
	// Generate keypair
	keypair, err := noise.DH25519.GenerateKeypair(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Generate certificate twice
	cert1, err := DeriveTLSCertificate(keypair)
	if err != nil {
		t.Fatalf("Failed to derive certificate 1: %v", err)
	}

	cert2, err := DeriveTLSCertificate(keypair)
	if err != nil {
		t.Fatalf("Failed to derive certificate 2: %v", err)
	}

	// Parse both
	parsed1, _ := x509.ParseCertificate(cert1.Certificate[0])
	parsed2, _ := x509.ParseCertificate(cert2.Certificate[0])

	// CommonName should be identical (deterministic)
	if parsed1.Subject.CommonName != parsed2.Subject.CommonName {
		t.Error("Certificate CommonName is not deterministic")
	}

	// Note: Full certificate bytes will differ due to timestamps and random serial number
	// But the identity (CommonName = PeerID) should be stable
}
