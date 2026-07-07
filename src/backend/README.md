# Backend

Backend services live under this tree as separate projects. `gateway/` is the implemented API Gateway; the other service directories are initialized scaffolds so CI, Docker builds, and future feature work have stable project homes.

- `auth/` is a Java/Spring Boot scaffold with `GET /health`, a smoke test, and Dockerfile.
- `gateway/` is the Go API Gateway with REST/WS edge behavior, tests, and Dockerfile.
- All other backend microservices from [docs/MICROSERVICES.md](../../docs/MICROSERVICES.md) are Go scaffolds with `GET /health`, smoke tests, individual `go.mod`, and Dockerfile.
- Docker build context for `gateway`, `realtime`, `chat`, `messaging`, `user`, and `social` is `src/backend` (canonical list: [`scripts/ci/backend-docker-context.txt`](../../scripts/ci/backend-docker-context.txt); same in [Makefile](../../Makefile) `GO_SERVICES_BACKEND_CONTEXT` and CI docker build). Other Go services use `src/backend/<service>`.

From the repository root, **`make build-all`** mirrors backend CI on the host: `check-toolchain` (Go 1.26, Docker daemon, Maven/Java), compose + buf in Docker, then host `go test`, `golangci-lint`, Gateway race tests, Auth `mvn test`, local images `voice-<service>:local`, and `testcontainers-prune`. Integration tests need Docker on the host (see [`pkg/integrationtest/`](pkg/integrationtest/) and [docs/TESTING.md](../../docs/TESTING.md)). Flutter is separate: **`make flutter-ci`**.

SQL migrations for v1 core DM scope databases ([docs/DATA_SCOPE_V1.md](../../docs/DATA_SCOPE_V1.md)) are currently in [`migrations/`](migrations/). Ownership and tools: [docs/OPERATIONS.md](../../docs/OPERATIONS.md) (Flyway for Auth Java, golang-migrate for Go).
