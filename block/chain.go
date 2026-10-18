package block

import (
	"fmt"
	"time"
)

// Chain
type Chain struct {
	Created        time.Time
	Genesis        *Block
	Difficulty     int
	numberOfBlocks int
}

// IncrNumberOfBlocks
func (c *Chain) IncrNumberOfBlocks() int {
	c.numberOfBlocks = c.numberOfBlocks + 1
	return c.numberOfBlocks
}

// CreateChain
func CreateChain(difficulty int) *Chain {
	chain := &Chain{
		Created:    time.Now(),
		Difficulty: difficulty,
	}
	b := NewBlock(map[string]string{
		"Name": "Genesis Block",
	})
	b.PreviousHash = "0000"
	b.MineBlock(difficulty)
	chain.Genesis = b
	return chain
}

// AddBlock mines b onto the end of the chain, unless it has an invalid transaction
func (c *Chain) AddBlock(b *Block) error {
	for _, t := range b.Transactions {
		if !t.Verify() {
			return fmt.Errorf("invalid transaction from %x", t.From)
		}
	}
	block := c.Genesis
	prevHash := block.Hash
	for block.Next != nil {
		block = block.Next
		prevHash = block.Hash
	}
	b.PreviousHash = prevHash
	b.Idx = c.IncrNumberOfBlocks()
	b.MineBlock(c.Difficulty)
	block.Next = b
	return nil
}

// Validate
func (c *Chain) Validate() *Block {
	var foundInvalid *Block = nil
	c.EveryBlock(func(b *Block) {
		if !b.Verify(c.Difficulty) {
			foundInvalid = b
		}
	})
	return foundInvalid
}

// EveryBlock
func (c *Chain) EveryBlock(fn func(b *Block)) {
	cur := c.Genesis
	for cur.Next != nil {
		fn(cur)
		cur = cur.Next
	}
	fn(cur)
}
