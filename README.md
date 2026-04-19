# Voice

Discord-like messenger with voice and matchmaking. Product and architecture live in [`docs/`](docs/).

## Layout

| Path | Purpose |
|------|---------|
| [`docs/`](docs/) | Specifications, glossary, phased plan |
| [`protos/`](protos/) | gRPC / Protobuf (`buf.work.yaml` + [`protos/buf.yaml`](protos/buf.yaml); S2S: [`protos/voice/s2s/v1/s2s.proto`](protos/voice/s2s/v1/s2s.proto)) |
| [`src/frontend/`](src/frontend/) | Flutter client (placeholder until app lands here) |
| [`src/backend/`](src/backend/) | Go services, gateway, shared libs; SQL migrations under `migrations/` |
| [`src/admin/`](src/admin/) | Admin web (placeholder) |

Monorepo conventions: [docs/REPOSITORIES.md](docs/REPOSITORIES.md). Roadmap: [docs/PLAN.md](docs/PLAN.md).

## Local stack

Prerequisites: Docker with Compose v2.

```text
copy .env.example .env
docker compose up -d
```

PostgreSQL exposes five logical databases (`auth_db`, `user_db`, `social_db`, `chat_db`, `messaging_db`) plus Redis. Apply SQL migrations with your chosen tool (Flyway for Auth Java module, [golang-migrate](https://github.com/golang-migrate/migrate) for Go services); files live in [`src/backend/migrations/`](src/backend/migrations/).

## Checks

See [docs/TESTING.md](docs/TESTING.md). From repo root:

```text
make buf-lint
```

Full CI parity: GitHub Actions on pull requests to `master` (protobuf lint + breaking check; docs link check when `docs/` changes).
