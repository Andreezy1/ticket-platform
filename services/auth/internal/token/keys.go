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

func LoadPrivateKey(path string) (ed25519.PrivateKey, ed25519.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, nil, fmt.Errorf("failed to decode private key file")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	edKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("private key is not of type ed25519")
	}

	pub, ok := edKey.Public().(ed25519.PublicKey)
	if !ok {
		return nil, nil, fmt.Errorf("ed25519: unexpected public key type %T", edKey.Public())
	}

	return edKey, pub, nil
}

func computeKID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}
