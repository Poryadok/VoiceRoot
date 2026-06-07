# Voice

Discord-like messenger with voice and matchmaking. Product and architecture live in [`docs/`](docs/).

## Layout

| Path | Purpose |
|------|---------|
| [`docs/`](docs/) | Specifications, glossary, phased plan |
| [`protos/`](protos/) | gRPC / Protobuf (`buf.work.yaml` + [`protos/buf.yaml`](protos/buf.yaml); codegen: [`buf.gen.yaml`](buf.gen.yaml) → `make buf-generate`; S2S: [`protos/voice/s2s/v1/s2s.proto`](protos/voice/s2s/v1/s2s.proto)) |
| [`src/frontend/`](src/frontend/) | Flutter client (Phase 1 in progress; see `src/frontend/README.md`) |
| [`src/backend/`](src/backend/) | Go services, gateway, shared libs; SQL migrations under `migrations/` |
| [`src/admin/`](src/admin/) | Admin web (placeholder) |

Monorepo conventions: [docs/REPOSITORIES.md](docs/REPOSITORIES.md). Roadmap: [docs/PLAN.md](docs/PLAN.md).

## Toolchain (reference)

Versions below match the primary dev machine (Windows); bump this table when you intentionally upgrade. CI agents may differ—if something breaks only locally or only in CI, compare versions first.

| Tool | Version |
|------|---------|
| Go | 1.26.2 |
| Flutter | 3.41.7 (stable) |
| Dart | 3.11.5 |
| Java (Auth service) | 21.0.5 |
| Docker | 27.4.0 |
| Docker Compose | v2.31.0 |
| [buf](https://buf.build/) | 1.50.0 |

## Local stack

Prerequisites: Docker with Compose v2 (see **Toolchain**).

```text
copy .env.example .env
docker compose up -d
```

PostgreSQL exposes five logical databases (`auth_db`, `user_db`, `social_db`, `chat_db`, `messaging_db`) plus Redis and NATS. Init scripts under [`docker/postgres/`](docker/postgres/) apply the first-wave SQL from [`src/backend/migrations/`](src/backend/migrations/) on a **new** volume. Without `--profile app`, compose starts **infra only** (Postgres, Redis, NATS) — no app containers.

### Phase 1 full stack (`--profile app`)

**`make compose-app-up`** (same as `docker compose --profile app up -d --build`) starts the local **Phase 1** stand from [docs/PLAN.md](docs/PLAN.md) § «Фаза 1 — MVP: личные сообщения»:

| Service | Role |
|---------|------|
| `auth` | Register / login / JWT / JWKS |
| `user` | Profiles, user search |
| `social` | Friends |
| `chat` | DM chats |
| `messaging` | Send / history (REST) |
| `realtime` | WebSocket fan-out (`/ws` via gateway) |
| `gateway` | REST + WS edge (gRPC transcoding to services above) |
| `web` | Flutter web shell |

Recommended host ports in `.env` (avoids clashing with other apps on `:8080`):

```text
GATEWAY_PORT=18080
VOICE_API_PUBLIC_URL=http://127.0.0.1:18080
WEB_PORT=9080
```

**Media (avatars, attachments):** with `make compose-app-up`, **MinIO** (local S3-compatible storage) starts automatically when `*_R2_*` variables are set in `.env` (see [`.env.example`](.env.example)). Without them, text DM and realtime chat still work; avatar presign and file upload return `FailedPrecondition`. Staging and production use **Cloudflare R2**, not MinIO.

Then:

| What | URL |
|------|-----|
| Web UI | **`http://127.0.0.1:9080`** |
| API Gateway (REST + WS) | **`http://127.0.0.1:18080`** (`ws://127.0.0.1:18080/ws`) |

The web image bakes `VOICE_API_PUBLIC_URL` at build time; rebuild after changing it (`make compose-app-up`). Stop: `make compose-down` or `docker compose --profile app down`.

Live Flutter / gateway integration tests use the same gateway base URL — see [`src/frontend/integration_test/README.md`](src/frontend/integration_test/README.md).

## Checks

See [docs/TESTING.md](docs/TESTING.md). From repo root (Docker must be running):

```text
make build-all
```

Собирает образ **`voice-gateway:local`** и прогоняет buf + compose config + тесты gateway в контейнерах (эквивалент основным job’ам [`.github/workflows/ci.yml`](.github/workflows/ci.yml), когда CI уже запускается на GitHub). При установленном **buf** на хосте: `make buf-lint`.

Когда репозиторий подключён к **GitHub Actions**, на PR в `master` дополнительно гоняются те же проверки на раннере плюс, при настроенном репозитории, проверка ссылок в `docs/`; выкат на staging — по [DEPLOYMENT.md](docs/DEPLOYMENT.md), отдельно от `make build-all`. Публичный FQDN gateway на текущем стенде: **`voice.tastytest.online`** (переменная `VOICE_GATEWAY_INGRESS_HOST`, DNS и Cloudflare Flexible — в `DEPLOYMENT.md`).