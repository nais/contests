binary:
	go build -o bin/contests cmd/contests/main.go

linux-binary:
	GOOS=linux GOARCH=amd64	go build -o bin/contests cmd/contests/main.go

local:
	go run cmd/contests/main.go --bind-address=127.0.0.1:8080
