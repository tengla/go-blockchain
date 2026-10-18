package node

import (
	"bytes"
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

// Node keeps a copy of the chain and shares new blocks with its peers
type Node struct {
	mu    sync.Mutex
	chain *block.Chain
	peers []string
	miner ed25519.PublicKey
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

// Account returns what the owner of key has to spend, and the nonce of the
// last transaction they sent
func (n *Node) Account(key ed25519.PublicKey) (balance, nonce int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.chain.Balance(key), n.chain.Nonce(key)
}

// Mine mines a new block onto the chain and sends it to every peer
func (n *Node) Mine(data map[string]string, txs ...*block.Transaction) (*block.Block, error) {
	for _, t := range txs {
		// otherwise anyone could post a coinbase that pays themselves
		if t.IsCoinbase() {
			return nil, errors.New("the node adds the coinbase itself")
		}
	}
	if n.miner != nil {
		txs = append([]*block.Transaction{block.NewCoinbase(n.miner)}, txs...)
	}
	n.mu.Lock()
	b := block.NewBlock(data, txs...)
	err := n.chain.AddBlock(b)
	n.mu.Unlock()
	if err != nil {
		return nil, err
	}
	n.broadcast(b)
	return b, nil
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
	n.mu.Unlock()

	switch {
	case err == nil:
		n.broadcast(b)
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
		}
		n.mu.Unlock()

		if err != nil {
			log.Printf("sync from %s: %v", p, err)
		}
		if replaced {
			// let peers that are behind us catch up too
			n.broadcast(candidate.Tip())
		}
	}
}

// App returns the node's HTTP API:
//
//	GET  /blocks            the node's chain
//	POST /blocks            receive a block from a peer
//	GET  /account?key=...   a public key's {"Balance": 50, "Nonce": 1}
//	POST /mine              mine a block: {"Data": {...}, "Transactions": [...]}
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
		var resp struct{ Balance, Nonce int }
		resp.Balance, resp.Nonce = n.Account(key)
		return sendJSON(c, resp)
	})

	app.Post("/mine", func(c *fiber.Ctx) error {
		var req struct {
			Data         map[string]string
			Transactions []*block.Transaction
		}
		if err := json.Unmarshal(c.Body(), &req); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		b, err := n.Mine(req.Data, req.Transactions...)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		return sendJSON(c, b)
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

// broadcast sends a block to every peer. A peer that is down is skipped.
func (n *Node) broadcast(b *block.Block) {
	body, err := json.Marshal(b)
	if err != nil {
		log.Printf("broadcast: %v", err)
		return
	}
	for _, p := range n.peerList() {
		resp, err := client.Post(p+"/blocks", "application/json", bytes.NewReader(body))
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
