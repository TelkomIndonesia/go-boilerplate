# Go Boilerplate

![Package Dependency](./diagram.svg)

Features:

- [x] Database (postgres) with Encryption at Rest (tink)
  - [x] Derivable encryption key (per tenant)
  - [x] Rotatable encription key
  - [x] Blind index as bloom filter for exact match
  - [x] Outbox pattern (kafka)
- [x] CI/CD as Code (dagger).
- [x] Dockerized.
- [x] Env Config.
- [x] Auto Load CA & Leaf HTTP TLS certificate.
- [x] mTLS support.
- [x] Opentelemetry (console, otlphttp, and datadog trace provider).
- [x] Plugable log (zap).
