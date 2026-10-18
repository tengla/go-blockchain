package block

import (
	"crypto/ed25519"
	"fmt"
)

// ledger holds each key's balance and the nonce of its last transaction,
// as worked out from every transaction in the chain so far
type ledger struct {
	balances map[string]int
	nonces   map[string]int
}

func newLedger() *ledger {
	return &ledger{
		balances: map[string]int{},
		nonces:   map[string]int{},
	}
}

func (l *ledger) balance(key ed25519.PublicKey) int {
	return l.balances[string(key)]
}

func (l *ledger) nonce(key ed25519.PublicKey) int {
	return l.nonces[string(key)]
}

// apply returns a copy of the ledger with the transactions applied in order.
// The ledger itself is left alone, so a block that fails part way through
// changes nothing. Applying in order means two transactions in one block
// can't both spend the same coins.
func (l *ledger) apply(txs []*Transaction) (*ledger, error) {
	next := newLedger()
	for k, v := range l.balances {
		next.balances[k] = v
	}
	for k, v := range l.nonces {
		next.nonces[k] = v
	}
	for _, t := range txs {
		if !t.IsCoinbase() {
			from := string(t.From)
			if want := next.nonces[from] + 1; t.Nonce != want {
				return nil, fmt.Errorf("transaction from %x has nonce %d, expected %d", t.From, t.Nonce, want)
			}
			if have := next.balances[from]; have < t.Amount {
				return nil, fmt.Errorf("%x has %d, can't send %d", t.From, have, t.Amount)
			}
			next.nonces[from] = t.Nonce
			next.balances[from] -= t.Amount
		}
		next.balances[string(t.To)] += t.Amount
	}
	return next, nil
}
