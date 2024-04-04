binary: check fmt
	go build -o bin/contests cmd/contests/main.go

linux-binary:
	GOOS=linux GOARCH=amd64	go build -o bin/contests cmd/contests/main.go

local:
	go run cmd/contests/main.go --bind-address=127.0.0.1:8080

check: staticcheck vulncheck deadcode

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

deadcode:
	go run golang.org/x/tools/cmd/deadcode@latest -test ./...

fmt:
	go run mvdan.cc/gofumpt@latest -w ./

helm-lint:
	helm lint --strict ./charts
