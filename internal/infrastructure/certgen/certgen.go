package certgen

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

type Generator struct {
	caCert *x509.Certificate
	caKey  *ecdsa.PrivateKey
}

func NewGenerator(caCertPEM, caKeyPEM []byte) (*Generator, error) {
	const op = "certgen"
	caCert, err := parseCert(caCertPEM)
	if err != nil {
		return nil, fmt.Errorf("%s: parse ca cert: %w", op, err)
	}

	caKey, err := parseKey(caKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("%s: parse ca key: %w", op, err)
	}

	return &Generator{caCert: caCert, caKey: caKey}, nil
}

func (g *Generator) GenerateServerCert(ip net.IP) (certPEM, keyPEM []byte, err error) {
	const op = "generate server cert"
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: generate key: %w", op, err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("%s: generate serial: %w", op, err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: ip.String(),
		},
		IPAddresses: []net.IP{ip},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(10, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, g.caCert, &key.PublicKey, g.caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: create certificate: %w", op, err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: marshal key: %w", op, err)
	}

	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM, nil
}

func parseCert(pemBytes []byte) (*x509.Certificate, error) {
	const op = "parse certificate"
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return cert, nil
}

func parseKey(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	const op = "parse ec private key"
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return key, nil
}
