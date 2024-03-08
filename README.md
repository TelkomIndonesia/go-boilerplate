# Go Boilerplate

![Package Dependency](./diagram.svg)

Features:

- [x] Database (postgres) with Encryption at Rest (tink)
  - Per-tenant encryption key
  - Rotatable encription key
  - Blind index for exact match
- [x] CI/CD as Code (dagger)
- [x] Dockerized
- [x] Env Config
- [x] Auto Load TLS certificate for HTTPS Server
- [x] Auto Load CA certificate for HTTPS Client
- [x] Opentelemetry (console, otlphttp, and datadog trace provider)
- [x] Plugable log (zap)
