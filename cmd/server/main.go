package main

import (
	"context"
	"crypto/ed25519"
	"flag"
	"log"
	"net"
	"strings"
	"time"

	"github.com/tengla/fibro/node"
	"github.com/tengla/fibro/wallet"
)

var (
	addr       = flag.String("addr", ":3000", "The address to listen on")
	self       = flag.String("url", "", "The URL peers reach this node at. Defaults to http://localhost and the port in -addr")
	peers      = flag.String("peers", "", "Comma-separated peer URLs, such as http://localhost:3001")
	difficulty = flag.Int("difficulty", 3, "The mining difficulty, which must match the peers'")
	miner      = flag.String("miner", "", "Mine blocks and pay the reward to this public key file. Without one, the node doesn't mine")
	interval   = flag.Duration("interval", 10*time.Second, "How often a miner mines a block")
	data       = flag.String("data", "", "A file to save the chain and peers in, and load them from when the node starts")
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
	if *data != "" {
		// A file that fails to load stops the node, instead of being
		// overwritten by a fresh chain
		if err := n.Persist(*data); err != nil {
			log.Fatal(err)
		}
	}
	n.Sync()
	if minerKey != nil {
		go n.MineEvery(context.Background(), *interval)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	url, err := selfURL()
	if err != nil {
		log.Fatal(err)
	}
	// Peers sync from us when we announce, so we must already be listening.
	// Their requests wait in the listen queue until the app starts serving.
	go n.Announce(url)

	log.Printf("listening on %s as %s with %d blocks and %d peers", *addr, url, len(n.Blocks()), len(n.Peers()))
	log.Fatal(n.App().Listener(ln))
}

func selfURL() (string, error) {
	if *self != "" {
		return strings.TrimSuffix(*self, "/"), nil
	}
	host, port, err := net.SplitHostPort(*addr)
	if err != nil {
		return "", err
	}
	if host == "" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port), nil
}
