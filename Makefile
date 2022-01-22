build:
	go build -o bin/main server.go

run:
	go run server.go

test:
	go test ./...

gen:
	go generate ./...