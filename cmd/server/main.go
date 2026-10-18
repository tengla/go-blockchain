package main

import (
	"context"
	"crypto/ed25519"
	"flag"
	"log"
	"strings"
	"time"

	"github.com/tengla/fibro/node"
	"github.com/tengla/fibro/wallet"
)

var (
	addr       = flag.String("addr", ":3000", "The address to listen on")
	peers      = flag.String("peers", "", "Comma-separated peer URLs, such as http://localhost:3001")
	difficulty = flag.Int("difficulty", 3, "The mining difficulty, which must match the peers'")
	miner      = flag.String("miner", "", "Mine blocks and pay the reward to this public key file. Without one, the node doesn't mine")
	interval   = flag.Duration("interval", 10*time.Second, "How often a miner mines a block")
)

func main() {

	flag.Parse()

	var peerList []string
	for _, p := range strings.Split(*peers, ",") {
		if p = strings.TrimSpace(p); p != "" {
			peerList = append(peerList, strings.TrimSuffix(p, "/"))
		}
	}

	var minerKey ed25519.PublicKey
	if *miner != "" {
		var err error
		if minerKey, err = wallet.ReadPublic(*miner); err != nil {
			log.Fatal(err)
		}
	}

	n := node.New(*difficulty, minerKey, peerList...)
	n.Sync()
	if minerKey != nil {
		go n.MineEvery(context.Background(), *interval)
	}

	log.Printf("listening on %s with %d peers", *addr, len(peerList))
	log.Fatal(n.App().Listen(*addr))
}
