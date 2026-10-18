// Package wallet stores key pairs in files. A wallet is just a key pair: the
// chain works out what each key holds from its transactions.
package wallet

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// Create writes a new key pair to name.key and name.pub. The keys are
// base64 encoded, as they appear in the JSON.
func Create(name string) error {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}
	if err := os.WriteFile(name+".key", []byte(Encode(priv)+"\n"), 0600); err != nil {
		return err
	}
	return os.WriteFile(name+".pub", []byte(Encode(pub)+"\n"), 0644)
}

// ReadPrivate reads a private key written by Create
func ReadPrivate(path string) (ed25519.PrivateKey, error) {
	key, err := read(path, ed25519.PrivateKeySize)
	return ed25519.PrivateKey(key), err
}

// ReadPublic reads a public key written by Create
func ReadPublic(path string) (ed25519.PublicKey, error) {
	key, err := read(path, ed25519.PublicKeySize)
	return ed25519.PublicKey(key), err
}

// Encode is how keys are written in files, JSON and URLs
func Encode(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

// DecodePublic is the reverse of Encode for a public key
func DecodePublic(s string) (ed25519.PublicKey, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, err
	}
	if len(key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("expected a %d byte key, got %d", ed25519.PublicKeySize, len(key))
	}
	return ed25519.PublicKey(key), nil
}

func read(path string, size int) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	if len(key) != size {
		return nil, fmt.Errorf("%s: expected a %d byte key, got %d", path, size, len(key))
	}
	return key, nil
}
