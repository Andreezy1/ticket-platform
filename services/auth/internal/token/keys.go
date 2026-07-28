package token

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("failed to decode private key file")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	edKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not of type ed25519")
	}

	return edKey, nil
}

func PublicKeyFromPrivate(priv ed25519.PrivateKey) ed25519.PublicKey {
	return priv.Public().(ed25519.PublicKey)
}

func computeKID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}
