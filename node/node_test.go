package node

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"net"
	"net/http"
	"testing"

	"github.com/tengla/fibro/block"
)

// startNode runs a node on a free local port and returns it with its URL
func startNode(t *testing.T, difficulty int) (*Node, string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	n := New(difficulty)
	app := n.App()
	go app.Listener(ln)
	t.Cleanup(func() {
		// Shutdown waits for open connections, including idle keep-alive ones
		client.CloseIdleConnections()
		http.DefaultClient.CloseIdleConnections()
		app.Shutdown()
	})
	return n, "http://" + ln.Addr().String()
}

func connect(a *Node, aURL string, b *Node, bURL string) {
	a.AddPeer(bURL)
	b.AddPeer(aURL)
}

func tipHash(n *Node) string {
	blocks := n.Blocks()
	return blocks[len(blocks)-1].Hash
}

func TestMinedBlockReachesEveryNode(t *testing.T) {
	a, aURL := startNode(t, 1)
	b, bURL := startNode(t, 1)
	c, cURL := startNode(t, 1)
	// a and c only reach each other through b
	connect(a, aURL, b, bURL)
	connect(b, bURL, c, cURL)

	mined, err := a.Mine(map[string]string{"Name": "Block 1"})
	if err != nil {
		t.Fatal(err)
	}
	for name, n := range map[string]*Node{"b": b, "c": c} {
		if len(n.Blocks()) != 2 || tipHash(n) != mined.Hash {
			t.Fatalf("Node %s did not get the mined block", name)
		}
	}
}

func TestNodeCatchesUpWhenAddingPeer(t *testing.T) {
	a, aURL := startNode(t, 1)
	_, bURL := startNode(t, 1)
	for i := 0; i < 3; i++ {
		a.Mine(map[string]string{"Name": "Block"})
	}
	resp, err := http.Post(bURL+"/peers", "application/json",
		bytes.NewReader([]byte(`{"URL": "`+aURL+`"}`)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	var blocks []*block.Block
	resp, err = http.Get(bURL + "/blocks")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&blocks); err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 4 || blocks[3].Hash != tipHash(a) {
		t.Fatalf("Node should have caught up to 4 blocks, has %d", len(blocks))
	}
}

func TestForkResolvesToLongerChain(t *testing.T) {
	a, aURL := startNode(t, 1)
	b, bURL := startNode(t, 1)

	// while apart, a and b mine competing blocks on the same genesis
	a.Mine(map[string]string{"Name": "a1"})
	a.Mine(map[string]string{"Name": "a2"})
	orphan, _ := b.Mine(map[string]string{"Name": "b1"})

	connect(a, aURL, b, bURL)
	a.Mine(map[string]string{"Name": "a3"})

	if len(b.Blocks()) != 4 || tipHash(b) != tipHash(a) {
		t.Fatal("Node b should have switched to a's longer chain")
	}
	for _, blk := range b.Blocks() {
		if blk.Hash == orphan.Hash {
			t.Fatal("Node b's own block should have been dropped")
		}
	}
}

func TestTiedForkKeepsOwnChain(t *testing.T) {
	a, aURL := startNode(t, 1)
	b, bURL := startNode(t, 1)
	a.Mine(map[string]string{"Name": "a1"})
	own, _ := b.Mine(map[string]string{"Name": "b1"})

	connect(a, aURL, b, bURL)
	b.Sync()

	if tipHash(b) != own.Hash {
		t.Fatal("On a tie the node should keep the chain it has")
	}
}

func TestRejectsInvalidBlock(t *testing.T) {
	a, _ := startNode(t, 1)
	_, bURL := startNode(t, 1)
	mined, _ := a.Mine(map[string]string{"Name": "Block 1"})
	mined.Data["Name"] = "I am now trying to screw you over"
	body, _ := json.Marshal(mined)

	resp, err := http.Post(bURL+"/blocks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Tampered block should get status 400, got %d", resp.StatusCode)
	}
}

func TestRejectsInvalidTransactionOverHTTP(t *testing.T) {
	_, url := startNode(t, 1)
	_, alice, _ := ed25519.GenerateKey(nil)
	bobPub, _, _ := ed25519.GenerateKey(nil)
	tx := block.NewTransaction(alice, bobPub, 10)
	tx.Amount = 1000
	body, _ := json.Marshal(map[string]interface{}{
		"Data":         map[string]string{"Name": "Block 1"},
		"Transactions": []*block.Transaction{tx},
	})

	resp, err := http.Post(url+"/mine", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Mining an invalid transaction should get status 400, got %d", resp.StatusCode)
	}
}

func TestIgnoresChainWithDifferentDifficulty(t *testing.T) {
	a, aURL := startNode(t, 1)
	b, bURL := startNode(t, 2)
	for i := 0; i < 3; i++ {
		a.Mine(map[string]string{"Name": "Block"})
	}
	connect(a, aURL, b, bURL)
	b.Sync()
	if len(b.Blocks()) != 1 {
		t.Fatal("Node should not adopt a chain mined at a different difficulty")
	}
}
