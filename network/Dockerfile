FROM golang:1.24-alpine AS build

WORKDIR /src
COPY go.mod proxy.go proxy_test.go ./
RUN go test ./... \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/koja-proxy proxy.go

FROM alpine:3.22

COPY --from=build /out/koja-proxy /usr/local/bin/koja-proxy

ENTRYPOINT ["/usr/local/bin/koja-proxy"]
