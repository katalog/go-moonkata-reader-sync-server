package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// loadOrCreateTLSCertificate prepares this server's HTTPS certificate — since a public CA won't
// issue a certificate for a private IP (192.168.x.x, etc.), we use a self-signed certificate.
// Instead of validating this certificate via a CA chain, the Android side stores the fingerprint
// from the moment its first "connection test" succeeds, and on every request afterward just
// checks it matches that fingerprint exactly (the same TOFU approach SSH uses when it stores a
// host key on first connect — see .docs/PC_SYNC_SERVER_PLAN.md). So the certificate's CN/SAN
// doesn't need to match the actual connection address — it just needs to stay the same certificate
// every time. Once created, it's saved under %APPDATA% and reused — if it changed on every
// restart, the fingerprint Android has stored would keep mismatching, requiring re-pairing.
func loadOrCreateTLSCertificate() (tls.Certificate, error) {
	dir, err := configDir()
	if err != nil {
		return tls.Certificate{}, err
	}
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		return cert, nil
	}

	cert, err := generateSelfSignedCertificate(certPath, keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	return cert, nil
}

func generateSelfSignedCertificate(certPath string, keyPath string) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, err
	}

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: "moonkata-sync-server"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(20, 0, 0), // a personal LAN tool, so a long validity with no periodic renewal
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	certOut, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		certOut.Close()
		return tls.Certificate{}, err
	}
	certOut.Close()

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}); err != nil {
		keyOut.Close()
		return tls.Certificate{}, err
	}
	keyOut.Close()

	return tls.LoadX509KeyPair(certPath, keyPath)
}

// certificateFingerprint computes the SHA-256 fingerprint of the leaf certificate in exactly the
// same format as Android's PcTlsTrust.sha256Fingerprint (colon-separated uppercase hex pairs, 32
// of them) — QR pairing (.docs/SYNC_MULTIUSER_PLAN.md stage 6) sends this value as-is in the QR,
// and Android compares the string directly, so the pinned TLS connection fails if the format is
// off by even one character.
func certificateFingerprint(cert tls.Certificate) (string, error) {
	if len(cert.Certificate) == 0 {
		return "", errors.New("certificate is empty")
	}
	sum := sha256.Sum256(cert.Certificate[0])
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":"), nil
}
