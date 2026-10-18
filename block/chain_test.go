package block

import (
	"encoding/json"
	"log"
	"testing"
)

func TestCreateChain(t *testing.T) {
	c := CreateChain(1)
	if c == nil {
		log.Fatal("Chain is nil")
	}
	if c.Genesis == nil {
		log.Fatal("Chain Genesis is nil")
	}
}

func TestAddBlock(t *testing.T) {
	c := CreateChain(1)
	c.AddBlock(NewBlock(map[string]string{
		"Name": "Block 1",
	}))
	c.AddBlock(NewBlock(map[string]string{
		"Name": "Block 2",
	}))
	if c.numberOfBlocks != 2 {
		t.Fatal("Number of blocks should be 2")
	}
	i := 0
	c.EveryBlock(func(b *Block) {
		i += 1
	})
	if i != 3 {
		t.Fatalf("Should iterate 2 blocks with EveryBlock, was %d", i)
	}
	if c.Validate() != nil {
		t.Fatal("Chain not valid")
	}
}
func TestShouldNotBeAbleToTamperWithBlock(t *testing.T) {
	c := CreateChain(1)
	c.AddBlock(NewBlock(map[string]string{
		"Name": "Block 1",
	}))
	c.AddBlock(NewBlock(map[string]string{
		"Name": "Block 2",
	}))
	c.AddBlock(NewBlock(map[string]string{
		"Name": "Block 3",
	}))
	c.Genesis.Next.Next.Data["Name"] = "I am now trying to screw you over"
	if c.Validate() == nil {
		t.Fatal("Chain should not be valid, because somebody tampered with it.")
	}
}

func TestShouldNotBeAbleToTamperAndRehash(t *testing.T) {
	c := CreateChain(3)
	c.AddBlock(NewBlock(map[string]string{
		"Name": "Block 1",
	}))
	c.AddBlock(NewBlock(map[string]string{
		"Name": "Block 2",
	}))
	// tamper, then fix up the hashes without mining
	b := c.Genesis.Next
	b.Data["Name"] = "I am now trying to screw you over"
	for cur := b; cur != nil; cur = cur.Next {
		cur.Hash = cur.GenerateHash()
		if cur.Next != nil {
			cur.Next.PreviousHash = cur.Hash
		}
	}
	if c.Validate() == nil {
		t.Fatal("Chain should not be valid, because the rehashed blocks were not mined.")
	}
}

func TestAddBlockRejectsInvalidTransaction(t *testing.T) {
	_, alice := newKey(t)
	bobPub, _ := newKey(t)
	tx := NewTransaction(alice, bobPub, 10, 1)
	tx.Amount = 1000
	c := CreateChain(1)
	if c.AddBlock(NewBlock(map[string]string{"Name": "Block 1"}, tx)) == nil {
		t.Fatal("AddBlock should reject a block with an invalid transaction")
	}
	if c.Genesis.Next != nil {
		t.Fatal("Rejected block should not be added to the chain")
	}
}

func TestShouldNotBeAbleToSwapTransaction(t *testing.T) {
	alicePub, alice := newKey(t)
	bobPub, _ := newKey(t)
	malloryPub, mallory := newKey(t)
	c := CreateChain(1)
	if err := c.AddBlock(NewBlock(map[string]string{"Name": "Block 1"},
		NewCoinbase(alicePub, 1), NewTransaction(alice, bobPub, 10, 1))); err != nil {
		t.Fatal(err)
	}
	// a validly signed transaction, but not the one that was mined
	c.Genesis.Next.Transactions[1] = NewTransaction(mallory, malloryPub, 10, 1)
	if c.Validate() == nil {
		t.Fatal("Chain should not be valid, because a transaction was swapped out.")
	}
}

func TestGenesisIsTheSameEverywhere(t *testing.T) {
	if CreateChain(2).Genesis.Hash != CreateChain(2).Genesis.Hash {
		t.Fatal("Two chains with the same difficulty should share a genesis block")
	}
}

// copyBlocks round-trips blocks through JSON, as if they came from a peer
func copyBlocks(t *testing.T, blocks []*Block) []*Block {
	bytes, err := json.Marshal(blocks)
	if err != nil {
		t.Fatal(err)
	}
	var out []*Block
	if err := json.Unmarshal(bytes, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestAppendBlock(t *testing.T) {
	a := CreateChain(1)
	a.AddBlock(NewBlock(map[string]string{"Name": "Block 1"}))
	a.AddBlock(NewBlock(map[string]string{"Name": "Block 2"}))
	blocks := copyBlocks(t, a.Blocks())

	b := CreateChain(1)
	if b.AppendBlock(blocks[2]) != ErrNotOnTip {
		t.Fatal("Block 2 should not go straight onto the genesis block")
	}
	for _, blk := range blocks[1:] {
		if err := b.AppendBlock(blk); err != nil {
			t.Fatal(err)
		}
	}
	if b.Len() != 3 || b.Tip().Hash != a.Tip().Hash {
		t.Fatal("Chain should match the one the blocks came from")
	}
}

func TestAppendBlockRejectsUnminedBlock(t *testing.T) {
	c := CreateChain(3)
	b := NewBlock(map[string]string{"Name": "Block 1"})
	b.PreviousHash = c.Genesis.Hash
	b.Idx = 1
	b.Hash = b.GenerateHash()
	if err := c.AppendBlock(b); err == nil || err == ErrNotOnTip {
		t.Fatalf("Unmined block should be rejected as invalid, got %v", err)
	}
}

func TestLoadChain(t *testing.T) {
	a := CreateChain(1)
	a.AddBlock(NewBlock(map[string]string{"Name": "Block 1"}))
	a.AddBlock(NewBlock(map[string]string{"Name": "Block 2"}))

	c, err := LoadChain(1, copyBlocks(t, a.Blocks()))
	if err != nil {
		t.Fatal(err)
	}
	if c.Len() != 3 || c.Tip().Hash != a.Tip().Hash || c.Validate() != nil {
		t.Fatal("Loaded chain should match the original")
	}

	tampered := copyBlocks(t, a.Blocks())
	tampered[1].Data["Name"] = "I am now trying to screw you over"
	if _, err := LoadChain(1, tampered); err == nil {
		t.Fatal("LoadChain should reject a tampered block")
	}
}
