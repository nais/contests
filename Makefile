DATE=$(shell date "+%Y-%m-%d")
LAST_COMMIT=$(shell git --no-pager log -1 --pretty=%h)
VERSION="$(DATE)-$(LAST_COMMIT)"
LDFLAGS := -X github.com/nais/contests/pkg/version.Revision=$(shell git rev-parse --short HEAD) -X github.com/nais/contests/pkg/version.Version=$(VERSION)

all:
	go build -o bin/contests cmd/contests/main.go

release:
	go build -a -installsuffix cgo -o bin/contests -ldflags "-s $(LDFLAGS)" cmd/contests/main.go

local:
	go run cmd/contests/main.go --bind-address=127.0.0.1:8080
