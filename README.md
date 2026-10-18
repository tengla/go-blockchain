# Block Chain experiment

This is a block chain experiment written in Golang.

Build and run:

```
make
./dist/block-chain -difficulty 3
```

Run a small network of nodes, each in its own terminal. A node only needs to
know one node that is already running: when it starts, it announces itself to
its `-peers`, which add it as a peer too. Peers reach it at `-url`, which
defaults to `http://localhost` and the port in `-addr`.

A node started with `-miner` mines a block every `-interval` (10s by default)
and is paid a reward for each one. The reward starts at 50 and halves every 100
blocks, so no more than 9700 coins are ever made. The others only keep a copy
of the chain:

```
./dist/tx -keygen alice
./dist/node -addr :3001 -miner alice.pub
./dist/node -addr :3002 -peers http://localhost:3001
./dist/node -addr :3003 -peers http://localhost:3002
```

Blocks mined on the first node show up on the others, and each node lists its
peers:

```
curl localhost:3003/blocks
curl localhost:3001/peers
```

A node keeps the chain in memory, so it is lost when every node is down. Give
a node a `-data` file to save its chain and peers in after every change. When
it starts again, it loads them back and checks every block, so a damaged or
edited file stops the node instead of being overwritten:

```
./dist/node -addr :3001 -miner alice.pub -data chain.json
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
