package auth

import (
	"crypto/ed25519"
	"crypto/x509"
	"testing"
	"time"

	"github.com/flynn/noise"
)

// Edge Case: Invalid keypair length
func TestDeriveTLSCertificate_InvalidPrivateKeyLength(t *testing.T) {
	invalidKeypair := noise.DHKey{
		Private: make([]byte, 16), // Wrong length (should be 32)
		Public:  make([]byte, 32),
	}

	_, err := DeriveTLSCertificate(invalidKeypair)
	if err == nil {
		t.Error("Expected error for invalid private key length, got nil")
	}

	if err != nil && err.Error() != "invalid noise private key length: 16" {
		t.Errorf("Wrong error message: %v", err)
	}
}

// Edge Case: Empty keypair
func TestDeriveTLSCertificate_EmptyKeypair(t *testing.T) {
	emptyKeypair := noise.DHKey{
		Private: []byte{},
		Public:  []byte{},
	}

	_, err := DeriveTLSCertificate(emptyKeypair)
	if err == nil {
		t.Error("Expected error for empty keypair, got nil")
	}
}

// Edge Case: Nil keypair bytes
func TestDeriveTLSCertificate_NilKeypair(t *testing.T) {
	nilKeypair := noise.DHKey{
		Private: nil,
		Public:  nil,
	}

	_, err := DeriveTLSCertificate(nilKeypair)
	if err == nil {
		t.Error("Expected error for nil keypair, got nil")
	}
}

// Edge Case: Zero-filled keypair (valid length but all zeros)
func TestDeriveTLSCertificate_ZeroKeypair(t *testing.T) {
	zeroKeypair := noise.DHKey{
		Private: make([]byte, 32), // All zeros
		Public:  make([]byte, 32), // All zeros
	}

	cert, err := DeriveTLSCertificate(zeroKeypair)
	if err != nil {
		t.Errorf("Should accept zero keypair: %v", err)
	}

	// Certificate should still be generated (though not secure in practice)
	if len(cert.Certificate) == 0 {
		t.Error("Expected certificate to be generated")
	}

	// Verify it's a valid Ed25519 key
	if cert.PrivateKey == nil {
		t.Error("Expected private key to be set")
	}

	edKey, ok := cert.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		t.Error("Expected Ed25519 private key")
	}

	if len(edKey) != ed25519.PrivateKeySize {
		t.Errorf("Wrong Ed25519 key size: got %d, want %d", len(edKey), ed25519.PrivateKeySize)
	}
}

// Edge Case: Certificate expiration boundaries
func TestDeriveTLSCertificate_ExpirationBoundaries(t *testing.T) {
	keypair, err := noise.DH25519.GenerateKeypair(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	now := time.Now()
	cert, err := DeriveTLSCertificate(keypair)
	if err != nil {
		t.Fatalf("Failed to derive certificate: %v", err)
	}

	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Check NotBefore is close to now (within 5 seconds)
	timeDiff := parsed.NotBefore.Sub(now)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	if timeDiff > 5*time.Second {
		t.Errorf("NotBefore too far from now: diff=%v", timeDiff)
	}

	// Check NotAfter is exactly 10 years from NotBefore
	expectedExpiry := parsed.NotBefore.Add(10 * 365 * 24 * time.Hour)
	if !parsed.NotAfter.Equal(expectedExpiry) {
		t.Errorf("NotAfter mismatch: got %v, want %v", parsed.NotAfter, expectedExpiry)
	}

	// Verify certificate is currently valid
	now = time.Now()
	if now.Before(parsed.NotBefore) {
		t.Error("Certificate not yet valid")
	}
	if now.After(parsed.NotAfter) {
		t.Error("Certificate already expired")
	}
}

// Edge Case: Multiple certificates from same keypair (determinism check)
func TestDeriveTLSCertificate_MultipleCalls(t *testing.T) {
	keypair, err := noise.DH25519.GenerateKeypair(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Generate 5 certificates from the same keypair
	certs := make([][]byte, 5)
	peerIDs := make([]string, 5)

	for i := 0; i < 5; i++ {
		cert, err := DeriveTLSCertificate(keypair)
		if err != nil {
			t.Fatalf("Call %d failed: %v", i, err)
		}

		parsed, _ := x509.ParseCertificate(cert.Certificate[0])
		certs[i] = cert.Certificate[0]
		peerIDs[i] = parsed.Subject.CommonName
	}

	// All PeerIDs should be identical (deterministic)
	for i := 1; i < 5; i++ {
		if peerIDs[i] != peerIDs[0] {
			t.Errorf("PeerID %d differs: %s != %s", i, peerIDs[i], peerIDs[0])
		}
	}

	// Certificate bytes will differ due to timestamps, but identity is stable
	t.Logf("All 5 certificates have same PeerID: %s", peerIDs[0])
}

// Edge Case: Ed25519 key derivation correctness
func TestDeriveTLSCertificate_Ed25519KeyDerivation(t *testing.T) {
	keypair, err := noise.DH25519.GenerateKeypair(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	cert, err := DeriveTLSCertificate(keypair)
	if err != nil {
		t.Fatalf("Failed to derive certificate: %v", err)
	}

	// Extract the Ed25519 private key
	edPriv, ok := cert.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		t.Fatal("Private key is not Ed25519")
	}

	// Derive Ed25519 key manually and compare
	expectedEdPriv := ed25519.NewKeyFromSeed(keypair.Private)

	if len(edPriv) != len(expectedEdPriv) {
		t.Errorf("Key length mismatch: got %d, want %d", len(edPriv), len(expectedEdPriv))
	}

	// Keys should be identical
	for i := range edPriv {
		if edPriv[i] != expectedEdPriv[i] {
			t.Errorf("Key byte %d differs: got %d, want %d", i, edPriv[i], expectedEdPriv[i])
			break
		}
	}

	// Verify public key matches
	edPub := edPriv.Public().(ed25519.PublicKey)
	expectedEdPub := expectedEdPriv.Public().(ed25519.PublicKey)

	if len(edPub) != len(expectedEdPub) {
		t.Error("Public key length mismatch")
	}

	for i := range edPub {
		if edPub[i] != expectedEdPub[i] {
			t.Error("Public key mismatch")
			break
		}
	}
}

// Edge Case: Certificate can be parsed by standard library
func TestDeriveTLSCertificate_StandardLibraryCompatibility(t *testing.T) {
	keypair, err := noise.DH25519.GenerateKeypair(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	cert, err := DeriveTLSCertificate(keypair)
	if err != nil {
		t.Fatalf("Failed to derive certificate: %v", err)
	}

	// Parse with x509
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("Certificate cannot be parsed by x509: %v", err)
	}

	// Verify signature algorithm
	if parsed.SignatureAlgorithm != x509.PureEd25519 {
		t.Errorf("Wrong signature algorithm: got %v, want PureEd25519", parsed.SignatureAlgorithm)
	}

	// Verify public key algorithm
	if parsed.PublicKeyAlgorithm != x509.Ed25519 {
		t.Errorf("Wrong public key algorithm: got %v, want Ed25519", parsed.PublicKeyAlgorithm)
	}

	// Verify it's a valid Ed25519 public key
	_, ok := parsed.PublicKey.(ed25519.PublicKey)
	if !ok {
		t.Error("Public key is not Ed25519")
	}
}

// Edge Case: GetTLSConfig with invalid keypair
func TestGetTLSConfig_InvalidKeypair(t *testing.T) {
	invalidKeypair := noise.DHKey{
		Private: make([]byte, 10), // Wrong length
		Public:  make([]byte, 32),
	}

	_, err := GetTLSConfig(invalidKeypair)
	if err == nil {
		t.Error("Expected error for invalid keypair, got nil")
	}
}

// Edge Case: GetTLSConfig returns proper config structure
func TestGetTLSConfig_ConfigStructure(t *testing.T) {
	keypair, err := noise.DH25519.GenerateKeypair(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	config, err := GetTLSConfig(keypair)
	if err != nil {
		t.Fatalf("Failed to get TLS config: %v", err)
	}

	// Verify all required fields
	if len(config.Certificates) != 1 {
		t.Errorf("Wrong number of certificates: got %d, want 1", len(config.Certificates))
	}

	if !config.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}

	if len(config.NextProtos) != 1 {
		t.Errorf("Wrong number of NextProtos: got %d, want 1", len(config.NextProtos))
	}

	if config.NextProtos[0] != "quicnode" {
		t.Errorf("Wrong NextProtos: got %v, want [quicnode]", config.NextProtos)
	}

	// Verify certificate is usable
	cert := config.Certificates[0]
	if len(cert.Certificate) == 0 {
		t.Error("Certificate chain is empty")
	}

	if cert.PrivateKey == nil {
		t.Error("Private key is nil")
	}
}

// Edge Case: Serial number consistency
func TestDeriveTLSCertificate_SerialNumber(t *testing.T) {
	keypair, err := noise.DH25519.GenerateKeypair(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	cert, err := DeriveTLSCertificate(keypair)
	if err != nil {
		t.Fatalf("Failed to derive certificate: %v", err)
	}

	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Serial number should always be 1
	if parsed.SerialNumber.Int64() != 1 {
		t.Errorf("Wrong serial number: got %d, want 1", parsed.SerialNumber.Int64())
	}
}

// Edge Case: Certificate has no additional extensions
func TestDeriveTLSCertificate_MinimalExtensions(t *testing.T) {
	keypair, err := noise.DH25519.GenerateKeypair(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	cert, err := DeriveTLSCertificate(keypair)
	if err != nil {
		t.Fatalf("Failed to derive certificate: %v", err)
	}

	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Should not be a CA
	if parsed.IsCA {
		t.Error("Certificate should not be a CA")
	}

	// Should not have subject alternative names
	if len(parsed.DNSNames) > 0 {
		t.Error("Certificate should not have DNS names")
	}

	if len(parsed.IPAddresses) > 0 {
		t.Error("Certificate should not have IP addresses")
	}

	// Should have basic constraints valid
	if !parsed.BasicConstraintsValid {
		t.Error("BasicConstraintsValid should be true")
	}
}

// Edge Case: PeerID from invalid public key
func TestNewSession_InvalidPublicKeyLength(t *testing.T) {
	invalidKey := make([]byte, 16) // Wrong length

	_, err := NewSession(invalidKey)
	if err == nil {
		t.Error("Expected error for invalid public key length")
	}

	if err != nil && err.Error() != "invalid public key length: 16 (expected 32)" {
		t.Errorf("Wrong error message: %v", err)
	}
}

// Edge Case: PeerID from empty public key
func TestNewSession_EmptyPublicKey(t *testing.T) {
	emptyKey := []byte{}

	_, err := NewSession(emptyKey)
	if err == nil {
		t.Error("Expected error for empty public key")
	}
}

// Edge Case: PeerID from nil public key
func TestNewSession_NilPublicKey(t *testing.T) {
	_, err := NewSession(nil)
	if err == nil {
		t.Error("Expected error for nil public key")
	}
}

// Edge Case: Concurrent certificate generation
func TestDeriveTLSCertificate_Concurrent(t *testing.T) {
	keypair, err := noise.DH25519.GenerateKeypair(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	// Generate certificates concurrently
	results := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			_, err := DeriveTLSCertificate(keypair)
			results <- err
		}()
	}

	// Check all succeeded
	for i := 0; i < 10; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent call %d failed: %v", i, err)
		}
	}
}

// Edge Case: Concurrent GetTLSConfig calls
func TestGetTLSConfig_Concurrent(t *testing.T) {
	keypair, err := noise.DH25519.GenerateKeypair(nil)
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	results := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			_, err := GetTLSConfig(keypair)
			results <- err
		}()
	}

	for i := 0; i < 10; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent call %d failed: %v", i, err)
		}
	}
}
