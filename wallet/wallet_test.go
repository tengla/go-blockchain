package wallet

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateAndRead(t *testing.T) {
	name := filepath.Join(t.TempDir(), "alice")
	if err := Create(name); err != nil {
		t.Fatal(err)
	}
	priv, err := ReadPrivate(name + ".key")
	if err != nil {
		t.Fatal(err)
	}
	pub, err := ReadPublic(name + ".pub")
	if err != nil {
		t.Fatal(err)
	}
	if !pub.Equal(priv.Public()) {
		t.Fatal("The public key should belong to the private key")
	}
	info, err := os.Stat(name + ".key")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("The private key should only be readable by its owner, has mode %v", info.Mode().Perm())
	}
}

func TestReadRejectsWrongKind(t *testing.T) {
	name := filepath.Join(t.TempDir(), "alice")
	if err := Create(name); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPrivate(name + ".pub"); err == nil {
		t.Fatal("A public key should not read as a private key")
	}
}

func TestDecodePublic(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(nil)
	got, err := DecodePublic(Encode(pub))
	if err != nil || !got.Equal(pub) {
		t.Fatalf("Key should survive Encode and DecodePublic, got %v", err)
	}
	if _, err := DecodePublic("bm9wZQ=="); err == nil {
		t.Fatal("A short key should be rejected")
	}
}
