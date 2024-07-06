# Go Boilerplate

![Package Dependency](./diagram.svg)

Features:

- [x] Database (postgres) with Encryption at Rest (tink)
  - [x] Derivable encryption key.
  - [x] Rotatable encription key.
  - [x] Blind index as bloom filter for exact match.
  - [x] Outbox pattern (kafka).
  - [x] Query-to-code generator (SQLC).
- [x] HTTP API
  - [x] OpenAPI-to-code generator (oapi-codegen).
  - [x] Auto Load CA & Leaf TLS certificate.
  - [x] mTLS support.
- [x] Opentelemetry (console, otlphttp, and datadog trace provider).
- [x] Plugable log (zap).
- [x] Env config.
- [x] Dockerized.
- [x] CI/CD as Code (dagger)

## Using as library

The packages under `pkg/util` are reusable for importing into other project. Moreover `pkg/util/cmd` can be used to instantiate all the packages using environment variable for [quick inclusion](./pkg/cmd/cmd.go#L117-L135)

test
