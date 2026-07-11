/*
Package lan implements a hop transport that syncs directly between two machines
on the same network over mutually-authenticated TLS with pinned self-signed
certificates.
*/
package lan

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

/* Identity is this machine's LAN TLS certificate and its fingerprint. */
type Identity struct {
	Cert        tls.Certificate
	Fingerprint string
}

/* fingerprintOf returns the lowercase hex SHA-256 of a DER-encoded certificate. */
func fingerprintOf(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

/*
LoadOrCreateIdentity loads the machine's LAN certificate and key from dir,
generating and persisting a new self-signed pair on first use.
*/
func LoadOrCreateIdentity(dir string) (Identity, error) {
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if _, err := os.Stat(certPath); err == nil {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return Identity{}, err
		}
		return Identity{Cert: cert, Fingerprint: fingerprintOf(cert.Certificate[0])}, nil
	}
	return createIdentity(dir, certPath, keyPath)
}

/* createIdentity generates a self-signed ed25519 certificate and writes it to disk. */
func createIdentity(dir, certPath, keyPath string) (Identity, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Identity{}, err
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Identity{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "hop-lan"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	if err != nil {
		return Identity{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return Identity{}, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return Identity{}, err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return Identity{}, err
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return Identity{}, fmt.Errorf("loading freshly written identity: %w", err)
	}
	return Identity{Cert: cert, Fingerprint: fingerprintOf(cert.Certificate[0])}, nil
}
