package block

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// Block
type Block struct {
	Idx          int
	Data         map[string]string
	Transactions []*Transaction
	MerkleRoot   string
	Created      time.Time
	Hash         string
	PreviousHash string
	Nonce        int
	Next         *Block `json:"-"`
}

func (b *Block) MineBlock(difficulty int) {
	b.MerkleRoot = MerkleRoot(b.Transactions)
	for {
		b.Nonce += 1
		b.Hash = b.GenerateHash()
		if b.MeetsDifficulty(difficulty) {
			break
		}
	}
}

// MeetsDifficulty reports whether the hash has the required leading zeros
func (b *Block) MeetsDifficulty(difficulty int) bool {
	return strings.HasPrefix(b.Hash, strings.Repeat("0", difficulty))
}

// NewBlock
func NewBlock(data map[string]string, txs ...*Transaction) *Block {
	b := &Block{
		Data:         data,
		Transactions: txs,
		Created:      time.Now(),
		Nonce:        0,
	}
	return b
}

// GenerateHash
func (b *Block) GenerateHash() string {
	sha := sha256.New()
	// Created is hashed as Unix nanoseconds, since its string form includes
	// the monotonic clock reading, which is lost when the block is serialized.
	// Fields are separated so different inputs can't produce the same string.
	// Transactions are covered through MerkleRoot.
	str := fmt.Sprintf("%d|%d|%s|%d|%s|%s",
		b.Nonce, b.Idx, b.PreviousHash, b.Created.UnixNano(), b.MerkleRoot, b.DataToJSON())
	_, err := sha.Write([]byte(str))
	if err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%x", sha.Sum(nil))
}

func (b *Block) Verify(difficulty int) bool {
	for _, t := range b.Transactions {
		if !t.Verify() {
			return false
		}
	}
	if b.MerkleRoot != MerkleRoot(b.Transactions) {
		return false
	}
	if b.Hash != b.GenerateHash() {
		return false
	}
	if !b.MeetsDifficulty(difficulty) {
		return false
	}
	if b.Next == nil {
		return true
	}
	return b.Hash == b.Next.PreviousHash
}

func (b *Block) DataToJSON() string {
	bytes, err := json.Marshal(b.Data)
	if err != nil {
		log.Fatal(err)
	}
	return string(bytes)
}

func (b *Block) ToJSON() string {
	bytes, err := json.Marshal(b)
	if err != nil {
		log.Fatal(err)
	}
	return string(bytes)
}
