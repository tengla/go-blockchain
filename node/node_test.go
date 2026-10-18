package node

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/tengla/fibro/block"
	"github.com/tengla/fibro/wallet"
)

// startNode runs a node on a free local port and returns it with its URL
func startNode(t *testing.T, difficulty int) (*Node, string) {
	return startMiner(t, difficulty, nil)
}

// startMiner is startNode for a node that pays its rewards to miner
func startMiner(t *testing.T, difficulty int, miner ed25519.PublicKey) (*Node, string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	n := New(difficulty, miner)
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
	tx := block.NewTransaction(alice, bobPub, 10, 1)
	tx.Amount = 1000
	if status := postTransaction(t, url, tx); status != http.StatusBadRequest {
		t.Fatalf("An invalid transaction should get status 400, got %d", status)
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

func postTransaction(t *testing.T, nodeURL string, tx *block.Transaction) int {
	body, _ := json.Marshal(tx)
	resp, err := http.Post(nodeURL+"/transactions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func getAccount(t *testing.T, nodeURL string, key ed25519.PublicKey) (balance, nonce, pending int) {
	resp, err := http.Get(nodeURL + "/account?key=" + url.QueryEscape(wallet.Encode(key)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var account struct{ Balance, Nonce, Pending int }
	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		t.Fatal(err)
	}
	return account.Balance, account.Nonce, account.Pending
}

func TestMinerIsPaidAndCanSpend(t *testing.T) {
	minerPub, miner, _ := ed25519.GenerateKey(nil)
	bobPub, _, _ := ed25519.GenerateKey(nil)
	a, aURL := startMiner(t, 1, minerPub)
	b, bURL := startNode(t, 1)
	connect(a, aURL, b, bURL)

	if _, err := a.Mine(nil); err != nil {
		t.Fatal(err)
	}
	if balance, _, _ := getAccount(t, bURL, minerPub); balance != block.InitialReward {
		t.Fatalf("Miner should have earned %d, has %d", block.InitialReward, balance)
	}

	// sent to b, which passes it on to the miner
	if status := postTransaction(t, bURL, block.NewTransaction(miner, bobPub, 10, 1)); status != http.StatusOK {
		t.Fatalf("Miner should be able to spend the reward, got status %d", status)
	}
	if len(a.Pending()) != 1 {
		t.Fatal("The transaction should have reached the miner's pool")
	}
	if balance, _, pending := getAccount(t, aURL, bobPub); balance != 0 || pending != 0 {
		t.Fatal("Bob should get nothing until the transaction is mined")
	}

	// the second block pays another reward, so the miner has 2 * Reward - 10
	if _, err := a.Mine(nil); err != nil {
		t.Fatal(err)
	}
	for name, n := range map[string]*Node{"a": a, "b": b} {
		if len(n.Pending()) != 0 {
			t.Fatalf("Node %s should have dropped the mined transaction from its pool", name)
		}
	}
	for _, u := range []string{aURL, bURL} {
		if balance, _, _ := getAccount(t, u, bobPub); balance != 10 {
			t.Fatalf("Bob should have 10 on %s, has %d", u, balance)
		}
		if balance, nonce, _ := getAccount(t, u, minerPub); balance != 2*block.InitialReward-10 || nonce != 1 {
			t.Fatalf("Miner should have %d and nonce 1 on %s, has %d and %d", 2*block.InitialReward-10, u, balance, nonce)
		}
	}
}

func TestPoolCountsPendingTransactions(t *testing.T) {
	alicePub, alice, _ := ed25519.GenerateKey(nil)
	bobPub, _, _ := ed25519.GenerateKey(nil)
	a, url := startMiner(t, 1, alicePub)
	a.Mine(nil)

	for nonce := 1; nonce <= 2; nonce++ {
		if status := postTransaction(t, url, block.NewTransaction(alice, bobPub, 20, nonce)); status != http.StatusOK {
			t.Fatalf("Transaction %d should be accepted, got status %d", nonce, status)
		}
	}
	if _, nonce, pending := getAccount(t, url, alicePub); nonce != 0 || pending != 2 {
		t.Fatalf("Alice should have nonce 0 and 2 pending, has %d and %d", nonce, pending)
	}
	// alice has 50 and 40 of it is already pending
	if status := postTransaction(t, url, block.NewTransaction(alice, bobPub, 20, 3)); status != http.StatusBadRequest {
		t.Fatalf("Spending coins that are already pending should get status 400, got %d", status)
	}
	if status := postTransaction(t, url, block.NewTransaction(alice, bobPub, 5, 2)); status != http.StatusBadRequest {
		t.Fatalf("A nonce already pending should get status 400, got %d", status)
	}

	b, _ := a.Mine(nil)
	if len(b.Transactions) != 3 {
		t.Fatalf("Block should have the coinbase and both transactions, has %d", len(b.Transactions))
	}
}

func TestPoolDropsTransactionsMinedElsewhere(t *testing.T) {
	alicePub, alice, _ := ed25519.GenerateKey(nil)
	bobPub, _, _ := ed25519.GenerateKey(nil)
	a, aURL := startMiner(t, 1, alicePub)
	b, bURL := startNode(t, 1)
	a.Mine(nil)
	connect(a, aURL, b, bURL)
	b.Sync()

	if err := a.AddTransaction(block.NewTransaction(alice, bobPub, 10, 1)); err != nil {
		t.Fatal(err)
	}
	if len(b.Pending()) != 1 {
		t.Fatal("Node b should have the transaction in its pool")
	}
	a.Mine(nil)
	if len(b.Pending()) != 0 {
		t.Fatal("Node b should drop a transaction once a peer's block includes it")
	}
}

func TestRejectsCoinbaseTransaction(t *testing.T) {
	_, url := startNode(t, 1)
	malloryPub, _, _ := ed25519.GenerateKey(nil)
	if status := postTransaction(t, url, block.NewCoinbase(malloryPub, 1)); status != http.StatusBadRequest {
		t.Fatalf("A coinbase from the caller should get status 400, got %d", status)
	}
}

func TestAccountRejectsBadKey(t *testing.T) {
	_, url := startNode(t, 1)
	resp, err := http.Get(url + "/account?key=nope")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("A malformed key should get status 400, got %d", resp.StatusCode)
	}
}

func TestMineEvery(t *testing.T) {
	minerPub, _, _ := ed25519.GenerateKey(nil)
	a, _ := startMiner(t, 1, minerPub)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		a.MineEvery(ctx, 10*time.Millisecond)
		close(done)
	}()
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done
	if len(a.Blocks()) < 3 {
		t.Fatalf("Miner should have mined several blocks, has %d", len(a.Blocks())-1)
	}
}

func TestMinerStopsBeingPaidWhenRewardRunsOut(t *testing.T) {
	minerPub, _, _ := ed25519.GenerateKey(nil)
	a, _ := startMiner(t, 1, minerPub)
	for i := 0; block.Reward(len(a.Blocks())) > 0; i++ {
		if i > 64*block.HalvingInterval {
			t.Fatal("The reward should run out")
		}
		if _, err := a.Mine(nil); err != nil {
			t.Fatal(err)
		}
	}
	balance, _, _ := a.Account(minerPub)
	b, err := a.Mine(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Transactions) != 0 {
		t.Fatal("Once the reward is 0 a block should have no coinbase")
	}
	if after, _, _ := a.Account(minerPub); balance != 9700 || after != balance {
		t.Fatalf("Miner should have all 9700 coins and no more, has %d then %d", balance, after)
	}
}

func TestNodeThatStartedFirstLearnsAboutLaterOnes(t *testing.T) {
	// a starts alone and mines, then b starts knowing only about a
	a, aURL := startNode(t, 1)
	a.Mine(map[string]string{"Name": "a1"})
	b, bURL := startNode(t, 1)
	b.AddPeer(aURL)
	b.Sync()
	b.Announce(bURL)

	if peers := a.Peers(); len(peers) != 1 || peers[0] != bURL {
		t.Fatalf("Node a should have b as its peer, has %v", peers)
	}
	mined, _ := a.Mine(map[string]string{"Name": "a2"})
	if len(b.Blocks()) != 3 || tipHash(b) != mined.Hash {
		t.Fatal("Node b should get the blocks a mines after b announced itself")
	}
}

func TestPeersOverHTTP(t *testing.T) {
	a, aURL := startNode(t, 1)
	a.AddPeer("http://localhost:3002/")
	resp, err := http.Get(aURL + "/peers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var peers []string
	if err := json.NewDecoder(resp.Body).Decode(&peers); err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0] != "http://localhost:3002" {
		t.Fatalf("Peers should list the peer without its trailing slash, got %v", peers)
	}
}
