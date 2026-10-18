package block

import (
	"crypto/ed25519"
	"testing"
)

// fund mines a block whose reward goes to key
func fund(t *testing.T, c *Chain, key ed25519.PublicKey) {
	// Len counts the genesis block, so it is the index of the next block
	if err := c.AddBlock(NewBlock(nil, NewCoinbase(key, c.Len()))); err != nil {
		t.Fatal(err)
	}
}

func TestBalancesFollowTransactions(t *testing.T) {
	alicePub, alice := newKey(t)
	bobPub, bob := newKey(t)
	c := CreateChain(1)
	fund(t, c, alicePub)
	if err := c.AddBlock(NewBlock(nil, NewTransaction(alice, bobPub, 10, 1))); err != nil {
		t.Fatal(err)
	}
	if err := c.AddBlock(NewBlock(nil,
		NewTransaction(bob, alicePub, 3, 1), NewTransaction(alice, bobPub, 5, 2))); err != nil {
		t.Fatal(err)
	}
	if c.Balance(alicePub) != InitialReward-10+3-5 || c.Balance(bobPub) != 10-3+5 {
		t.Fatalf("Balances should be %d and %d, got %d and %d",
			InitialReward-10+3-5, 10-3+5, c.Balance(alicePub), c.Balance(bobPub))
	}
	if c.Nonce(alicePub) != 2 || c.Nonce(bobPub) != 1 {
		t.Fatalf("Nonces should be 2 and 1, got %d and %d", c.Nonce(alicePub), c.Nonce(bobPub))
	}
}

func TestRejectsSpendingMoreThanBalance(t *testing.T) {
	alicePub, alice := newKey(t)
	bobPub, bob := newKey(t)
	c := CreateChain(1)
	fund(t, c, alicePub)
	if c.AddBlock(NewBlock(nil, NewTransaction(bob, alicePub, 1, 1))) == nil {
		t.Fatal("A key with no coins should not be able to send any")
	}
	if c.AddBlock(NewBlock(nil, NewTransaction(alice, bobPub, InitialReward+1, 1))) == nil {
		t.Fatal("Alice should not be able to send more than she has")
	}
	if c.Len() != 2 {
		t.Fatal("Rejected blocks should not be added to the chain")
	}
}

func TestRejectsReplayedTransaction(t *testing.T) {
	alicePub, alice := newKey(t)
	bobPub, _ := newKey(t)
	c := CreateChain(1)
	fund(t, c, alicePub)
	tx := NewTransaction(alice, bobPub, 10, 1)
	if err := c.AddBlock(NewBlock(nil, tx)); err != nil {
		t.Fatal(err)
	}
	if c.AddBlock(NewBlock(nil, tx)) == nil {
		t.Fatal("The same signed transaction should not be mined twice")
	}
	if c.Balance(bobPub) != 10 {
		t.Fatalf("Bob should have 10, has %d", c.Balance(bobPub))
	}
}

func TestRejectsSkippedNonce(t *testing.T) {
	alicePub, alice := newKey(t)
	bobPub, _ := newKey(t)
	c := CreateChain(1)
	fund(t, c, alicePub)
	if c.AddBlock(NewBlock(nil, NewTransaction(alice, bobPub, 10, 2))) == nil {
		t.Fatal("Alice's first transaction should need nonce 1")
	}
}

func TestRejectsDoubleSpendInOneBlock(t *testing.T) {
	alicePub, alice := newKey(t)
	bobPub, _ := newKey(t)
	carolPub, _ := newKey(t)
	c := CreateChain(1)
	fund(t, c, alicePub)
	// each is affordable on its own, but not both
	err := c.AddBlock(NewBlock(nil,
		NewTransaction(alice, bobPub, 30, 1), NewTransaction(alice, carolPub, 30, 2)))
	if err == nil {
		t.Fatal("Alice should not be able to spend the same coins twice in one block")
	}
	if c.Balance(alicePub) != InitialReward || c.Balance(bobPub) != 0 || c.Nonce(alicePub) != 0 {
		t.Fatal("A rejected block should not change any balance or nonce")
	}
}

func TestCanSpendRewardInTheSameBlock(t *testing.T) {
	alicePub, alice := newKey(t)
	bobPub, _ := newKey(t)
	c := CreateChain(1)
	if err := c.AddBlock(NewBlock(nil, NewCoinbase(alicePub, 1), NewTransaction(alice, bobPub, 10, 1))); err != nil {
		t.Fatal(err)
	}
	if c.Balance(bobPub) != 10 {
		t.Fatalf("Bob should have 10, has %d", c.Balance(bobPub))
	}
}

func TestRejectsCoinbaseThatIsNotFirst(t *testing.T) {
	alicePub, _ := newKey(t)
	c := CreateChain(1)
	if c.AddBlock(NewBlock(nil, NewCoinbase(alicePub, 1), NewCoinbase(alicePub, 1))) == nil {
		t.Fatal("A block should pay only one reward")
	}
}

// minedBlock mines a block onto c's tip without going through c, the way a
// peer that doesn't follow the rules would
func minedBlock(c *Chain, txs ...*Transaction) *Block {
	b := NewBlock(nil, txs...)
	b.PreviousHash = c.Tip().Hash
	b.Idx = c.Tip().Idx + 1
	b.MineBlock(c.Difficulty)
	return b
}

func TestAppendBlockRejectsOverspend(t *testing.T) {
	_, alice := newKey(t)
	bobPub, _ := newKey(t)
	c := CreateChain(1)
	err := c.AppendBlock(minedBlock(c, NewTransaction(alice, bobPub, 10, 1)))
	if err == nil || err == ErrNotOnTip {
		t.Fatalf("A peer's block spending coins alice doesn't have should be rejected, got %v", err)
	}
}

func TestLoadChainRebuildsBalances(t *testing.T) {
	alicePub, alice := newKey(t)
	bobPub, _ := newKey(t)
	a := CreateChain(1)
	fund(t, a, alicePub)
	if err := a.AddBlock(NewBlock(nil, NewTransaction(alice, bobPub, 10, 1))); err != nil {
		t.Fatal(err)
	}
	c, err := LoadChain(1, copyBlocks(t, a.Blocks()))
	if err != nil {
		t.Fatal(err)
	}
	if c.Balance(alicePub) != InitialReward-10 || c.Balance(bobPub) != 10 || c.Nonce(alicePub) != 1 {
		t.Fatal("Loaded chain should have the same balances and nonces as the original")
	}
}

func TestRewardHalves(t *testing.T) {
	for idx, want := range map[int]int{
		0:                      0,
		1:                      InitialReward,
		HalvingInterval:        InitialReward,
		HalvingInterval + 1:    InitialReward / 2,
		2*HalvingInterval + 1:  InitialReward / 4,
		6*HalvingInterval + 1:  0,
		1000 * HalvingInterval: 0,
	} {
		if got := Reward(idx); got != want {
			t.Fatalf("Reward for block %d should be %d, got %d", idx, want, got)
		}
	}
	total := 0
	for idx := 1; Reward(idx) > 0; idx++ {
		total += Reward(idx)
	}
	if total != 9700 {
		t.Fatalf("All rewards together should be 9700, got %d", total)
	}
}

func TestChainPaysHalvedReward(t *testing.T) {
	alicePub, _ := newKey(t)
	c := CreateChain(1)
	for i := 0; i < HalvingInterval; i++ {
		fund(t, c, alicePub)
	}
	idx := c.Len()
	if c.AddBlock(NewBlock(nil, NewCoinbase(alicePub, 1))) == nil {
		t.Fatalf("Block %d should not pay the first reward of %d", idx, InitialReward)
	}
	fund(t, c, alicePub)
	if want := HalvingInterval*InitialReward + InitialReward/2; c.Balance(alicePub) != want {
		t.Fatalf("Alice should have %d, has %d", want, c.Balance(alicePub))
	}
}

func TestRejectsWrongReward(t *testing.T) {
	alicePub, _ := newKey(t)
	c := CreateChain(1)
	coinbase := NewCoinbase(alicePub, 1)
	coinbase.Amount++
	if c.AddBlock(NewBlock(nil, coinbase)) == nil {
		t.Fatal("A coinbase paying more than the reward should be rejected")
	}
	err := c.AppendBlock(minedBlock(c, coinbase))
	if err == nil || err == ErrNotOnTip {
		t.Fatalf("A peer's block paying more than the reward should be rejected, got %v", err)
	}
}
