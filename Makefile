build:
	go build -o dist/block-chain cmd/block/main.go
	go build -o dist/node cmd/server/main.go
	go build -o dist/tx cmd/tx/main.go
