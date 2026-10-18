# Block Chain experiment

This is a block chain experiment written in Golang.

Build and run:

```
make
./dist/block-chain -difficulty 3
```

Run a small network of nodes, each in its own terminal. Peering should go both
ways, so a node that falls behind can download the longer chain:

```
./dist/node -addr :3001 -peers http://localhost:3002
./dist/node -addr :3002 -peers http://localhost:3001,http://localhost:3003
./dist/node -addr :3003 -peers http://localhost:3002
```

Mine a block on one node and see it on the others:

```
curl -X POST localhost:3001/mine -d '{"Data": {"Name": "Hello"}}'
curl localhost:3003/blocks
```

### Sending coins

A wallet is just a key pair. The chain works out each key's balance from its
transactions, and rejects a block where anyone spends more than they have.
Coins are created by mining: each block a node mines pays a reward of 50 to
its `-miner` key.

Make key pairs and start a node that pays alice for mining:

```
./dist/tx -keygen alice
./dist/tx -keygen bob
./dist/node -addr :3001 -miner alice.pub
```

Mine a block to earn the reward, then send some of it to bob. `tx` signs the
transaction and fetches alice's next nonce from the node, so the same
transaction can't be mined twice:

```
curl -X POST localhost:3001/mine -d '{}'
./dist/tx -from alice.key -to bob.pub -amount 10 | curl -X POST localhost:3001/mine -d @-
curl -G localhost:3001/account --data-urlencode key="$(cat bob.pub)"
```
