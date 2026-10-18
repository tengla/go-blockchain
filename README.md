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
