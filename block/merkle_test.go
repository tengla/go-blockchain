package block

import (
	"testing"
)

func newTransactions(t *testing.T, n int) []*Transaction {
	_, alice := newKey(t)
	bobPub, _ := newKey(t)
	txs := make([]*Transaction, n)
	for i := range txs {
		txs[i] = NewTransaction(alice, bobPub, i+1, i+1)
	}
	return txs
}

func TestMerkleRootIsDeterministic(t *testing.T) {
	txs := newTransactions(t, 5)
	if MerkleRoot(txs) != MerkleRoot(txs) {
		t.Fatal("Merkle root is not deterministic")
	}
}

func TestMerkleRootChangesWithAnyTransaction(t *testing.T) {
	for n := 1; n <= 7; n++ {
		txs := newTransactions(t, n)
		root := MerkleRoot(txs)
		for i := range txs {
			orig := txs[i]
			txs[i] = newTransactions(t, 1)[0]
			if MerkleRoot(txs) == root {
				t.Fatalf("Replacing transaction %d of %d did not change the root", i, n)
			}
			txs[i] = orig
		}
	}
}

func TestMerkleRootDependsOnOrder(t *testing.T) {
	txs := newTransactions(t, 2)
	swapped := []*Transaction{txs[1], txs[0]}
	if MerkleRoot(txs) == MerkleRoot(swapped) {
		t.Fatal("Reordering transactions did not change the root")
	}
}

func TestMerkleRootDoesNotDuplicateOddTransaction(t *testing.T) {
	txs := newTransactions(t, 3)
	padded := append(txs, txs[2])
	if MerkleRoot(txs) == MerkleRoot(padded) {
		t.Fatal("[a b c] and [a b c c] should not share a root")
	}
}
