package main

import (
	"crypto/ed25519"
	"flag"
	"fmt"
	"log"

	"github.com/tengla/fibro/block"
)

var difficulty = flag.Int("difficulty", 2, "The mining difficulty")

func main() {

	flag.Parse()

	alicePub, alice, err := ed25519.GenerateKey(nil)
	if err != nil {
		log.Fatal(err)
	}
	bobPub, bob, err := ed25519.GenerateKey(nil)
	if err != nil {
		log.Fatal(err)
	}

	chain := block.CreateChain(*difficulty)

	add := func(b *block.Block) {
		if err := chain.AddBlock(b); err != nil {
			log.Fatal(err)
		}
	}

	// alice mines the first block, so she has coins to send
	add(block.NewBlock(map[string]string{
		"Name": "I am the first",
	}, block.NewCoinbase(alicePub)))

	add(block.NewBlock(map[string]string{
		"Name": "I am the second",
	}, block.NewTransaction(alice, bobPub, 10, 1)))

	add(block.NewBlock(map[string]string{
		"Name": "I am the third", "Note": "Funny",
	}, block.NewTransaction(bob, alicePub, 3, 1), block.NewTransaction(alice, bobPub, 5, 2)))

	add(block.NewBlock(map[string]string{
		"Name": "I am the fourth",
	}))

	add(block.NewBlock(map[string]string{
		"Name": "I am the fifth",
	}))

	add(block.NewBlock(map[string]string{
		"Name": "I am the sixth",
	}))

	chain.EveryBlock(func(b *block.Block) {
		fmt.Printf("%s\n", b.ToJSON())
	})
}
