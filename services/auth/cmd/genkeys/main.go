package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	outDir := flag.String("out", "./keys", "Output directory for generated keys")
	flag.Parse()

	if err := os.Mkdir(*outDir, 0o700); err != nil {
		fatal(err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fatal(err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		fatal(err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		fatal(err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	})
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	privPath := filepath.Join(*outDir, "ed25519_private.pem")
	pubPath := filepath.Join(*outDir, "ed25519_public.pem")

	if err := os.WriteFile(privPath, privPEM, 0o600); err != nil {
		fatal(err)
	}
	if err := os.WriteFile(pubPath, pubPEM, 0o644); err != nil {
		fatal(err)
	}

	sum := sha256.Sum256(pubBytes)
	kid := base64.RawURLEncoding.EncodeToString(sum[:16])
	fmt.Println("Keys generated successfully:")
	fmt.Println("private:", privPath)
	fmt.Println("public:", pubPath)
	fmt.Println("key ID:", kid)
	fmt.Println("The key ID needs to be saved to the service configuration (as `AUTH_KEY_ID`), along with the file paths.")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "genkeys: error", err)
	os.Exit(1)
}
