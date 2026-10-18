package node

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/tengla/fibro/block"
)

// saved is what the node keeps on disk. Pending transactions are left out,
// since senders can submit them again.
type saved struct {
	Blocks []*block.Block
	Peers  []string
}

// Persist loads the chain and peers saved at path, if the file exists, and
// saves them there whenever they change. The saved chain is checked block by
// block, like a chain from a peer, so a damaged or edited file is an error
// instead of being trusted.
func (n *Node) Persist(path string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		// a new node, which starts from the genesis block
	case err != nil:
		return err
	default:
		var s saved
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("%s: %v", path, err)
		}
		chain, err := block.LoadChain(n.chain.Difficulty, s.Blocks)
		if err == nil && chain.Genesis.Hash != n.chain.Genesis.Hash {
			err = errors.New("different genesis block, so maybe another difficulty")
		}
		if err != nil {
			return fmt.Errorf("%s: %v", path, err)
		}
		n.chain = chain
		for _, p := range s.Peers {
			n.addPeer(p)
		}
	}

	n.data = path
	// saving now also checks that the file can be written
	return n.save()
}

// store saves the node, logging any error, since what changed has already
// happened. The caller holds n.mu.
func (n *Node) store() {
	if err := n.save(); err != nil {
		log.Printf("save %s: %v", n.data, err)
	}
}

// save writes the chain and peers to the data file, if the node has one. It
// writes a new file and renames it over the old one, so a crash part way
// through leaves the old file whole. The caller holds n.mu.
func (n *Node) save() error {
	if n.data == "" {
		return nil
	}
	data, err := json.Marshal(saved{Blocks: n.chain.Blocks(), Peers: n.peers})
	if err != nil {
		return err
	}
	tmp := n.data + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, n.data)
}
