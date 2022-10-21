FROM golang:1.19.1-alpine as builder
RUN apk add --no-cache git make
ENV GOOS=linux
ENV CGO_ENABLED=0
ENV GO111MODULE=on
COPY . /src
WORKDIR /src
RUN rm -f go.sum
RUN go get ./...
#No test files in contests
#RUN go test ./...
RUN make release

FROM gcr.io/distroless/base
WORKDIR /app
COPY --from=builder /src/bin/contests /app/contests
ENTRYPOINT ["/app/contests"]
