// Package warp owns the Cloudflare WARP registration client and key utilities.
package warp

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// Keypair holds an X25519 private/public key pair in base64-encoded form.
type Keypair struct {
	PrivateKey string // base64 standard encoding
	PublicKey  string // base64 standard encoding
}

// GenerateKeypair generates a fresh X25519 keypair for WARP registration.
func GenerateKeypair() (Keypair, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return Keypair{}, fmt.Errorf("generate X25519 key: %w", err)
	}
	return Keypair{
		PrivateKey: base64.StdEncoding.EncodeToString(priv.Bytes()),
		PublicKey:  base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes()),
	}, nil
}
