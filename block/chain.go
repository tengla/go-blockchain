package block

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"time"
)

// genesisTime is fixed so every node mines the same genesis block
var genesisTime = time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

// ErrNotOnTip means a block does not build on the last block of the chain
var ErrNotOnTip = errors.New("block does not extend the tip of the chain")

// Chain
type Chain struct {
	Created        time.Time
	Genesis        *Block
	Difficulty     int
	numberOfBlocks int
	ledger         *ledger
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
		ledger:     newLedger(),
	}
	b := NewBlock(map[string]string{
		"Name": "Genesis Block",
	})
	b.Created = genesisTime
	b.PreviousHash = "0000"
	b.MineBlock(difficulty)
	chain.Genesis = b
	return chain
}

// LoadChain rebuilds a chain from blocks in order, such as ones from a peer,
// checking every block on the way
func LoadChain(difficulty int, blocks []*Block) (*Chain, error) {
	if len(blocks) == 0 {
		return nil, errors.New("no blocks")
	}
	g := blocks[0]
	if g.Idx != 0 || !g.Verify(difficulty) {
		return nil, errors.New("invalid genesis block")
	}
	chain := &Chain{
		Created:    time.Now(),
		Genesis:    g,
		Difficulty: difficulty,
		ledger:     newLedger(),
	}
	for _, b := range blocks[1:] {
		if err := chain.AppendBlock(b); err != nil {
			return nil, err
		}
	}
	return chain, nil
}

// Len is the number of blocks, genesis included
func (c *Chain) Len() int {
	return c.numberOfBlocks + 1
}

// Tip is the last block of the chain
func (c *Chain) Tip() *Block {
	block := c.Genesis
	for block.Next != nil {
		block = block.Next
	}
	return block
}

// Has reports whether a block with this hash is in the chain
func (c *Chain) Has(hash string) bool {
	found := false
	c.EveryBlock(func(b *Block) {
		if b.Hash == hash {
			found = true
		}
	})
	return found
}

// Blocks returns the blocks in order, genesis first
func (c *Chain) Blocks() []*Block {
	var blocks []*Block
	c.EveryBlock(func(b *Block) {
		blocks = append(blocks, b)
	})
	return blocks
}

// Balance is what the owner of key has to spend
func (c *Chain) Balance(key ed25519.PublicKey) int {
	return c.ledger.balance(key)
}

// Nonce is the nonce of the last transaction sent by the owner of key, so
// their next one needs Nonce + 1
func (c *Chain) Nonce(key ed25519.PublicKey) int {
	return c.ledger.nonce(key)
}

// AddBlock mines b onto the end of the chain, unless it has an invalid
// transaction or one the sender can't afford
func (c *Chain) AddBlock(b *Block) error {
	if err := verifyTransactions(b.Transactions); err != nil {
		return err
	}
	ledger, err := c.ledger.apply(b.Transactions)
	if err != nil {
		return err
	}
	block := c.Tip()
	b.PreviousHash = block.Hash
	b.Idx = c.IncrNumberOfBlocks()
	b.MineBlock(c.Difficulty)
	block.Next = b
	c.ledger = ledger
	return nil
}

// AppendBlock adds a block that was already mined, such as one from a peer
func (c *Chain) AppendBlock(b *Block) error {
	block := c.Tip()
	if b.PreviousHash != block.Hash || b.Idx != block.Idx+1 {
		return ErrNotOnTip
	}
	if !b.Verify(c.Difficulty) {
		return fmt.Errorf("block %d is invalid", b.Idx)
	}
	ledger, err := c.ledger.apply(b.Transactions)
	if err != nil {
		return fmt.Errorf("block %d: %w", b.Idx, err)
	}
	c.IncrNumberOfBlocks()
	block.Next = b
	c.ledger = ledger
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
