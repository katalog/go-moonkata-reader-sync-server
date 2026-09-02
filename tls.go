package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// loadOrCreateTLSCertificate는 이 서버의 HTTPS 인증서를 준비한다 — 사설 IP(192.168.x.x 등)엔 공인
// CA가 인증서를 발급해주지 않으므로 자체 서명(self-signed) 인증서를 쓴다. 안드로이드 쪽은 이 인증서를
// CA 체인으로 검증하는 대신, 최초 "연결 테스트" 성공 시점의 지문(fingerprint)을 저장해두고 이후
// 요청마다 그 지문과 정확히 같은지만 확인한다(SSH가 최초 접속 때 호스트 키를 저장해두는 것과 같은
// TOFU 방식, .docs/PC_SYNC_SERVER_PLAN.md 참고) — 그래서 인증서의 CN/SAN이 실제 접속 주소와 맞을
// 필요는 없고, 그냥 매번 같은 인증서를 계속 쓰기만 하면 된다. 한 번 만들면 %APPDATA%에 저장해두고
// 재사용 — 재실행할 때마다 바뀌면 안드로이드 쪽에 저장해둔 지문이 계속 안 맞아서 재인증이 필요해진다.
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
		NotAfter:              time.Now().AddDate(20, 0, 0), // 개인용 LAN 도구라 정기 갱신 없이 길게
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
