package block

import (
	"crypto/ed25519"
	"testing"
)

func newKey(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func TestTransactionVerify(t *testing.T) {
	_, alice := newKey(t)
	bobPub, _ := newKey(t)
	if !NewTransaction(alice, bobPub, 10, 1).Verify() {
		t.Fatal("Signed transaction should verify")
	}
}

func TestTamperedTransactionFailsVerify(t *testing.T) {
	_, alice := newKey(t)
	bobPub, _ := newKey(t)
	tx := NewTransaction(alice, bobPub, 10, 1)
	tx.Amount = 1000
	if tx.Verify() {
		t.Fatal("Transaction with a changed amount should not verify")
	}
}

func TestTransactionSignedByWrongKeyFailsVerify(t *testing.T) {
	alicePub, _ := newKey(t)
	bobPub, _ := newKey(t)
	_, mallory := newKey(t)
	// mallory signs, but claims the money comes from alice
	tx := NewTransaction(mallory, bobPub, 10, 1)
	tx.From = alicePub
	if tx.Verify() {
		t.Fatal("Transaction not signed by the sender should not verify")
	}
}

func TestTransactionWithBadKeyFailsVerify(t *testing.T) {
	bobPub, _ := newKey(t)
	tx := &Transaction{From: []byte("short"), To: bobPub, Amount: 10}
	if tx.Verify() {
		t.Fatal("Transaction with a malformed key should not verify")
	}
}

func TestTransactionWithNonPositiveAmountFailsVerify(t *testing.T) {
	_, alice := newKey(t)
	bobPub, _ := newKey(t)
	for _, amount := range []int{0, -10} {
		if NewTransaction(alice, bobPub, amount, 1).Verify() {
			t.Fatalf("Transaction with amount %d should not verify", amount)
		}
	}
}

func TestCoinbaseVerify(t *testing.T) {
	alicePub, alice := newKey(t)
	if !NewCoinbase(alicePub, 1).Verify() {
		t.Fatal("Coinbase should verify")
	}

	nothing := NewCoinbase(alicePub, 1)
	nothing.Amount = 0
	signed := NewCoinbase(alicePub, 1)
	signed.Signature = ed25519.Sign(alice, signed.signingPayload())
	noOne := NewCoinbase(nil, 1)
	for name, tx := range map[string]*Transaction{
		"no amount":    nothing,
		"a signature":  signed,
		"no recipient": noOne,
	} {
		if tx.Verify() {
			t.Fatalf("Coinbase with %s should not verify", name)
		}
	}
}

func TestNonceIsSigned(t *testing.T) {
	_, alice := newKey(t)
	bobPub, _ := newKey(t)
	tx := NewTransaction(alice, bobPub, 10, 1)
	tx.Nonce = 2
	if tx.Verify() {
		t.Fatal("Transaction with a changed nonce should not verify")
	}
}
