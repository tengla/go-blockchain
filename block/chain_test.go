package block

import (
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
	tx := NewTransaction(alice, bobPub, 10)
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
	_, alice := newKey(t)
	bobPub, _ := newKey(t)
	malloryPub, mallory := newKey(t)
	c := CreateChain(1)
	if err := c.AddBlock(NewBlock(map[string]string{"Name": "Block 1"},
		NewTransaction(alice, bobPub, 10))); err != nil {
		t.Fatal(err)
	}
	// a validly signed transaction, but not the one that was mined
	c.Genesis.Next.Transactions[0] = NewTransaction(mallory, malloryPub, 10)
	if c.Validate() == nil {
		t.Fatal("Chain should not be valid, because a transaction was swapped out.")
	}
}
