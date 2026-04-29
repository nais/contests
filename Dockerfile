FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git
COPY . /src
WORKDIR /src
RUN rm -f go.sum
RUN go get ./...
RUN GOOS=linux GOARCH=amd64 go build -o bin/contests cmd/contests/main.go

FROM gcr.io/distroless/base
WORKDIR /app
COPY --from=builder /src/bin/contests /app/contests
ENTRYPOINT ["/app/contests"]
