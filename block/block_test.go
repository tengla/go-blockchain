package block

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewBlock(t *testing.T) {
	b := NewBlock(map[string]string{
		"Name": "Test",
	})
	if b == nil {
		t.Fatal("Could not create Block")
	}
	if b.Data["Name"] != "Test" {
		t.Fatal("Block.Data['Name'] is not 'Test'")
	}
	// should be very recent
	if time.Since(b.Created) > time.Duration(time.Second) {
		t.Fatal("Block.Created was not set correctly")
	}
}

func TestMineBlock(t *testing.T) {
	// g for genesis
	g := NewBlock(map[string]string{
		"Name": "Genesis",
	})
	g.Idx = 1
	g.PreviousHash = "0000"
	g.Hash = g.GenerateHash()
	if g.Hash != g.GenerateHash() {
		t.Fatal("Hash is not deterministic.")
	}
}

func TestHashSurvivesJSONRoundTrip(t *testing.T) {
	_, alice := newKey(t)
	bobPub, _ := newKey(t)
	c := CreateChain(1)
	c.AddBlock(NewBlock(map[string]string{
		"Name": "Block 1",
	}, NewTransaction(alice, bobPub, 10)))
	var b Block
	if err := json.Unmarshal([]byte(c.Genesis.Next.ToJSON()), &b); err != nil {
		t.Fatal(err)
	}
	if b.Hash != b.GenerateHash() {
		t.Fatal("Hash changed after a JSON round trip.")
	}
	if !b.Verify(c.Difficulty) {
		t.Fatal("Block does not verify after a JSON round trip.")
	}
}
