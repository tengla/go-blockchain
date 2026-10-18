package node

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"

	"github.com/tengla/fibro/block"
)

func TestRestartsFromDataFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.json")
	minerPub, _, _ := ed25519.GenerateKey(nil)
	a := New(1, minerPub, "http://localhost:3002")
	if err := a.Persist(path); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		a.Mine(nil)
	}
	a.AddPeer("http://localhost:3003")

	// the same node, started again
	b := New(1, minerPub)
	if err := b.Persist(path); err != nil {
		t.Fatal(err)
	}
	if len(b.Blocks()) != 4 || tipHash(b) != tipHash(a) {
		t.Fatalf("Restarted node should have a's 4 blocks, has %d", len(b.Blocks()))
	}
	if balance, _, _ := b.Account(minerPub); balance != 3*block.InitialReward {
		t.Fatalf("Miner should still have %d, has %d", 3*block.InitialReward, balance)
	}
	if peers := b.Peers(); len(peers) != 2 || peers[1] != "http://localhost:3003" {
		t.Fatalf("Restarted node should have a's peers, has %v", peers)
	}
}

func TestSavesBlocksFromPeers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.json")
	a, aURL := startNode(t, 1)
	b, bURL := startNode(t, 1)
	if err := b.Persist(path); err != nil {
		t.Fatal(err)
	}
	connect(a, aURL, b, bURL)
	a.Mine(map[string]string{"Name": "a1"})

	c := New(1, nil)
	if err := c.Persist(path); err != nil {
		t.Fatal(err)
	}
	if len(c.Blocks()) != 2 || tipHash(c) != tipHash(a) {
		t.Fatal("Node b should have saved the block it got from a")
	}
}

func TestPersistRejectsTamperedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.json")
	a := New(1, nil)
	if err := a.Persist(path); err != nil {
		t.Fatal(err)
	}
	a.Mine(map[string]string{"Name": "Hello"})
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(data, []byte(`"Hello"`), []byte(`"Goodbye"`), 1)
	if err := os.WriteFile(path, tampered, 0644); err != nil {
		t.Fatal(err)
	}

	if err := New(1, nil).Persist(path); err == nil {
		t.Fatal("A tampered file should not load")
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(after, tampered) {
		t.Fatal("A file that fails to load should be left as it is")
	}
}

func TestPersistRejectsDifferentDifficulty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chain.json")
	a := New(1, nil)
	if err := a.Persist(path); err != nil {
		t.Fatal(err)
	}
	a.Mine(nil)
	if err := New(2, nil).Persist(path); err == nil {
		t.Fatal("A chain mined at another difficulty should not load")
	}
}

func TestPersistFailsWhenFileCantBeWritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "chain.json")
	if err := New(1, nil).Persist(path); err == nil {
		t.Fatal("Persist should fail when the data file can't be written")
	}
}
