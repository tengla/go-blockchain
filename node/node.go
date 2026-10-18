package node

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/tengla/fibro/block"
	"github.com/tengla/fibro/wallet"
)

// client is shared by every node in the process, for talking to peers
var client = &http.Client{Timeout: 10 * time.Second}

// Node keeps a copy of the chain and shares new blocks and transactions
// with its peers. Transactions wait in a pool until a block includes them.
type Node struct {
	mu      sync.Mutex
	chain   *block.Chain
	peers   []string
	miner   ed25519.PublicKey
	pending []*block.Transaction
}

// New creates a node with a fresh chain. The reward for each block it mines
// goes to miner, or to no one if miner is nil. Peers are base URLs such as
// http://localhost:3001
func New(difficulty int, miner ed25519.PublicKey, peers ...string) *Node {
	n := &Node{
		chain: block.CreateChain(difficulty),
		miner: miner,
	}
	for _, p := range peers {
		n.AddPeer(p)
	}
	return n
}

// AddPeer
func (n *Node) AddPeer(url string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, p := range n.peers {
		if p == url {
			return
		}
	}
	n.peers = append(n.peers, url)
}

// Blocks returns the node's current chain, genesis first
func (n *Node) Blocks() []*block.Block {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.chain.Blocks()
}

// Account returns what the owner of key has to spend in mined blocks, the
// nonce of the last transaction they had mined, and how many of theirs are
// waiting in the pool. Their next transaction needs nonce + pending + 1.
func (n *Node) Account(key ed25519.PublicKey) (balance, nonce, pending int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, t := range n.pending {
		if bytes.Equal(t.From, key) {
			pending++
		}
	}
	return n.chain.Balance(key), n.chain.Nonce(key), pending
}

// Pending returns the transactions waiting to be mined, in order
func (n *Node) Pending() []*block.Transaction {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]*block.Transaction{}, n.pending...)
}

// AddTransaction puts a transaction in the pool and sends it to every peer.
// It must fit after the ones already waiting, so a sender can't spend the
// same coins twice before a block is mined.
func (n *Node) AddTransaction(t *block.Transaction) error {
	// otherwise anyone could add a coinbase that pays themselves
	if t.IsCoinbase() {
		return errors.New("only a miner can add a coinbase")
	}
	n.mu.Lock()
	for _, p := range n.pending {
		if bytes.Equal(p.Hash(), t.Hash()) {
			// already have it, which also stops it going round the peers forever
			n.mu.Unlock()
			return nil
		}
	}
	err := n.chain.CheckTransactions(append(n.pending, t))
	if err == nil {
		n.pending = append(n.pending, t)
	}
	n.mu.Unlock()
	if err != nil {
		return err
	}
	n.broadcast("/transactions", t)
	return nil
}

// Mine mines a block with every pending transaction onto the chain and
// sends it to every peer. The block starts with a coinbase if the node has
// a miner key.
func (n *Node) Mine(data map[string]string) (*block.Block, error) {
	n.mu.Lock()
	var txs []*block.Transaction
	if n.miner != nil {
		txs = append(txs, block.NewCoinbase(n.miner))
	}
	b := block.NewBlock(data, append(txs, n.pending...)...)
	err := n.chain.AddBlock(b)
	if err == nil {
		n.pending = nil
	}
	n.mu.Unlock()
	if err != nil {
		return nil, err
	}
	n.broadcast("/blocks", b)
	return b, nil
}

// MineEvery mines a block every interval until ctx is done. Blocks are
// mined even with no pending transactions, since that is how a miner earns
// its first coins.
func (n *Node) MineEvery(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := n.Mine(nil); err != nil {
				log.Printf("mine: %v", err)
			}
		}
	}
}

// Receive handles a block from a peer. A block that extends the chain is
// added and passed on. A block from further ahead means a peer has a longer
// chain, maybe on a fork, so the node syncs.
func (n *Node) Receive(b *block.Block) error {
	n.mu.Lock()
	if n.chain.Has(b.Hash) {
		n.mu.Unlock()
		return nil
	}
	tip := n.chain.Tip()
	err := n.chain.AppendBlock(b)
	if err == nil {
		n.prune()
	}
	n.mu.Unlock()

	switch {
	case err == nil:
		n.broadcast("/blocks", b)
	case errors.Is(err, block.ErrNotOnTip):
		if b.Idx > tip.Idx {
			n.Sync()
		}
	default:
		return err
	}
	return nil
}

// Sync downloads every peer's chain and switches to the longest valid one
// that shares our genesis block. With a fixed difficulty every block takes
// the same work, so the chain with the most work is the longest one. On a
// tie the node keeps the chain it already has.
func (n *Node) Sync() {
	for _, p := range n.peerList() {
		blocks, err := n.fetchBlocks(p)
		if err != nil {
			log.Printf("sync from %s: %v", p, err)
			continue
		}
		n.mu.Lock()
		candidate, err := block.LoadChain(n.chain.Difficulty, blocks)
		if err == nil && candidate.Genesis.Hash != n.chain.Genesis.Hash {
			err = errors.New("different genesis block")
		}
		replaced := err == nil && candidate.Len() > n.chain.Len()
		if replaced {
			n.chain = candidate
			n.prune()
		}
		n.mu.Unlock()

		if err != nil {
			log.Printf("sync from %s: %v", p, err)
		}
		if replaced {
			// let peers that are behind us catch up too
			n.broadcast("/blocks", candidate.Tip())
		}
	}
}

// App returns the node's HTTP API:
//
//	GET  /blocks            the node's chain
//	POST /blocks            receive a block from a peer
//	GET  /transactions      the transactions waiting to be mined
//	POST /transactions      add a transaction to the pool: {"From": ..., "To": ..., ...}
//	GET  /account?key=...   a public key's {"Balance": 50, "Nonce": 1, "Pending": 0}
//	POST /peers             add a peer and sync from it: {"URL": "http://..."}
func (n *Node) App() *fiber.App {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})

	app.Get("/blocks", func(c *fiber.Ctx) error {
		return sendJSON(c, n.Blocks())
	})

	app.Post("/blocks", func(c *fiber.Ctx) error {
		var b block.Block
		if err := json.Unmarshal(c.Body(), &b); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		if err := n.Receive(&b); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/account", func(c *fiber.Ctx) error {
		key, err := wallet.DecodePublic(c.Query("key"))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString("key: " + err.Error())
		}
		var resp struct{ Balance, Nonce, Pending int }
		resp.Balance, resp.Nonce, resp.Pending = n.Account(key)
		return sendJSON(c, resp)
	})

	app.Get("/transactions", func(c *fiber.Ctx) error {
		return sendJSON(c, n.Pending())
	})

	app.Post("/transactions", func(c *fiber.Ctx) error {
		var t block.Transaction
		if err := json.Unmarshal(c.Body(), &t); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		if err := n.AddTransaction(&t); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		return c.SendStatus(fiber.StatusOK)
	})

	app.Post("/peers", func(c *fiber.Ctx) error {
		var req struct{ URL string }
		if err := json.Unmarshal(c.Body(), &req); err != nil || req.URL == "" {
			return c.Status(fiber.StatusBadRequest).SendString("expected {\"URL\": \"http://...\"}")
		}
		n.AddPeer(req.URL)
		n.Sync()
		return c.SendStatus(fiber.StatusOK)
	})

	return app
}

func (n *Node) peerList() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.peers...)
}

// prune drops pending transactions that no longer fit on the chain, such as
// ones a new block already includes. The caller holds n.mu.
func (n *Node) prune() {
	var keep []*block.Transaction
	for _, t := range n.pending {
		if n.chain.CheckTransactions(append(keep, t)) == nil {
			keep = append(keep, t)
		}
	}
	n.pending = keep
}

// broadcast posts a block or transaction to path on every peer. A peer that
// is down is skipped.
func (n *Node) broadcast(path string, v interface{}) {
	body, err := json.Marshal(v)
	if err != nil {
		log.Printf("broadcast: %v", err)
		return
	}
	for _, p := range n.peerList() {
		resp, err := client.Post(p+path, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("broadcast to %s: %v", p, err)
			continue
		}
		resp.Body.Close()
	}
}

func (n *Node) fetchBlocks(peer string) ([]*block.Block, error) {
	resp, err := client.Get(peer + "/blocks")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var blocks []*block.Block
	if err := json.NewDecoder(resp.Body).Decode(&blocks); err != nil {
		return nil, err
	}
	return blocks, nil
}

// sendJSON uses encoding/json so peers decode exactly what was encoded
func sendJSON(c *fiber.Ctx, v interface{}) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Send(body)
}
