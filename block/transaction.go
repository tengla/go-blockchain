package block

import (
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
)

// Transaction moves Amount from the owner of From to To
type Transaction struct {
	From      ed25519.PublicKey
	To        ed25519.PublicKey
	Amount    int
	Signature []byte
}

// NewTransaction creates a transaction signed with the sender's private key
func NewTransaction(from ed25519.PrivateKey, to ed25519.PublicKey, amount int) *Transaction {
	t := &Transaction{
		From:   from.Public().(ed25519.PublicKey),
		To:     to,
		Amount: amount,
	}
	t.Signature = ed25519.Sign(from, t.signingPayload())
	return t
}

// signingPayload is the part of the transaction the signature covers
func (t *Transaction) signingPayload() []byte {
	return []byte(fmt.Sprintf("%x|%x|%d", t.From, t.To, t.Amount))
}

// Verify checks that the transaction was signed by the owner of From
func (t *Transaction) Verify() bool {
	// ed25519.Verify panics on a key of the wrong length
	if len(t.From) != ed25519.PublicKeySize || len(t.To) != ed25519.PublicKeySize {
		return false
	}
	if t.Amount <= 0 {
		return false
	}
	return ed25519.Verify(t.From, t.signingPayload(), t.Signature)
}

// Hash identifies the transaction, signature included
func (t *Transaction) Hash() []byte {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%x", t.signingPayload(), t.Signature)))
	return h[:]
}
