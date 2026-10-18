# Block Chain experiment

This is a block chain experiment written in Golang.

Build and run:

```
make
./dist/block-chain -difficulty 3
```

Run a small network of nodes, each in its own terminal. Peering should go both
ways, so a node that falls behind can download the longer chain. A node started
with `-miner` mines a block every `-interval` (10s by default) and is paid a
reward of 50 for each one. The others only keep a copy of the chain:

```
./dist/tx -keygen alice
./dist/node -addr :3001 -peers http://localhost:3002 -miner alice.pub
./dist/node -addr :3002 -peers http://localhost:3001,http://localhost:3003
./dist/node -addr :3003 -peers http://localhost:3002
```

Blocks mined on the first node show up on the others:

```
curl localhost:3003/blocks
```

### Sending coins

A wallet is just a key pair. The chain works out each key's balance from its
transactions, and rejects a block where anyone spends more than they have.
Coins are created by mining, so alice has some once the first block is mined.

Send some of them to bob. `tx` signs the transaction and fetches alice's next
nonce from a node, so the same transaction can't be mined twice. Any node
accepts it into its pool of pending transactions and passes it on to its peers,
and the miner includes it in the next block:

```
./dist/tx -keygen bob
./dist/tx -from alice.key -to bob.pub -amount 10 | curl -X POST localhost:3003/transactions -d @-
curl localhost:3003/transactions
curl -G localhost:3003/account --data-urlencode key="$(cat bob.pub)"
```

`/account` shows the balance in mined blocks, so bob's 10 appears once the next
block is mined.
