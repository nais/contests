FROM golang:1.20.2-alpine as builder
RUN apk add --no-cache git make
ENV GOOS=linux
ENV CGO_ENABLED=0
ENV GO111MODULE=on
COPY . /src
WORKDIR /src
RUN rm -f go.sum
RUN go get ./...
RUN make release

FROM gcr.io/distroless/base
WORKDIR /app
COPY --from=builder /src/bin/contests /app/contests
ENTRYPOINT ["/app/contests"]
