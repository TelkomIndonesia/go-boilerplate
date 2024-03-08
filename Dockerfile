# syntax = docker/dockerfile:1.2

FROM golang:1.21 AS builder

WORKDIR /src
COPY ./ ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o profile ./cmd 



FROM alpine AS alpine

RUN apk add --no-cache ca-certificates



FROM scratch 

COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /src/profile /usr/local/bin/profile

ENTRYPOINT ["/usr/local/bin/profile"]