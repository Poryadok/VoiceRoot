# Matchmaking Service

## Обзор

Матчмейкинг для геймеров: каталог игр, очереди поиска, алгоритм подбора, рейтинги. Ключевая отличительная фича Voice.

**Язык**: Go
**БД**: PostgreSQL `matchmaking_db`, Redis (очереди)

## Ответственность

- Каталог игр (JSON-документы с режимами, ролями, рангами)
- Глобальный матчмейкинг (v1 — без верификации)
- Space-level матчмейкинг (future — кастомные правила пространства)
- Соло и групповые заявки (party)
- Точные критерии: игра, режим, роль, ранг, регион
- При `roles_required` роль — обязательная self-reported метка из каталога;
  повтор роли между участниками разрешён, без квот и балансировки в V1
- Регион обязателен (нет cross-region)
- Таймауты: 15 мин (default) / 30 мин (expanded)
- Создание матч-отряда при матче
- Рейтинг после завершения (1-5 звёзд)
- Бан из ММ (не затрагивает мессенджер)
- Интеграция со Stories ("ищу пати")
- Нет одновременных поисков (1 активный на профиль)

## API (gRPC)

```protobuf
service MatchmakingService {
  // Каталог игр
  rpc ListGames(ListGamesRequest) returns (GameList);
  rpc GetGame(GetGameRequest) returns (Game);
  rpc CreateGame(CreateGameRequest) returns (Game); // admin/moderator
  rpc UpdateGame(UpdateGameRequest) returns (Game);
  rpc SearchGames(SearchGamesRequest) returns (GameList);

  // Поиск
  rpc StartSearch(StartSearchRequest) returns (SearchSession);
  rpc CancelSearch(CancelSearchRequest) returns (Empty);
  rpc GetSearchStatus(GetSearchStatusRequest) returns (SearchSession);

  // Матч
  rpc GetMatch(GetMatchRequest) returns (Match);
  rpc GetMatchHistory(GetMatchHistoryRequest) returns (MatchList);

  // Рейтинг
  rpc RateMatch(RateMatchRequest) returns (Empty);
  rpc GetPlayerRating(GetPlayerRatingRequest) returns (PlayerRating);

  // MM Ban
  rpc BanFromMM(BanFromMMRequest) returns (Empty); // moderation
  rpc UnbanFromMM(UnbanFromMMRequest) returns (Empty);
  rpc GetMMBanStatus(GetMMBanStatusRequest) returns (MMBanStatus);
}
```

## Модель данных

### Public game catalog ordering

`ListGames` follows [BE164-PUBLIC-POPULARITY-v1](../features/game-catalog.md#public-popularity-and-paging-be164-public-popularity-v1).
One PostgreSQL statement orders all active games by global live searching-session
count, then descending creation time and UUID. It supplies the ordered IDs and
page metadata from the same MVCC snapshot; `CURRENT_TIMESTAMP` is the database
cutoff. Space rows, non-searching state and null/expired deadlines never rank.
The implementation hashes only canonical ordered UUID bytes with a versioned
public-domain separator; no Redis scan, private counts or snapshot cache.

New bounded cursors carry a version, order fingerprint and next ordinal. A
changed order/membership yields `games_cursor_stale`/`FailedPrecondition` with no
page; malformed tokens yield `InvalidArgument`. Valid historical timestamp/UUID
chains continue active-only chronological paging. Staff `ListGameRequests`,
including its active filter, keeps its separate chronological store entry point;
`SearchGames` is unchanged. Query cost includes the full active-game sequence
and eligible global searching rows, rather than popularity-sorting a truncated
chronological page. Real PostgreSQL integration records the query plan; this is
not a retained-snapshot or stable-through-arbitrary-churn promise.

```
games
├── id (UUID)
├── name
├── icon_url
├── external_id (nullable — для авто-дедупликации)
├── config (jsonb) — {
│     modes: [{ name, slots, roles: [{name, required}], ranks: [{name, value}] }],
│     regions: ["eu", "na", "cis", ...]
│   }
├── status (active | archived)
├── created_by (profile_id)
├── created_at
└── updated_at

search_sessions
├── id (UUID)
├── profile_id
├── party_id (nullable — для групповых)
├── game_id (FK)
├── mode
├── criteria (jsonb — role, rank, region)
├── status (searching | matched | timeout | cancelled)
├── timeout_at
├── matched_at (nullable)
├── match_id (nullable, FK)
├── created_at
└── updated_at

parties
├── id (UUID)
├── leader_profile_id
├── member_profile_ids (jsonb)
├── game_id (FK)
├── mode
├── criteria (jsonb)
├── created_at
└── disbanded_at (nullable)

matches
├── id (UUID)
├── game_id (FK)
├── mode
├── region
├── participants (jsonb — [{profile_id, role, rank}])
├── voice_room_id (nullable — voice room матча / матч-отряда)
├── chat_id (nullable — temp chat)
├── status (active | completed | abandoned)
├── created_at
└── completed_at (nullable)

match_ratings
├── match_id (FK)
├── rater_profile_id
├── rated_profile_id
├── score (1-5)
├── created_at
└── UNIQUE(match_id, rater_profile_id, rated_profile_id)

player_ratings
├── profile_id (FK)
├── game_id (FK)
├── average_rating (float)
├── total_matches (int)
├── total_ratings_received (int)
├── created_at
└── updated_at

mm_bans
├── profile_id (FK)
├── reason
├── banned_by (profile_id)
├── expires_at (nullable — null = permanent)
├── created_at
└── revoked_at (nullable)

profile_game_entries
├── profile_id (UUID)
├── game_id (FK → games)
├── region (TEXT, required)
├── role (TEXT, optional)
├── rank (TEXT, optional)
├── updated_at
└── PRIMARY KEY (profile_id, game_id)
```

`match_ratings` is the authoritative per-match vote ledger. `player_ratings` is
the per-profile/game aggregate projection and is recomputed from those votes in
the same transaction as each submission. The unique vote key makes retries
idempotent; retrying a previously stored vote repairs the projection without
counting that vote twice. Migration `000015_reconcile_player_rating_aggregates`
repairs existing projections from the retained vote ledger.

## Алгоритм матчинга

```
Redis Queue per (game_id, mode, region):
  sorted by created_at (FIFO)

Matcher Worker (горизонтально масштабируемый):
1. Poll очередь
2. Для каждой заявки:
   a. Найти совместимые заявки (exact match по criteria)
   b. Если набралось slots — создать Match
   c. Создать voice room матч-отряда (Voice Service)
   d. Создать temp chat (Chat Service)
   e. Уведомить участников (NATS → Notification)
3. Проверить таймауты (15/30 мин)
```

`roles_required` валидирует присутствие роли и её принадлежность `modes[].roles`.
Поле `roles[].required` и контракт каталога сохраняются, но не задают уникальные
слоты: V1 допускает одинаковые и разные валидные роли в одном предложении.
Балансировка или role quotas требуют отдельного контракта и не выводятся из
текущей конфигурации.

## Публикуемые события (→ NATS)

Доменный поток JetStream: **`matchmaking.events`** ([CONTRACT_MATRIX.md](../CONTRACT_MATRIX.md)).

| Событие               | Данные                                     |
|-----------------------|--------------------------------------------|
| `mm.search_started`   | session_id, profile_id, game, mode, region |
| `mm.search_cancelled` | session_id, profile_id                     |
| `mm.search_timeout`   | session_id, profile_id                     |
| `mm.match_found`      | match_id, participants, game, room_id      |
| `mm.match_completed`  | match_id, duration                         |
| `mm.rating_submitted` | match_id, rater_id, rated_id, score        |
| `mm.player_banned`    | profile_id, reason                         |

## Зависимости

- **Redis** — очереди поиска, active session lock
- **Voice Service** — создание voice room для матч-отряда при матче
- **Chat Service** — создание временного чата при матче
- **Notification Service** — (через NATS) уведомление о найденном матче
- **Story Service** — (через NATS) "ищу пати" → автоматическая заявка
- **Moderation Service** — проверка ММ-банов

### Rating privacy S2S

Стандартные Compose и cluster deployment обязаны задавать `USER_GRPC_ADDR`,
`SOCIAL_GRPC_ADDR` и `SPACE_GRPC_ADDR`: вместе они применяют аудиторию
`show_mm_rating` для друзей, друзей друзей и участников Space. Если
`USER_GRPC_ADDR` отсутствует в минимальной standalone-конфигурации,
Matchmaking сохраняет degraded passthrough для чтения рейтинга и не пытается
подменить его новой privacy-политикой; `SOCIAL_GRPC_ADDR` и
`SPACE_GRPC_ADDR` используются только после подключения User privacy client.

## P3 Space lifecycle participant (target)

Matchmaking stores a durable per-Space generation fence plus immutable operation
and receipt evidence. `FROZEN` cancels active searches tied to the Space and
denies create/join/match/result mutations before cached membership shortcuts.
`LIVE` requires the next generation. `PURGE_DECIDED` permanently suppresses
delayed work and removes Space queues, sessions, candidates, projections and
Redis state. The receipt binds the root manifest but owns no Chat page set. Full
request/receipt bytes retain 30 days from this participant's completion;
compact `PURGED` fence is permanent.

## Protected Match completion

`CompleteMatch` is exposed through Gateway at
`POST /api/v1/matchmaking/matches/{match_id}/complete`. Gateway calls the
dedicated Matchmaking principal listener on `:9092` with a short-lived,
request-bound `gateway` principal over mTLS. The verifier accepts only the
configured issuer set and exact Matchmaking method; Space methods remain denied
when no Space issuer is configured. The protected runtime also requires the
current Auth session-epoch floor and consumes replay IDs in the configured
Redis store. The ordinary `:9090` listener does not authorize this RPC.

The principal binds the verified account, profile, session epoch, exact RPC,
request hash and operation ID. Matchmaking checks participant membership and
stores completion idempotency by `(actor_profile_id, operation_id)`: an exact
same-actor replay returns the durable outcome, while a changed request under
that key conflicts. Provider teardown and aggregate completion remain durable
separate lifecycle steps; a successful HTTP response is not evidence that
external Chat or Voice cleanup has completed.

The final participant leave also advances each original matched search session's
recovery generation and commits an exact-session queue-release obligation in the
same transaction. Matched session rows and their history links remain intact;
they are not cancelled through `CancelSearch`. A separate pending-only terminal
outbox, invisible to legacy recovery consumers, captures the immutable routing.
Its worker locks only the outbox row and retries until Redis retires the FIFO and
only the profile lock still owned by that session. A delayed release cannot
delete a newer search reservation, and an older enqueue generation cannot
resurrect the retired search. Successful Redis acknowledgement deletes the entire
outbox row, including its personal routing. Parent purge or binary rollback must
not cascade or acknowledge pending obligations; an absent parent does not prevent
their exact-owner projection. Migration 000019 is additive, precedes the new
writer, and refuses DOWN while pending rows or concurrent writers exist. Binary
rollback keeps the schema and pending rows for re-upgrade. Completions performed
entirely by an old producer do not create new terminal intents; this forward
producer does not backfill them. Queue convergence and provider teardown are
independent cleanup steps; after they complete, both profiles may search again
and their persisted peer bans still apply.

Pending means cleanup is incomplete, including during Redis outage after parent
purge. It is not a personal-data retention exception. Canonical `P30D` erasure or
irreversible pseudonymization remains required independently of Redis ACK.
The exact field inventory and required owned A4 deletion-integration gap are
recorded in [DATA_STORES.md](../DATA_STORES.md#matchmaking-terminal-search-release-outbox).
This A3 slice does not establish A4/privacy or final acceptance.

The A4 local terminal-routing consumer accepts already-verified immutable Auth
schedule/decision/current facts and complete User coverage; it allocates no new
RPC, subject or producer authority. It uses the original `purge_at`, database
time after serialization, and a Redis-independent expiry pass before ordinary
session/queue work. A delayed restore committed strictly before that deadline
may permit fresh admission only with verified current ACTIVE authority and
current profile eligibility. It never reconstructs erased routing. An old cycle
cannot cancel a newer current deletion, and restored backups cannot serve from
stale local authority alone. Protected producer adapters, writer binding and
backup/rollback activation are still required before claiming A4 integration.
Coverage intake refuses ambiguous unbound legacy operations. It never infers
cycle membership from MM creation time versus Auth decision time, or upgrades a
previously escaped unbounded projection by editing its SQL row. Only operations
with exact lifecycle-aware writer binding enter the bounded local scope; an
empty SQL result does not prove historical Redis cleanup.

The recovery worker claims each incomplete provider operation independently and
replays only its persisted request. Every RPC has a fresh 10-second deadline;
transient failures persist a retry time using 1-second exponential backoff,
bounded jitter and a 5-minute cap. Durable provider receipts reset the aggregate
progress clock. Prometheus reports the oldest pending progress age and terminal
contract-mismatch count: no progress for 15 minutes alerts, while a contract
mismatch raises an immediate critical alert and is never retried automatically.

### MatchSquad principal JWKS publisher

When MatchSquad principal verification is enabled, Matchmaking also publishes
the public portion of its current and next RSA signing keys at
`https://<matchmaking-service>:8443/internal/v1/principal/jwks.json`. The
listener uses the existing `principal.JWKSHandlerKeys` representation and
serves only GET over TLS 1.2 or newer. Its server certificate must identify
the configured service DNS name; its private key and the signing-key directory
are mounted read-only. The HTTPS JWKS identity is separate from the mTLS
certificates used by the MatchSquad gRPC listeners.

Configure `MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR`,
`MATCHMAKING_PRINCIPAL_ACTIVE_KID`,
`MATCHMAKING_PRINCIPAL_JWKS_LISTEN_ADDR`,
`MATCHMAKING_PRINCIPAL_JWKS_TLS_CERT_FILE`, and
`MATCHMAKING_PRINCIPAL_JWKS_TLS_KEY_FILE` as one complete profile. If every
setting is absent, the publisher stays disabled; any partial or invalid
profile fails startup. The directory contains the same two distinct RSA
private-key files used by the issuer, so both key IDs remain available during
rotation while only the active ID signs new credentials. Consumers configure
the exact HTTPS URL and a dedicated CA file; they fail closed if TLS trust or
the JWKS document is unavailable. The endpoint is internal and must not be
added to public ingress.

When Space or Gateway principal verification is configured, Matchmaking keeps
this JWKS publisher available while it loads the required initial issuer key
sets. Only source-availability failures are retried with bounded requests and
capped backoff. Matchmaking does not open ordinary health or gRPC listeners
until each configured key set is valid. Invalid TLS trust, redirects, permanent
HTTP errors, malformed key sets, and invalid configuration remain startup
errors; cancellation stops the wait.
