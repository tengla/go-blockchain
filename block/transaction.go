package block

import (
	"crypto/ed25519"
	"crypto/sha256"
	"fmt"
)

// Reward is what the miner of a block is paid through its coinbase
const Reward = 50

// Transaction moves Amount from the owner of From to To. Nonce counts the
// sender's transactions, starting at 1, so a signed transaction can't be
// mined twice. A coinbase has no From, Nonce or Signature.
type Transaction struct {
	From      ed25519.PublicKey
	To        ed25519.PublicKey
	Amount    int
	Nonce     int
	Signature []byte
}

// NewTransaction creates a transaction signed with the sender's private key.
// The nonce must be one more than the sender's last one.
func NewTransaction(from ed25519.PrivateKey, to ed25519.PublicKey, amount, nonce int) *Transaction {
	t := &Transaction{
		From:   from.Public().(ed25519.PublicKey),
		To:     to,
		Amount: amount,
		Nonce:  nonce,
	}
	t.Signature = ed25519.Sign(from, t.signingPayload())
	return t
}

// NewCoinbase creates the transaction that pays the miner of a block
func NewCoinbase(to ed25519.PublicKey) *Transaction {
	return &Transaction{To: to, Amount: Reward}
}

// IsCoinbase reports whether the transaction creates coins instead of
// moving them
func (t *Transaction) IsCoinbase() bool {
	return len(t.From) == 0
}

// signingPayload is the part of the transaction the signature covers
func (t *Transaction) signingPayload() []byte {
	return []byte(fmt.Sprintf("%x|%x|%d|%d", t.From, t.To, t.Amount, t.Nonce))
}

// Verify checks that the transaction was signed by the owner of From, or
// for a coinbase that it pays exactly the reward. Whether the sender can
// afford it depends on the chain, which checks it when adding the block.
func (t *Transaction) Verify() bool {
	if len(t.To) != ed25519.PublicKeySize {
		return false
	}
	if t.IsCoinbase() {
		return t.Amount == Reward && t.Nonce == 0 && len(t.Signature) == 0
	}
	// ed25519.Verify panics on a key of the wrong length
	if len(t.From) != ed25519.PublicKeySize {
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
