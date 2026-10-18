package main

import (
	"flag"
	"log"
	"strings"

	"github.com/tengla/fibro/node"
)

var (
	addr       = flag.String("addr", ":3000", "The address to listen on")
	peers      = flag.String("peers", "", "Comma-separated peer URLs, such as http://localhost:3001")
	difficulty = flag.Int("difficulty", 3, "The mining difficulty, which must match the peers'")
)

func main() {

	flag.Parse()

	var peerList []string
	for _, p := range strings.Split(*peers, ",") {
		if p = strings.TrimSpace(p); p != "" {
			peerList = append(peerList, strings.TrimSuffix(p, "/"))
		}
	}

	n := node.New(*difficulty, peerList...)
	n.Sync()

	log.Printf("listening on %s with %d peers", *addr, len(peerList))
	log.Fatal(n.App().Listen(*addr))
}
