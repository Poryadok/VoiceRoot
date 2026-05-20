# Backend

Backend services live under this tree as separate projects. `gateway/` is the implemented API Gateway; the other service directories are initialized scaffolds so CI, Docker builds, and future feature work have stable project homes.

- `auth/` is a Java/Spring Boot scaffold with `GET /health`, a smoke test, and Dockerfile.
- `gateway/` is the Go API Gateway with REST/WS edge behavior, tests, and Dockerfile.
- All other backend microservices from [docs/MICROSERVICES.md](../../docs/MICROSERVICES.md) are Go scaffolds with `GET /health`, smoke tests, individual `go.mod`, and Dockerfile.
- Docker build context for `gateway`, `realtime`, `chat`, `messaging`, `user`, and `social` is `src/backend` (see `scripts/ci/backend-docker-context.txt`); other Go services use `src/backend/<service>`.

Run the full backend-local CI path from the repository root with `make build-all`. It validates compose and protobufs, runs Go tests for all Go services and `pkg`, runs golangci-lint in each module, runs `go test -race` for Gateway, runs Auth Maven tests, and builds local Docker images named `voice-<service>:local`.

SQL migrations for Phase 0–1 databases ([docs/DATA_SCOPE_V1.md](../../docs/DATA_SCOPE_V1.md)) are currently in [`migrations/`](migrations/). Ownership and tools: [docs/OPERATIONS.md](../../docs/OPERATIONS.md) (Flyway for Auth Java, golang-migrate for Go).
