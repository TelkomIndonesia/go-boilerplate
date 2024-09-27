# Go Boilerplate

![Package Dependency](./diagram.svg)

Features:

- [x] Database (postgres) with Encryption at Rest (tink)
  - [x] Derivable encryption key.
  - [x] Rotatable encription key.
  - [x] Blind index as bloom filter for exact match.
  - [x] Outbox pattern (kafka + cloudevent + protobuf).
  - [x] Query-to-code generator (SQLC).
- [x] HTTP API
  - [x] OpenAPI-to-code generator (oapi-codegen).
  - [x] Auto Load CA & Leaf TLS certificate.
  - [x] mTLS support.
- [x] Opentelemetry (console, otlphttp, otlpgrpc, and datadog trace provider).
  - [x] Code Generator for auto instrumentation (otelwrap)
- [x] Plugable log (zap, testing).
  - [x] Embed opentelemetry trace-id + copy logged field to opentelemetry trace.
- [x] Env config.
- [x] Dockerized.
- [x] CI/CD as Code (dagger)

## Using as library

The packages under `pkg` are reusable for importing into other project. Moreover `pkg/cmd` can be used to instantiate all the packages using environment variable for [quick inclusion](./internal/cmd/cmd.go#L118-L136)

### Versions with BREAKING CHANGES

- v0.20.0 introduces major breaking changes as the package structure is completely rewritten
