package block

import (
	"crypto/sha256"
	"fmt"
)

// MerkleRoot hashes the transactions pairwise, level by level, down to a
// single root. Leaves and inner nodes get different prefixes, as in
// RFC 6962, so an inner node can't be passed off as a transaction.
func MerkleRoot(txs []*Transaction) string {
	if len(txs) == 0 {
		return fmt.Sprintf("%x", sha256.Sum256(nil))
	}
	level := make([][]byte, len(txs))
	for i, t := range txs {
		level[i] = hashWithPrefix(0x00, t.Hash())
	}
	for len(level) > 1 {
		var next [][]byte
		for i := 0; i < len(level); i += 2 {
			if i+1 == len(level) {
				// An odd node moves up unchanged. Bitcoin duplicates it
				// instead, which lets [a b c] and [a b c c] share a root.
				next = append(next, level[i])
				continue
			}
			next = append(next, hashWithPrefix(0x01, level[i], level[i+1]))
		}
		level = next
	}
	return fmt.Sprintf("%x", level[0])
}

func hashWithPrefix(prefix byte, parts ...[]byte) []byte {
	sha := sha256.New()
	sha.Write([]byte{prefix})
	for _, p := range parts {
		sha.Write(p)
	}
	return sha.Sum(nil)
}
