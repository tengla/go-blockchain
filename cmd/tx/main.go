package main

import (
	"crypto/ed25519"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/tengla/fibro/block"
	"github.com/tengla/fibro/wallet"
)

var (
	keygen = flag.String("keygen", "", "Write a new key pair to NAME.key and NAME.pub, then exit")
	from   = flag.String("from", "", "The sender's private key file")
	to     = flag.String("to", "", "The recipient's public key file")
	amount = flag.Int("amount", 0, "The amount to send")
	nonce  = flag.Int("nonce", 0, "The transaction's nonce. Without one, the next nonce is fetched from -node")
	node   = flag.String("node", "http://localhost:3001", "The node to fetch the nonce from")
)

func main() {

	flag.Parse()

	if *keygen != "" {
		if err := wallet.Create(*keygen); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *from == "" || *to == "" || *amount <= 0 {
		fmt.Fprintln(os.Stderr, "usage: tx -from alice.key -to bob.pub -amount 10")
		flag.PrintDefaults()
		os.Exit(2)
	}

	priv, err := wallet.ReadPrivate(*from)
	if err != nil {
		log.Fatal(err)
	}
	pub, err := wallet.ReadPublic(*to)
	if err != nil {
		log.Fatal(err)
	}
	if *nonce == 0 {
		if *nonce, err = nextNonce(priv.Public().(ed25519.PublicKey)); err != nil {
			log.Fatal(err)
		}
	}

	// The output is a body for POST /mine
	req := struct {
		Transactions []*block.Transaction
	}{
		Transactions: []*block.Transaction{block.NewTransaction(priv, pub, *amount, *nonce)},
	}
	if err := json.NewEncoder(os.Stdout).Encode(req); err != nil {
		log.Fatal(err)
	}
}

// nextNonce asks the node for the sender's last nonce
func nextNonce(key ed25519.PublicKey) (int, error) {
	resp, err := http.Get(*node + "/account?key=" + url.QueryEscape(wallet.Encode(key)))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%s: status %d", *node, resp.StatusCode)
	}
	var account struct{ Nonce int }
	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return 0, err
	}
	return account.Nonce + 1, nil
}
