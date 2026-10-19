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

### A3 Voice roster cancellation

The durable pull consumer `matchmaking_voice_roster_v1` reads only
`voice.roster_membership_changed` from `voice_events`. It ACKs after its local
inbox/reset transaction commits; transient database failure leaves delivery for
retry. Exact replay is a no-op. Conflicting event IDs, conflicting room versions
or malformed bindings retain evidence and raise the critical
`VoiceRosterContractMismatch` alert; they never authorize a partially trusted
roster. Operators must reconcile retained evidence and affected parties before
resuming a failed rollout. No automatic conflict deletion or recovery is claimed.

A newer room version cancels every member of each captured searching or
pending-accept party for that room, even when the changed profile was absent from
the captured manifest. JOINED also cancels that profile's captured SOLO search.
The same transaction abandons affected pending matches, recovers unrelated
parties, advances queue generations, writes RELEASE/ENQUEUE intents, and saves
`mm.search_cancelled` with reason `VOICE_ROSTER_CHANGED` once per cancelled
session. Older versions cannot cancel a newer capture. Frozen cancellation
bytes retry independently of broker availability; clients' historical empty
reason remains compatible.

Protected source revalidation belongs before proposal and before each provider
forward step/final activation, including background recovery. A definitive
source change invokes the same whole-party reset; an unavailable or ambiguous
source stays unknown and permits neither forward creation nor fabricated
cancellation. `BE012-ASYNC-PROVISION-RESET-v1` additionally requires committed
pre-RPC dispatch markers, monotone abandonment and receipt-based cleanup of
possibly created resources. That coordinator/cold-cut activation remains a
separate implementation and deployment gate; an inbox PASS does not satisfy it.

### Preactivation MatchSquad settlement (A3)

Migration `000021_match_squad_dispatch_settlement` records a monotone
`LIVE`/`ABANDONED` disposition, revision and immutable abandonment evidence on
the existing provider operation. Each provider has a permanent first-dispatch
identity/time and `NOT_ISSUED`, `MAY_HAVE_CREATED` or `RECEIPT_KNOWN` state.
Historical frozen requests migrate conservatively as possibly issued; an RPC
error, missing receipt or expired lease never establishes resource absence.

The abandonment transaction freezes cleanup obligations from dispatch evidence.
No dispatch closes locally without synthetic teardown evidence. Otherwise the
required set is `[chat]` or `[chat, voice]`; an unknown creation retains a
`WAITING_FOR_CREATION` slot without executable teardown bytes. Valid creation
receipts materialize the exact preallocated cleanup identity once, including late
receipts after abandonment. Known Chat cleanup proceeds while Voice is unknown.
Issued obligations stay discoverable until creation and cleanup are proven;
abandoned cleanup cannot activate the match or emit `mm.match_completed`.

Receipt-only transactions take match, operation, aggregate and participant locks
in that order. Forward transitions additionally require the complete captured
party/session cut and the outer account guard described below. Schema/storage
proof alone does not enable this coordinator: common request/recovery gates,
real provider replay and actual old-writer retirement remain activation gates.

### Private captured party authority (A3)

The protected Voice snapshot binds each roster profile to its native admission
account and admitted session epoch from the same final C2 cut. The initiating
device epoch may differ from a roster member's admitted epoch; SOLO uses exactly
the verified initiating subject. This private authority is never exposed in
public responses, roster events, logs or metric labels.

Migration `000022_party_member_authority` adds immutable
`voice_member_authority_version`, canonical `voice_member_authority` bytes,
`voice_member_authority_sha256` and opaque `voice_source_authority_sha256` to the
existing captured party. The tuple manifest is version 1 with sorted profile UUIDs
and a bijection to `member_profile_ids`; each account is unique within a party.
The source digest also binds native admission identity and the complete coverage
cut. Fresh validation compares both hashes: a changed valid cut resets the whole
party even when its public profile roster is unchanged. Missing, unsupported or
corrupt authority is unavailable; historical NULL captures require a new search,
without backfilling ownership from User metadata. Rollback refuses retained
private evidence.

All-member platform MM ban checks use the stored native account bindings.
`AcquirePlatformBanGuard` acquires shared transaction advisory locks in sorted
actual-key order before match, party, session and proposal/operation row locks.
Sanction Apply/Revoke acquire the same account locks exclusively before writing.
The transaction must use READ COMMITTED; the authoritative ban SELECT is a
separate statement after all locks, using the database clock for expiry. A CTE
that locks and reads in one statement is insufficient after waiting for a writer.
The same guard is required at enqueue, reservation, each forward provisioning
boundary and final activation. This preserves the accepted settlement path for
already-issued operations after a later ban or roster reset.

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

The private party platform-ban guard returns `PlatformBanDenial` for a known
active ban. Its neutral error text remains `party matchmaking unavailable`;
the defensive `Accounts()` accessor is local evidence for the coordinator to
identify affected parties from their verified private member tuples. Account
identifiers must not enter public errors, events, responses or log fields.
The query runs as a fresh READ COMMITTED statement after account and inner
transition locks; unavailable authority supplies no cancellation evidence.

The durable party queue projector leases revisioned PostgreSQL intents and
copies an entire eligible captured party to Redis in one atomic recovery
operation. Redis remains a recoverable projection rather than party authority.
Retries after a lost acknowledgement preserve session identities and recovery
generations. A release wins over an enqueue of the same generation; a late
worker cannot acknowledge a replaced intent or clear a successor lease.

Before acknowledging a successful Redis operation, the projector locks the
party, its sessions and the intent, then reads a fresh PostgreSQL statement to
compare the complete captured projection cut. A session transition that did
not increment the intent revision still invalidates that acknowledgement.
Invalid or disbanded parties release every retained session using its exact
generation; cleanup does not require retaining private member authority.
The component currently accepts qualified native source captures. LFP consent
requires its separate typed receipt adapter, and runtime startup remains part
of the complete source activation package.

Proposal reservation preserves each captured party as one unit. The store locks
all referenced parties in UUID order before locking search sessions, then checks
the complete immutable profile roster, current party status and session scope.
A partial queue projection, committed disband, expired session or scope mismatch
must create no match and advance no session generation. Database time is sampled
after waiting for these locks. These local checks complement the protected fresh
source revalidation and account-wide ban guard required by the full A3 path.
### Complete-party queue selection

The matcher groups queued sessions by their persisted MM party before choosing
participants. A party is selectable only when every recorded member has one
queued searching session; a missing member never becomes a smaller party.
Bundles retain the queue position of their first member. A bundle that does
not fit the remaining slots is skipped as a whole, and compatibility and peer
bans apply to every pair in the proposed group. PostgreSQL repeats the complete
membership and live session checks under the reservation locks.

This local grouping does not establish source authority for historical rows.
Native proposal authorization additionally requires fresh protected source
revalidation and the account ban guard; LFP qualification uses its separate
consent source contract.

Preproposal capture uses a distinct `ProposalCandidate`, without synthesizing
a match ID before reservation. It fingerprints the complete private parties,
all related session rows and canonical proposal parameters in one read-only
repeatable-read cut, then releases the transaction before protected source RPCs.
Its accounts come only from validated persisted private member authority.
`ProposalValidator` yields an immediate proof for that exact local cut or the
exact definitively changed party IDs. Any Unknown result discards a partial
change set. `CreateValidatedProposal` obtains sorted shared account ban locks
before complete party/session row locks in a short READ COMMITTED transaction,
then starts separate fresh ban and local cut reads before writing the match,
proposals and next session generations. The public legacy `CreateProposal`
refuses typed source parties; missing or mismatched immediate proof cannot
select that path. Its schema-independent authority-field check preserves the
actual pre-native migrations 001–018 path without treating historical rows as
Voice/LFP source authority.

When configured with `ProposalValidator`, the matcher Worker captures and
validates a new candidate on each queue pass before calling the locked API.
Unknown, malformed change sets and local changes after the source RPC leave
parties searching and create no match. Definitive source changes reset only
the affected captured whole parties. A typed local account ban is intersected
with their verified private member tuples and resets only those parties;
generic denial errors never authorize cancellation. The durable RELEASE intent
removes the affected sessions from Redis without removing unrelated parties.
Errors returned to the Worker logger contain no private source/account data.

An absent validator retains only the public pre-cut legacy reservation path,
whose storage guard refuses typed sources. Native startup must install the
actual protected validator and qualify the complete source package; this Worker
component does not install that authority or qualify LFP historical rows.

`StartSearch` consumes only the dedicated verified subject bound to its exact
request bytes. Legacy forwarded identity metadata cannot authorize this handler.
The protected Voice snapshot supplies the complete private party; criteria and
mode limits use its actual member count. One PostgreSQL transaction commits the
immutable capture, every member session and its durable queue intent, including
the fresh account-wide ban check. The response contains the initiating profile's
session. Retrying the same verified operation returns the original sessions;
another active search, changed capture or operation conflict cannot replace them.
Redis projection follows the committed intent, so request retries do not create
another party or recapture unrelated searches. Missing dependencies and unknown
source authority create no singleton fallback. Installing the protected snapshot
client, projector and native lifecycle runtime in the executable remains part of
the complete activation package. `StartSpaceQueue` retains its separate legacy
implementation until its source adapter is explicitly qualified.

The dedicated snapshot client factory accepts a complete profile:
`MATCHMAKING_VOICE_PARTY_SNAPSHOT_GRPC_ADDR`,
`MATCHMAKING_VOICE_PARTY_SNAPSHOT_CA_FILE`,
`MATCHMAKING_VOICE_PARTY_SNAPSHOT_CLIENT_CERT_FILE`,
`MATCHMAKING_VOICE_PARTY_SNAPSHOT_CLIENT_KEY_FILE` and
`MATCHMAKING_VOICE_PARTY_SNAPSHOT_SERVER_NAME`, with the existing two-key
`MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR` and active key ID. It requires mutual
TLS and signs each snapshot request with a fresh request-bound Matchmaking
service principal; forwarded caller metadata is replaced. Shared provider
signing keys alone do not implicitly configure this new client. A partial
profile fails construction, and a server-name mismatch or a certificate that
cannot authenticate a client fails before the application handler. Constructing
this client does not install the complete native serving cohort or authorize
activation; native main wiring must require the independently qualified package.

The MM native serving contract uses a separate fixed executable/image and its
own protected installation binding; Voice startup and Story restore bindings do
not substitute for it. The required native route set denies `StartSpaceQueue` and
profile-only `DecideLfpRequest` until genuine typed adapters are qualified. The
existing pre-cut executable remains a separately admitted artifact.

`NewNativeRouteHandler` binds that policy in the handler itself, so direct
invocation and ordinary gRPC registration cannot reopen the current
`StartSpaceQueue` or `DecideLfpRequest` implementations. Denial precedes request
parsing, membership lookup and all mutation, including the unqualified DECLINE
path. It does not mint a protected `StartSearch` subject, installation admission
or readiness. The separate fixed native root must use this constructor and
validate its complete required dependencies before exposing the service.

Migration 024 adds an installer-owned local admission head. An incomplete
CLOSED/PREPARED marker is sufficient to prevent legacy fallback; only a complete
ADMITTED binding can pass the cooperating transition guard. That guard takes one
stable database-local shared advisory transaction lock **before** account and
party/session locks, then reads the exact head in fresh READ COMMITTED statements.
The real installation closer takes the matching exclusive gate before changing
the head, waits admitted transactions through commit and retains the marker.
Source-party creation and proposal reservation consume their store's immutable
`NativeAdmission` binding; an unbound store refuses any begun head. The safe DOWN
refuses to discard a retained marker. Application guard APIs never issue a
receipt or mutate admission state. The remaining native writers and executable
must use the same gate before native activation.

The DOWN migration takes that exclusive admission gate and an ACCESS EXCLUSIVE
table lock before checking the head. A marker committed by a concurrent writer
while DOWN waits must be observed and retained, including writers that did not
take the cooperating advisory gate.

This local serialization component does not prove external MM installation
custody, physical database continuity, role privileges, predecessor retirement
or restore reconciliation. Those remain required independently, including the
complete route/factory package and exact real installation readback. An empty
head, matching fixture row or transport profile cannot authorize native serving.

Legacy `PartyStore.Create` and `SessionStore.Create` remain pre-cut-only paths.
They hold the local admission gate through commit with no native binding, so
any retained CLOSED/PREPARED/ADMITTED marker refuses their inserts. A native
binding on the party store never qualifies its profile-only legacy `Create`;
qualified source creation uses the separate atomic captured-party API.

The legacy `SessionStore.ResetToSearching` API likewise requires never-begun
admission and holds the gate through its recovery-generation commit. It cannot
revive an old session after local close. Terminal cancellation and expiry keep
their independent cleanup scope while fresh admission is closed.

The compatibility executable checks the fixed native descriptor path before
opening listeners, even when `DATABASE_URL` is absent. A present descriptor or an
unreadable presence check refuses legacy bootstrap. With a database configured,
it checks the admission head before its JWKS bootstrap: every begun state refuses
legacy serving. An absent table or never-begun empty head retains pre-cut behavior
only; this check cannot prove external artifact or credential admission.

Roster runtime preparation binds and validates the already provisioned durable
without fetching, acknowledging, resetting sessions or dispatching cancellation
outbox entries. Its explicit start belongs after the native root's complete
admission/dependency checks. Closing a prepared or started runtime is terminal;
it cannot be restarted to reopen processing. This lifecycle supplies no native
installation or publisher authority of its own.
### MM admission history and source origins

Migration `000025_mm_source_origin` retains installer-issued canonical Armed
receipt bytes, continuity records and private party origins. Receipt hashes use
`mmadmissionwire.ArmedHash`, not the serialized JSON hash; the receipt carries a
zero admission-receipt placeholder and `ReceiptBinding` supplies its final hash.
The first history root is `HistoryGenesisHash(receipt, lineage)`; each successor
root is the canonical `ContinuityHash` over the previous root and exact receipt
pair. Head lineage/root columns can remain NULL on historical markers: this is
unknown evidence, never automatic enrollment.

Receipt and continuity tables are controller-inserted and serving-read-only;
origins are inserted by MM in the same creation transaction as their party,
sessions and queue intents. UPDATE, DELETE and TRUNCATE retain all three forms
of evidence. DOWN acquires the native close gate and exclusive table locks
before a fresh emptiness check, and refuses any begun head or retained history.
No historical party is backfilled. Schema constraints and synthetic retention
fixtures do not validate canonical wire contents, provenance, installation
custody, or native readiness. The actual source-origin writer, history reader,
candidate-v2 stamp and fixed native executable must qualify those separately.

`mmadmission.ReadHistory` reads the exact ADMITTED binding and canonical history
in one transaction. Capture requires read-only REPEATABLE READ; a READ COMMITTED
forward check acquires the same shared gate without waiting and retains it until
COMMIT. Writers still acquire `Acquire` first, before account or row locks. The
reader checks canonical receipt hashes against both SQL indexes and head, each
continuity link/root, stable lineage and scope, increasing admission generation,
and an explicitly bounded chain of at most 128 receipts. Missing, malformed or
unsupported history is unavailable, never a new genesis or automatic enrollment.
`HistoryCut.Authority` is data for the root's independent descriptor/readback
comparison, not a custody assertion.

The first supported origin contract is `NATIVE_VOICE_C16_V1_ORIGIN_V1` only.
`AllowsOrigin` checks original receipt authority across every intervening
compatible transition. It deliberately does not authorize an arbitrary party:
the source consumer must also bind party, operation, request, owner capture and
private manifest to the persisted origin. LFP remains unavailable until its
separate genuine typed source and lifecycle are implemented. Current stamp
rotation expires old candidates while a continuous supported rollout preserves
the immutable original source origin.

Bound native `CreateSourceParty` copies its runtime binding once, reads canonical
history under the admission gate, and inserts its origin in the same transaction
as the party, every session and queue intent. Replay validates the existing
origin against that exact party and continuous history; it never inserts a
missing origin for a historical capture. A compatible rollout changes the
current binding while returning the original unchanged origin bytes. Session
conflicts or failed intent insertion roll the origin back with the entire write.

The native private manifest uses SHA-256 over domain
`voice.mm.native-source-private-manifest.v1` plus NUL, followed by big-endian
protocol U32, operation UUID16, request hash32, initiator account/profile UUID16,
initiator epoch U64, source-kind U32 byte length plus UTF-8, optional room U8
presence plus UUID16, roster-version U64, member-manifest hash32, member-authority
version U32, member-authority hash32 and source-authority hash32. Thus the wire
family `NATIVE_VOICE` cannot reinterpret SOLO as VOICE_ROSTER or replace the
captured c16 authority. Origin/canonical receipt validation is still separate
from actual installed launch, fresh external admission, and source revalidation.

Proposal and provisioning candidate v2 explicitly hash the current admission
stamp, canonical origin for every complete party, and validated history-cut
hash. The latter commits all canonical receipts and continuity records using
domain `voice.mm.validated-history-cut.v1` plus NUL and deterministic JSON.
Proposal/provision candidate domains are `voice.mm.proposal-local-cut.v2` and
`voice.mm.provision-local-cut.v2`; they retain the full local row cut including
provider-operation bookkeeping. Public candidate/proof APIs are unchanged.

Each candidate freezes its expected binding. Immediate locked verification
re-reads the same explicit material, so an admission transition expires old
proofs even when the roster is unchanged. Fresh capture under a supported
continuous rollout preserves existing original origins. V1 capture/verification
is permitted only in a genuinely never-begun native-head view; a pre-cut proof
cannot cross a begun cut or enroll a typed historical party without origin.
The candidate defense takes a nonblocking shared history gate on READ COMMITTED
and does not replace each forward owner's mandatory admission gate FIRST,
before all account and row locks. Native serving remains unavailable until the
complete root, fresh external admission and all forward owners are qualified.
