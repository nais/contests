FROM golang:1.27.4-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags="-s -w" -o /contests ./cmd/contests

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /contests /contests
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/contests"]
