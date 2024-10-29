FROM golang:1.23 AS builder

COPY --from=docker:cli /usr/local/bin/docker /usr/local/bin/docker

WORKDIR /src
COPY ./ ./

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -o dagger .

RUN mv dagger /usr/local/bin/dagger

ENTRYPOINT [ "dagger" ]

