# ExecPlan: единый спринт игровых интеграций Voice

## Purpose

Довести Voice-owned путь от независимо подтверждённого входа игрока до общения,
команды игровому backend, федерационной ноды и восстановления её bundle. Один
спринт означает один общий acceptance gate, а не один PR или обещание срока.
Этот документ — декомпозиция перед реализацией; ни один пункт ниже не помечен
готовым только потому, что существует спецификация или scaffold.

## Context

- Канон: [game-integrations](../features/game-integrations.md),
  [game-integration-api](../architecture/game-integration-api.md),
  [game-bot-interactions](../features/game-bot-interactions.md),
  [game-federation](../architecture/game-federation.md),
  [acceptance](game-integrations-acceptance.md),
  [design audit](game-integrations-design-audit.md).
- Общая очередь и границы: [PLAN](../PLAN.md), [TESTING](../TESTING.md),
  [DATA_MODEL](../DATA_MODEL.md), [DATA_STORES](../DATA_STORES.md),
  [ARCHITECTURE_REQUIREMENTS](../ARCHITECTURE_REQUIREMENTS.md),
  [CONTRIBUTING](../CONTRIBUTING.md), `.agent/AGENTS.md`, `.agent/PLANS.md`.
- Проверено на `codex/game-sdk-federation-docs` at `25c22d38` после
  `git fetch origin`: `origin/master` at `9dc04117` — предок ветки, ветка на
  9 коммитов впереди. Worktree спецификации чистый перед этим планом.
- The owner has now authorized this sprint to run alongside A1. The current
  `PLAN` still describes A1 as the only active milestone and federation as
  deferred because this sprint was being prepared. Record the parallel sprint
  in PLAN without changing A1's status. The initial master merge hold was
  explicitly lifted by the owner on 2026-10-02 for verified implementation
  checkpoints through a green-CI PR and merge commit. Staging deployment and
  live provider/device acceptance remain outside that authorization.
- В `src/backend/` нет Game Integration Service; есть только Federation Go
  scaffold (`main.go`, `health.go`), существующие Bot/Chat/Messaging/Voice и
  Java Auth. `federation_db` в DATA_STORES planned/not provisioned. Game API
  routes и `sdk-account` отсутствуют в текущем source inventory. Bot source
  audit привязан к старому SHA и перепроверяется на точном implementation base.

## Scope

**In:** отдельный Go Game Integration Service и store; Java Auth/User для
`sdk-account` и обеих конвертаций; app/env registry и bootstrap API;
versioned public protocol; Chat/Space/Role/Messaging/File/Realtime/Voice,
Bot/Notification/Flutter изменения; master + game node; один Voice Node bundle;
controlled game backend, protocol/media clients и обязательная acceptance.

**Out:** Unity/Unreal/engine assets, распространяемые SDK wrappers, developer
portal/CLI, публикуемые examples, engine certification, online migration между
нодами, HA и zero-downtime обещание.

**Invariant:** Auth остаётся Java, Realtime владеет WS; Messaging history
догружается отдельно по `chat_id`; каждый сервис пишет только в свой store;
master владеет accounts/Space/Role, node — разрешённым hosted content/media.

## Milestones and dependency spine

| Gate | Получаемый результат | Depends on | Acceptance |
|---|---|---|---|
| P0 | Честный implementation base, PLAN entry, замороженные authority contracts | — | Известны пересечения WIP и решения G/Q |
| P1 | Registry, Auth, sdk-account, device proof и обе конвертации | P0 | ID01–ID16 |
| P2 | Hosted sessions, party/match, MMO managed rights, real media | P1 | SE01–07, MMO01–05, SDK03/05/06 |
| P3 | Durable game events, commands, cards, messenger и opt-in | P1, P2 policy | BOT01–13 |
| P4 | Federation authority, hosted services, bundle и recovery | P1–P3 contracts | FED-AUTH, FED-BUNDLE, FED-UPGRADE, FED01–10 |
| P5 | Full Voice-owned acceptance and measured release evidence | P1–P4 | GI9, OPS01–02 |

P1 protocol and P4 transport tests may start in parallel once P0 freezes their
seams. A gate is complete only when its observable vertical runs on real Voice
services; a compiling scaffold or disabled flag is an intermediate artifact.

## Detailed steps

### Active continuation checkpoint — 2026-10-02

- Master integration: the owner explicitly requested merging the verified work
  on 2026-10-02. A clean integration branch, `codex/game-integration-master-20261002`,
  combines the sprint checkpoints with current `origin/master` at `1e7141cc5`
  and the isolated Auth source delivery `3d9e7097275f3e2b964e1286d114e8284d55e13f`.
  Required CI must pass before a merge commit. This checkpoint delivery does not
  close the full feature gate or authorize staging, live providers or devices.
  All inherited uncommitted WIP and the original 26 staged Dart blobs remain in
  the original worktree. Future agents must continue from that preserved WIP;
  a merged checkpoint is not evidence of full T73/T77/T90–T94 acceptance.
- Auth now serves complete owning account/SDK facts through a separately enabled
  source-only mTLS listener. Flyway26 and Go27 install byte-identical global
  transactional clock coverage over the ten source tables, with retained floor,
  trigger/function catalog pinning and forward-only downgrade refusal. Exactly
  one clean loader history is required. The reader bounds the whole operation,
  including pool checkout, to one second and reads state plus clock in one
  repeatable-read cut. Ordinary and standalone SDK identities remain distinct;
  every requested account has explicit present/missing facts. Raw key/session/
  linked-session leases keep bytes stable at time-only expiry and cap renewal.
  Passwords, token hashes, JWKs, provider subjects and proof/receipt bytes stay
  local. Java JDBC and Go decode the same exact all-eight-group golden bytes,
  including bindings, conversions and durable message grants. Both validate
  exact grant environment and authorization source. Auth facts alone neither
  prove a caller's SDK bearer nor establish Voice permission. Actual protected
  factory tests cover mTLS, replay, JWKS recovery, dependency failure, revocation
  and serving maintenance; source RPCs are absent from both legacy listeners.
  The isolated 27-path delivery passes 142 selected Java roots and 16 shared Go
  source roots without skips; GOWORK=off authoritysource/integrationtest vet passes.
  Current conversion protocol WIP and its authority_epoch test fixture repair
  remain local; isolated verification uses HEAD's matching older client/fixture.
  Scoped review clears the reader/runtime and the repaired Java/Go parity seam.
  GIS, unchanged-vector publisher, content ACL, permanent node fences and combined
  native-media/capacity acceptance remain open. No live source flag, schema or
  migration history is changed and no parent gate advances.
- User now serves an exact complete profile/SDK actor source on its separately
  enabled source-only mTLS listener. Clean catalog19 and one transactional global
  floor cover profile statements, durable inactive overlays and immutable SDK
  actor tombstones. Every requested profile has an explicit present/missing fact;
  absence alone neither grants User rights nor marks an SDK account inactive.
  Historical aliases only deny the exact source account/actor and never import
  target permissions/history. Names, privacy, receipt/proof/hash bytes stay local.
  Actual pinned migrate and protected application factory checks cover revocation,
  rollback preservation and dirty maintenance refusal. Catalog drift and scope
  errors deny reads. A new two-profile deletion regression exposed pgx conn busy
  before Search deletes; the canonical writer now closes its returning cursor
  before appending events in the same transaction. Exact event replay preserves
  the original fence and clock; forced outbox failure rolls back inbox, overlay,
  profiles, events and clock and permits retry. The isolated 17-file delivery
  passes 38 selected roots without skips, including SDK/deleted-account/event
  regressions and shared transport checks. User short passes 106 roots (96
  explicit integration skips); User vet and shared source/fixture vet with
  GOWORK=off pass. The existing local Game Integration protobuf dependency is
  declared explicitly so the isolated User module resolves its generated Bot
  types. Other go.mod WIP remains local. Scoped review clears this seam.
  Auth/GIS owning readers, common-vector publisher, content ACL, permanent node
  fences, bundle/media/capacity acceptance remain open. No live source activation
  or history changes and no parent gate advances.
- Space now has a complete owning source on a separately enabled source-only
  mTLS listener. Its schema24 state/floor cut includes membership, account bans,
  communication timeouts, lifecycle/ownership gates, scoped room/category/tree
  references and raw community owner/member leases. Public/guest settings do not
  invent membership. Current-generation managed membership requires both leases;
  the earlier validity cutoff prevents stale renewal even with no counter write.
  Expired raw rows remain stable at the same revision, while manual membership
  survives managed-roster expiry. Unknown missing floors and cross-Space tree
  references fail closed; known retained floors return complete closed state.
  The shared source-only runtime checks owning schema before registration and
  requires explicit per-owner activation, Federation trust, mTLS and Redis replay.
  Actual pinned migrate plus application factory tests prove complete reads,
  membership revocation and maintenance after refused downgrade. Shared transport
  checks cover TLS rejection, JWKS recovery and replay dependency failure; four
  PostgreSQL reader roots cover complete state, bans/timeouts/lifecycle common cut,
  scopes/floors and time-only expiry. The isolated 17-file delivery passes
  17 selected roots without skips and the complete Space short suite passes
  190 roots (335 explicit integration skips). Space vet and shared source/fixture
  vet with GOWORK=off pass. Scoped review finds no blocker in this seam.
  No source flag or live schema/history is changed. Complete User/Auth/GIS readers,
  production publisher/common vector, content ACL, permanent fences, native-media
  combined acceptance and measured capacity remain open; no parent gate advances.
- The protected `voice.authority.v1.AuthoritySourceService` contract and first
  Role owner are implemented. Role's complete canonical snapshot reads roles,
  assignments, chat/voice overrides, lifecycle/ownership/deletion gates and active
  room-scoped SDK grants with a read-only repeatable-read state/revision cut.
  Additive Role16 protects a global SDK revision; its sum with the per-Space Role
  floor detects both source classes, with conservative unrelated SDK invalidation.
  Exact schema/relations/triggers/function bodies, known bits, subject bounds and
  cross-Space assignment/session conflicts are checked before completeness.
  Registration is opt-in on the existing private mTLS listener, requires exact
  Federation trust and a clean schema16 preflight; subsequent reads repeat guards.
  Legacy and disabled listeners never expose the source RPCs. Actual pinned-driver
  PostgreSQL plus application-factory mTLS/JWKS/Redis tests cover snapshots,
  membership removal, replay/request tamper, bad transport and live maintenance.
  Source, activation and SDK floor regressions have saved red/green evidence.
  The isolated 27-file delivery passes 21 selected roots without skips, the
  complete Role short suite passes 97 roots (127 explicit integration skips),
  and Role/shared-source vet and buf lint pass. A scoped independent review
  clears this Role seam. Nothing is enabled in the
  live Compose runtime. Full Space/Auth/User/GIS owner reads and durable counters,
  exact owning room-set validation, publisher unchanged-vector common cut anchored
  before its first read, production masterVoice/node/SFU chain, permanent fences
  and measured capacity remain open. No parent T73/T74/T75/T77/T91 gate advances.
- Auth source catalogs now retain canonical refresh-token profile binding at
  Flyway V15 / golang-migrate 000016 and shift the complete SDK chain to
  V16-25 / 000017-26. All ten SDK UP bodies are unchanged and mirrored exactly;
  SDK DOWN refuses downgrade instead of deleting authority/receipt evidence.
  Fresh actual Flyway and pinned golang-migrate loads pass on private PostgreSQL
  16, with matching SDK columns/defaults and repeat no-op. Historical SDK-only
  Flyway history is refused with unchanged rows/history; the old Go marker18
  refuses current19 with saved state retained and a dirty maintenance marker.
  Current26 downgrade refuses at dirty25 and retains SDK owner receipts.
  The isolated delivered closure passes 127 selected roots without skips:
  three source contracts, four real loader/history/rollback checks and 120
  SDK identity/authorization/conversion checks; the author fixes the SDK
  fixture by naming the new FK-dependent receipt table in its cleanup.
  Current WIP authorization53/conversion22 also pass; scoped independent
  review finds no remaining fresh-catalog blocker. Full historical recovery
  remains unaccepted. No live Auth metadata/image is changed. Existing deployment history requires
  inspection and a matching verified backup/reconciliation plan; source
  renumbering never authorizes history repair/baseline/force. Parent T77/T91
  and the full bundle/owning-producer gates remain open.
- Checkpoint `9da01a3e2` delivers thirteen Role migration-repair paths; remote,
  delivered additions/deletions and original 26 staged blobs are verified.
  Active owning-source prerequisite: additive Space24/Role15 preserve v11 data,
  invalidate saved scopes at activation, cover authoritative insert/update/delete
  changes in both old/new scopes, and forbid epoch rewind/removal/truncation.
  Coarse Space-wide invalidations retain the existing Voice event schema. Baseline
  regressions reproduce missed bans/lifecycle/owner/chat/fence/scope changes and
  removed floors. The isolated committed closure passes nine real PostgreSQL
  roots without skips, including both complete pinned migrator catalogs, refused
  downgrade and late-activation rollback with original triggers/floors retained.
  All 286 short roots pass (449 explicit integration skips), both module vets
  pass, and independent scoped risk review finds no remaining blocker. Coverage is
  forward-only: downgrade must not silently remove authority protections; recovery
  uses a verified matching database/runtime bundle. Complete protected owner
  snapshots, Auth/User/GIS revision closure and the real publisher remain open.
- Role migration repair retains canonical game-grant v13 bytes and places
  lifecycle fences at unique v14. Real PostgreSQL checks pass all six migration
  roots without skips: unique source loading, exact historical/combined schema
  adoption, invalid catalog/version refusal, concurrent rollback barrier,
  forced-RLS non-superuser rollback and the pinned golang-migrate transition.
  The RLS regression failed before hardening: hidden receipts let DOWN succeed.
  Both directions now reject filtering/rewriting, inheritance and non-durable
  tables under locks; saved evidence is retained. Independent risk review finds
  no remaining blocker in this repair. The isolated committed dependency closure
  passes 23 PostgreSQL/runtime roots without skips (migration six, game grants
  four, retirement/ownership thirteen), all 96 Role short roots with 114 explicit
  PostgreSQL skips, and full module vet. Current WIP affected roots also pass.
  Next source-revision regressions reproduce missing bans/lifecycle/owner changes,
  both-scope updates and removable/rewindable revision floors. Repairing these is
  prerequisite to the real owning-source publisher; no parent accounting advances.
- Checkpoint `84d432357` delivers 22 boot/protocol/runtime/fixture/documentation
  paths. Remote HEAD and every delivered byte are verified; original 26 staged
  blobs equal the initial index. Next owning-source path uses each owner's
  monotonic per-Space revision captured with complete state in a repeatable-read
  transaction, then rereads all source tokens before publication. Unchanged
  vector gives a common stable cut; validity is anchored before first read and
  never exceeds two seconds. Missing/stale/changed owner state denies renewal.
  First batched blocker is duplicated Role migration 13: keep canonical game
  grant migration bytes unchanged; move lifecycle fence schema to unique 14 and
  safely adopt exact preexisting local schemas without changing rows/receipts.
  Reject partial/noncanonical schemas and retain evidence on rollback. Test
  clean upgrade, either historical-13 shape, exact bytes, source loading and
  rollback barriers. Then extend missing Space bans/lifecycle/ownership and
  Role lifecycle/chat-override/OLD+NEW scope revision coverage before exposing
  complete owner snapshots. Active Voice invalidation is the immediate revision
  consumer; the real Federation source publisher is the acceptance consumer.
  Source: game-federation §6 and T73/T74/T77; no parent accounting advances.
- Checkpoint `b4bf53caf` delivers seven Messaging preflight paths; exact delivered
  bytes, pushed remote and original 26 staged blobs are verified. Next T75/T77
  slice uses the actual independent SFU and node-media receivers: each process
  creates a fresh cryptographic boot UUID in memory and publishes a bounded
  one-second heartbeat request in a separate local IPC directory. The controller
  reads current requests after complete page verification and includes up to 16
  sorted unique UUIDs in its fresh applied-revision ACK; master signs that exact
  list in the short lease after current placement/policy/fence validation. A boot
  receiver accepts only a verified lease naming its own UUID, so restored Bundle
  files and rolled-back wall time cannot establish first authority. UUIDs are not
  configured/restored identities and request files contain no node credentials.
  Legacy transport fixtures remain decodable; production SFU/exchange/controller
  require the boot request directory. Tests cover replay/new receiver/fresh lease,
  malformed/expired/cross-node requests, real mTLS master ACK, both production
  receivers and controller death/restart. Source: game-federation §9 and T75/T77.
  The broader owning-source projection, remote purge participant and complete
  bundle/fault/load acceptance remain open; no parent accounting advances.
- Fresh boot guard now reaches both production receivers and controller. Current
  Federation root PostgreSQL/mTLS 22/0 and nodepublisher 5/1 (the intentionally
  opt-in Linux fixture) pass. Controlled Linux production edge restart 1/0
  proves the unchanged saved lease is still cryptographically valid at denial,
  a new process UUID exists, and a newly signed boot lease restores admission.
  The new SFU/controller/media images pass real four-peer/two-Space RTP 1/0:
  revoke eject 155ms; controller kill/reap eject 1174ms while signer/SFU remain
  alive. Same bounded reviewer clears protocol, scope, replay, documented
  deployment and evidence. Shared IPC UID10001/mode0750/0600 files and backup
  exclusion are documented; malformed requests fail closed. Isolated full root
  22/0 and module short 36/12 plus vet pass; the 12 skips are deliberately
  excluded integration paths covered by full PostgreSQL and Linux media runs.
  Final current-input Linux media 3/0 passes: direct signed authority,
  production controller death and production HTTPS edge restart. Worst measured
  eject is 1271ms; revoke 123–230ms; restart 0.23s. Images: SFU `680718ce69f7`
  (input `56f386196a48`), controller `73f36df5e095` (input `fd6e233493e8`),
  media fixture `e1d6e417ffdf` (input `db2a607c4757`). Exact build-input checks
  and prompt push preserve the original 26 staged blobs. Graphify's fresh
  changed-input attempt again timed out at 45.4s; only its owned process tree
  was stopped, so graph freshness remains unverified. Parent 43/60 unchanged;
  this mechanism does not complete owning-source projection, remote Space purge,
  whole bundle restore/upgrade or qualified capacity/fault acceptance.
- Checkpoint `693c26057` delivers the seven retirement/clock paths; current and
  isolated full Federation root 22/0, module short/vet, same reviewer and normal
  push pass, with all 26 staged Dart blobs preserved. Next bounded T77 repair:
  before enabled Messaging managed-chat/Chat/Space lifecycle workers or
  listeners serve, require clean migration >=000026 and all nine durable-intent
  columns/types/nullability. Missing/dirty/old/partial schema fails with a fixed
  diagnostic; disabled baseline retains its path. Seven real-PG attachment
  roots pass without skips, including schema and rollback evidence retention;
  three runtime config/order roots pass, including the actual enabled gate
  before TLS loading/listener creation. Current module short/vet pass. The
  rebuilt owned image `f2f33c34c4ed` refused startup because this fixture's clean
  marker still said 25 after an earlier direct canonical-26 DDL application.
  Under table/marker locks, a temporary table from the committed migration was
  compared with the complete columns/defaults/constraints/index contract;
  guarded marker 25→26 reconciliation preserved all ten intent rows and their
  aggregate hash. The same image now starts healthy. Actual public freeze/restore
  passes (21.81s); nonempty shared/exclusive attachment purge passes (12.08s,
  ten fences/ten purges, no skips) after fixing this runner's missing owned
  MinIO host-port setting. Isolated candidate root 21/0, attachment PG 7/0 and
  module short 213 pass/213 skip plus vet pass. The same reviewer clears the
  preflight/order repair. A bounded graph refresh times out at 45.4s; only its
  owned process tree stops. This is a
  preflight slice, not complete bundle upgrade/backup acceptance. Source: T77
  and Messaging durable File reference/lifecycle contract; physical remote node
  recovery and parent accounting remain open.
- Verified checkpoint `fc2ff6b6d` delivers 42 media issuer/client/exchange paths;
  isolated Federation root (19/0) plus authority/cache/protocol (6+9+1/0), Voice
  short (184/113) and both vets pass. Normal hooks/push preserve all 26 staged
  Dart blobs. T75 master issuer/controller repair now invalidates saved authority
  on a new canonical resource generation and requires a new complete reconciled
  owner policy. Purging/tombstoned history permanently denies revival, including
  pre-repair later active rows and generation-zero legacy content permissions;
  immutable rows stay intact. All snapshot/lease/revision/media issuance paths
  enforce these fences. Source and credential clocks are checked after blocking
  queries and before signing. Three new real-PG roots prove retirement, legacy
  backup-row denial and expired-source lock waits for lease/manifest/page/
  revisions; full Federation root 22 pass/0 skip and module short/vet pass.
  Same reviewer found the legacy-generation bypass; reproduced red, repaired
  and green. Source: game-federation §9 and T75 permanent-fence acceptance.
  Fresh-online boot/backup fencing and complete owning-source projection remain
  open; parent T75 and historical 43/60 accounting do not advance. A new bounded
  graph refresh timed out at 45.3s; only its owned process tree was stopped.
- T73/T74 now has a master Voice consumer and node-local HTTPS exchange candidate.
  A separated pinned Voice mTLS role resolves the exact current global resource
  route plus one unambiguous projected app/environment/binding/installation
  tuple, then issues the narrow signed credential under canonical locks. Node
  `node-media` holds only public master trust, local HTTPS identity and its SFU
  secret; it checks current Bundle before wrapping the original credential in a
  private LiveKit JWT. Signed `can_publish` also needs scoped `media_publish`;
  SFU rejects wrapper escalation and active publishers close when that permission
  disappears. Voice preserves canonical owner/membership/Role/fence checks and
  never sends a master user token or Voice client certificate to the node. Only
  explicit version-1 not-hosted lookup permits baseline fallback. Independent
  review reproduced a foreign-Space resource mistaken for not-hosted (0 pass,
  1 fail, no skips); global resource lookup now denies it. The same reviewer
  clears the repair. Full Federation root: 19 pass/0 skip; Voice short: 184 pass/
  113 intentionally skipped integration roots, 16 packages; both vets and master
  Voice Docker build pass. Actual separate UID 10001 node-media HTTPS child
  returns every admitted JWT in the controlled-signer/controller/SFU RTP run:
  1 pass/0 skip, 8.97s; revoke eject 237/237ms, controller death 1,168/1,170ms,
  live exchange refuses a fresh credential after authority expires. Its public
  WSS URL is substituted only for fixture-internal media transport, so full
  bundle TLS/owning-service projection/capacity acceptance remains open. New
  master/client tests and this media run are separate proofs, not a combined
  production owning-source run. Deployment must set the opt-in Voice config
  before exposing mapped rooms; incomplete configuration fails startup. Graphify
  update on these changed inputs again times out at 45.4s (own process only);
  graph freshness remains unverified. Accounting remains 43/60.
- The complete routed/application tuple passes real SFU admission negatives and
  controller process-death acceptance: 1 root, 0 skips, 7.99s; Space revoke eject
  188/189ms, controller SIGKILL eject 1,173/1,174ms. Master issuance passes full
  Federation root suite (19 roots, 0 skips) and vet, including actual mTLS roles,
  canonical route and expiry during a blocked resource lock. Next consumer is
  node-local HTTPS token exchange: only the master-signed narrow credential may
  choose subject/room/publish permission, node secret stays local, both exchange
  and SFU independently check current policy. Add signed `can_publish` and scoped
  `media_publish` permission before exchange so listen-only access cannot be
  escalated by node JWT claims. Regression order: master/cache publish denial,
  exchange token/privacy/expiry negatives, real runtime and media. Root is sole
  writer; bounded luna auditor traces trusted caller binding/route context for
  master Voice integration. Sources: game-federation §§5–7 and Voice canonical
  Role publish checks. Full producer projection, permanent boot fences, capacity
  and whole-sprint gates remain open; accounting stays 43/60.
- Next T74 consumer: extend the private media admission and projected permission
  tuple with positive routing generation, fresh grant nonce and optional complete
  app/environment/binding scope (installation where applicable). Admission and
  active-media checks require exact projected tuple/RTC room; legacy unscoped
  policy cannot authorize routed media. Master issuance must derive node/Space/
  room/generation from the current canonical route, use current complete policy,
  and accept only a separate trusted Voice mTLS caller, never a node/operator or
  client-supplied signing identity. The Voice/node exchange remains part of this
  consumer; no full T73/T74 claim before actual issuer plus media acceptance.
- T73/T74 next-consumer repair reproduced an active foreign-node resource
  registration (0 pass/1 fail, no skips) and now locks the exact node/Space
  placement before append/replay. Voice routes require the canonical owner's
  explicit RTC room; durable bindings are immutable and unique per node/room
  across resources/Spaces. Migration 5 preserves schema-4 evidence, rejects new
  foreign generations, and adds legacy missing rooms only through a new explicit
  owner generation. Full Federation root tests pass: 17 roots, 0 skips, including
  schema-4 upgrade, placement/collisions, canonical API statuses, real mTLS node
  controller, lifecycle and Q11. Vet passes; independent risk review finds no
  blocker. Actual producer projection/media-grant issuance remains next, and
  parent accounting remains 43/60.
- T73 production node controller now consumes the existing
  Federation node mTLS/bearer manifest/pages/lease API, verifies the complete
  signed per-Space policy before ACK, and atomically publishes the exact Bundle
  read by the SFU. Independent Space workers preserve the last complete file
  on fetch/verification failure and cannot extend its signed expiry. A directory
  sync failure after atomic rename means complete replacement with uncertain
  durability. Tests reject partial,
  foreign and stale envelopes, incomplete-policy ACKs, unsafe HTTP/redirects,
  bad receipts and publication failures; rollback floors apply before ACK.
  Four host roots pass; the Linux image passes all five roots including atomic
  file publication. A real Federation mTLS API/PostgreSQL test passes against
  current root WIP (separate from this dependency-closed controller artifact).
  The actual Linux production child runs UID/GID 10001 and consumes signed
  policy for two Spaces before four peers exchange RTP. Revocation closes one
  pair in 259/259ms; killing and reaping the controller closes the other in
  1,073/1,074ms, 0 skips, 7.53s. The signer continues advancing, file bytes and
  ACK counts stop changing, the SFU remains healthy, and stale/fresh unexpired
  bearer reconnects are denied. Independent bounded risk review finds no blocker.
  Actual master projection/media-grant issuance, fresh-online boot fencing,
  node bundle deployment/credential reload and capacity qualification remain
  separate open gates. Parent accounting remains 43/60.
- T40 real public attachment fixture first failed at zero durable File refs
  after a successful upload/send. The repair persists an exact pending send
  intent before the protected File acquire RPC, serializes client retries and
  recovery, and commits visibility plus intent completion under the Space
  lock/fence. Both ordinary Send and Forward use it when the protected owner
  runtime is enabled. Abandoned acquisitions expire after 15 minutes and use
  authenticated exact release; frozen intents defer to the sealed producer,
  and pending nonvisible refs are captured in that immutable set. Private
  purged-Space intent data expires with the 30-day parent evidence window.
  Seven PostgreSQL roots pass (retry/conflict, concurrency, abandoned recovery,
  legacy dedupe, freeze during acquire, pending freeze capture, producer
  replay/retention); migrated File metadata/bulk uploader denial passes.
  Rebuilt owners pass actual public upload/send/forward/freeze/purge with
  nonempty exclusive and shared blobs: Space `b24c1f1e-0386-4ec1-a707-73adbfbfb544`,
  operation `d560b84f-7336-476e-91f3-44a91f3c1a92`, PURGED/G2, 10 fences/10
  purges, 0 skips, 12.75s. The exclusive ref has durable GC handoff; the shared
  file retains exactly the foreign Space ref, stays LIVE and downloads its
  original bytes. Unchanged full selector activation, frontend propagation,
  game-owned attachment producers and full T40 scenario matrix remain open.
  No gate or parent accounting advanced.
- Dependency-complete lifecycle checkpoint `0cb996890` is pushed: 450 runtime,
  migration, protocol/generated-Go, Compose and scoped documentation paths.
  Its isolated HEAD-plus-selected-source snapshot passes short suites and vet
  in 13 modules (1,972 root passes, 1,547 intentionally skipped integration
  roots); corrected runners execute each binary from its package directory.
  Buf lint, format and breaking against local `master` pass. The original
  26 staged Dart paths and their indexed blobs are verified unchanged, and
  all remaining WIP is preserved. Snapshot/index/source hashes and the result
  remain in `tmp/lifecycle-runtime-checkpoint-20261002`. These short-suite
  results do not substitute for full PostgreSQL/public/platform acceptance.
  Parent accounting remains 43/60; T40 attachments/full messenger and the
  remaining whole-sprint gates are still open.
- T73 now has a maintained SFU implementation candidate pinned
  to upstream LiveKit v1.8.4/protocol v1.34.0 archive hashes. Dedicated private
  `voice_media_grant` claims bind explicit RTC room and the verified authority
  tuple; `RoomManager.StartSession` checks before new admission/resume and
  immediately before join/resume. An independent in-process watchdog checks
  saved admissions against per-Space signed complete policy/lease; one Space
  cannot replace another's scope. Short-lived credentials do not renew policy,
  and token refresh preserves the original credential. Missing authority
  configuration fails startup. Node-cache expiry/replay/clock rollback and
  registry higher-revision backup/credential regressions pass (9 + 3 roots);
  the same bounded reviewer clears this scoped security/integrity slice.
  A pinned image builds with actual upstream source. Actual
  `TestSFUEnforcesSignedAuthorityForRealMedia_live` passes without skips (7.89s):
  four peers exchange bidirectional nonempty RTP in two Spaces, a revoked pair
  closes in 172ms, the other pair continues after its admission credential
  expires, and stopping signed-authority refresh closes it in 1,273ms while
  the SFU remains alive. Unexpired stale JWT/grant reconnect is denied after
  revocation and lease expiry; private grant is absent from participant
  metadata/attributes. The first fixture had only one allocatable UDP socket;
  `udp_port: 7882` multiplexing fixes multi-peer ICE. This is controlled-signer,
  four-peer mechanism evidence, not production/capacity qualification. A clean
  initializer run repeats the pass (8.84s, revocation 199–200ms, partition
  1,275–1,276ms). Missing authority/trust startup tests fail closed with exit 1;
  the maintained patch corrects upstream's print-error-but-exit-zero behavior.
  Production master issuance/publisher, durable restore reconciliation,
  controller-process death and qualified 2× load/p95 evidence remain open.
  No strict federation capability or parent checkbox is activated from these
  intermediate results. Two bounded Graphify updates timed out; source files
  remain the verification authority and graph freshness is unverified.

- Handoff continuation preserves pushed HEAD `fb91bf9a1`, all inherited WIP
  and the 26 staged Dart bindings; the initial index/hash inventory is saved
  in `tmp/continuation-20261002`. The nonempty purge repair is now verified:
  Search admits only the authenticated request-bound Messaging child matching
  its sealed Space manifest and `PURGE_DECIDED` fence. Per-operation locking
  preserves the first receipt on concurrent retry; foreign Chat IDs fail
  atomically, ordinary frozen access stays denied, and exact terminal replay
  survives private-manifest compaction. Messaging atomically removes scoped
  reactions, hides, pins, game sidecars, read/delivery positions and scheduled
  payloads as well as messages; verified nanosecond cutoffs replay exactly
  despite PostgreSQL microsecond storage. Its Space adapter validates the
  existing T33 raw deterministic-protobuf digest, keeping the P3 parent digest
  domain separate. Red/green real-PostgreSQL and real-coordinator regressions,
  affected short suites/vet and the same bounded independent risk review pass.
  Public `TestComposeSpaceLifecycleExpiredPurge_live` passes without skips
  (9.95s): fresh Space `7b8d2d34-cc5b-41ed-8b41-6fbf42003328`, operation
  `74334e6a-fc9f-4ea6-b445-2775082f25fc`, `PURGED/G2`, ten fences and ten
  purge receipts, physical removal/tombstone/GET+restore404 and preserved
  control Space. No saved evidence was rewritten. This fixture advances only
  the guarded mutable aggregate deadline, as documented below; it does not
  claim seven elapsed days. Attachment-reference purge and the full T40
  messenger/game matrix remain open. Parent accounting remains 43/60.
- The expanded public freeze/restore fixture passes owner and joined-member
  ordinary Chat/history/Search reads before freeze; all content reads and sends
  receive policy denials while frozen, and restore reopens reads with the exact
  original message. Chat now excludes non-LIVE Space rows at direct lookup and
  inbox/archive/Space/custom+system-folder/quick-access SQL reads. The same
  reviewer found and cleared the frozen-shortcut whole-list failure; the new
  PostgreSQL regression proves control/standalone preservation and restored
  slots. The combined navigation PostgreSQL selection passes 10 root tests with
  no skips; full Chat short suite and vet pass. Refreshing the owned Social
  image from merged source resolved the previously stale member-history 503;
  this is steady-state evidence, not container replacement fault acceptance.

- Worktree remains `.treehouse/Voice-451d38/24/Voice`, branch
  `codex/game-sdk-federation-docs`; current HEAD is pushed Search checkpoint
  `fb91bf9a1`, after master merge `3aab07d4d`, incorporating `origin/master`
  at `9ef2c6de6` (138 new commits,
  including merged A1 PRs #596/#597). Last pushed runtime checkpoint is
  `d51d5b6bc` (Subscription typed receipt digest), after `82b19242f`
  (Role durable authenticated lifecycle barrier), `35fec71a6`
  (shared gRPC metrics registry), `0086b418e` (Subscription mTLS), `083eaedec` (Notification) and
  `2a26c9d36` (canonical File producer digest). Inherited staged Dart bindings and other sprint
  WIP are preserved. One writer owns this worktree; the same bounded Luna
  reviewer checks changed purge/security inputs.
- The shared `pkg/filerefmanifest` digest now has an independent empty-producer
  golden value and binding/invalid-input coverage (`go test ./filerefmanifest`,
  one passing test). Its checkpoint is a foundation for owner declarations;
  it does not mark a lifecycle runtime or parent gate accepted.
- Current uncommitted T40 integration adds Space runtime configuration and all
  ten typed owner clients, exact Chat-page import into Messaging/Notification,
  explicit SPACE/CHAT/MESSAGING File declarations, DB-time expiry decision,
  durable retry/stall tracking, local atomic purge and an explicitly development
  HMAC fixture. It corrects the purge dependency order to Role, Messaging, Chat,
  File, then remaining owners. Nonempty Messaging children now require the
  verified sealed producer release and cannot call File's LIVE-only ordinary
  release. Ordinary requests cannot bypass frozen mutation guards.
- Accepted component evidence for these current inputs: Space short suite
  340 tests/14 packages; Messaging short suite 403 tests/16 packages; both
  modules' vet; Messaging producer/child/purge selection 7 tests; PostgreSQL
  snapshot/expired-child denial, restored producer expiry, Space fresh-clock
  expiry, atomic local purge/30-day compaction/365-day expiry, durable retry/stall
  and exact outbox acknowledgement regressions. File's real PostgreSQL purge,
  restore and foreign durable tuple selection passes 3 tests. Real JetStream
  lifecycle delivery/deduplication and invalid-ack selection passes 2 tests.
  Results apply only to their covered inputs; subsequent edits need affected
  checks before a coherent runtime checkpoint.
- New Space migrations 000022/000023 retain ten compact completion tuples,
  remove private participant/operation evidence after PURGED +30 days, and
  expire tombstone/account HMACs after 365 days. READY delivery rows survive
  aggregate expiry independently; DELIVERED rows expire after their first ACK
  +30 days. Outbox and retention changes have passing PostgreSQL regression
  evidence and the same reviewer's clearance. Repeat-cycle PostgreSQL coverage
  now proves freeze/restore at G1/G2 followed by G3/G4, with prior operation
  outcomes preserved and old exact receipts inert. Completed restored outcomes
  expire after 30 days using DB time, including a Space that stays LIVE; active
  deletion proof remains available for finalization. The combined cycle,
  restored-outcome retention and atomic finalizer selection passed 3 tests.
  Production KMS/HSM integration is not claimed
  by the development key fixture.
- File, Search and Subscription now require verified client certificates on
  their protected listeners. Subscription's clean checkpoint passed 92 short
  tests/13 packages and vet before its pushed commit. Current File inputs pass
  197 short tests/12 packages and vet, including real TLS lifecycle/producer,
  receipt and capability owner routes; Search passes 147/11 and vet. A new
  Search-only Chat page listener repairs both the missing transport and Search's
  incorrect fresh request-ID binding. Chat passes 124/8 and vet, including real
  HTTPS JWKS, mTLS, replay, wrong-owner/hash and mutation-denial coverage. The
  same independent reviewer found no blocker in this transport batch.
- The opt-in `docker-compose.space-lifecycle.yml` adds all ten protected
  endpoints, separate owner keys/certificates and the development tombstone
  key. Public Search fixture keys bootstrap Chat without the base stack's
  Search/Chat dependency cycle. A read-only combined public-CA volume preserves
  existing User/Social/Story trust; Story's protected File consumer now presents
  its own client certificate. Story's 48 short tests/9 packages pass; template
  and full transport acceptance remain part of the consolidated gate. All
  ten participant images and the new Space dependency closure build on these
  WIP inputs. The latest fixture suite passes 27 runnable tests with one
  Linux-only POSIX permission skip. Runtime startup exposed and repaired raw
  NATS bootstrap subject drift, the Chat/Space eager JWKS dependency cycle,
  Space's HTTPS proxy upstream, the missing proxy readiness dependency and
  Messaging's incorrect issuer-specific Space JWKS path. Messaging now passes
  404 short tests/16 packages and vet. The shared metrics fix prevents Voice
  and Matchmaking listener startup panics; shared packages pass 369 tests/30
  packages and vet, with 8 race tests and vet on an isolated grpcmw snapshot.
  An attempted full shared-suite run in that partial snapshot lacked repository
  deployment fixtures and is not acceptance evidence. Independent review found
  no remaining blocker in the TLS bootstrap or metrics fixes.
- Remaining dependency blockers include the complete public lifecycle matrix
  and actual all-ten purge acceptance. Clean dependency-closed checkpoint
  packaging, fresh all-ten repeat-cycle recovery and the remaining
  T40/T58/T59/T73–T78/T90–T94 matrix stay open. No parent checkbox advances.
- The owned `voice-game-lifecycle-20261002` acceptance project is being created
  from this worktree with random loopback Gateway/PostgreSQL ports. Compose
  startup now passes for Space, all ten participants, Gateway and dependencies
  on WIP images; the passing freeze/restore slice is recorded below and does
  not close lifecycle acceptance. Public deletion-proof,
  delete, restore and existing GetSpace routes now use strict JSON and a separate
  Gateway-to-Space mTLS listener. Delegated-user signatures bind the verified
  actor, positive session epoch, exact method/body and request ID. Shared Redis
  enforces Auth's durable epoch floor and replay rejection across restarts.
  Ordinary Space listeners deny lifecycle mutations; the protected frozen read
  returns only the recorded owner's minimal projection and hides nonowners.
  Space passes 352 short tests/15 packages and vet; Gateway passes 683 short
  tests and vet before the latest route cases. Additional real TLS/replay/epoch
  runtime selection passes 12 tests including race; real PostgreSQL public read
  selection passes 2 tests; Gateway signed route selection passes 14 tests,
  including SDK/guest writes with forged regular-account headers and signed
  LIVE reads. The same security reviewer found no material bypass, and requested
  the combined real Gateway/TLS/frozen-store path now being tested. Fixture
  generation passes 28 runnable tests with one Linux-only permission skip.
  The opt-in live acceptance test is guarded by both explicit flags, an owned
  `voice-game-lifecycle-*` project and explicit loopback API/mail-fixture URLs.
  Its first run stopped before lifecycle writes because the member's default
  invite privacy denied joining; the fixture now explicitly opts into invitations.
  Real acceptance then exposed an extra protobuf wrapper between the Auth
  consumer and Store. The consumer now saves the exact deterministic typed
  response; the corrected wire regression failed before the fix, then three
  focused tests including real PostgreSQL persistence/retry passed. Space's
  updated short suite passes 353 tests/15 packages and vet; the same reviewer
  cleared this repair. The rebuilt Space image is
  `sha256:3ffa3d1f7e55ac02128a4a0053fca799fc599d659999f216e97041596b0b8a0d`.
  The next run exposed Role's absent fence RPC. Checkpoint `82b19242f` now adds
  the protected handler, durable generation/manifest fence, ordinary/ownership
  admission guards, exact receipts, retention worker and migration 000013.
  Retirement now requires the matching durable PURGE_DECIDED tuple before
  mutation. Actual mTLS/JWKS/PostgreSQL freeze/restore/restart, compact replay
  and retirement barrier selection passes 3 tests; earlier affected lock and
  migration selection passes 14. A clean dependency-closed Role snapshot passes
  319 short tests/10 packages and vet. The same reviewer cleared the barrier
  and full typed request digest. The checkpoint contains exactly 20 Role/docs
  paths; inherited staged Dart bindings remain outside it.
  Public acceptance then exposed inconsistent participant receipt digests:
  Role, Chat, Voice, Matchmaking and Subscription must all bind their full
  typed RPC wrapper, using its protobuf FQN + NUL + deterministic bytes.
  This is distinct from transport credential hashing. Role's repair is in the
  checkpoint; the other four are current WIP. Their PostgreSQL regressions
  failed before repair and now pass: Chat 1, Voice 1, Matchmaking 1 and
  Subscription 2 (including purge and exact restart replay). Affected suites,
  independent review and combined public acceptance remain in progress.
  Their current short suites pass Chat 124/8, Voice 368/25, Matchmaking 141/18
  and Subscription 92/13; all four vet checks pass after correcting an
  inherited Voice test stub to clone rather than copy protobuf internal locks.
  The same reviewer cleared the four digest repairs. Pre-fix disposable
  participant receipts remain immutable and cannot satisfy the repaired
  protocol; fresh fixture Spaces are used instead of deleting old evidence.
  No production/staging activation is claimed. Any non-disposable rollout
  with earlier receipts requires a non-destructive compatibility plan first.
  Subscription's isolated two-path checkpoint separately passes 92 short
  tests/13 packages, vet and the same 2 PostgreSQL regressions, and is pushed
  as `d51d5b6bc`; no unrelated staged bindings entered it.
  The next run stopped at CreateSpace HTTP 500 after container replacement:
  Space's ordinary Role connection remained on its old Docker IP, while
  the protected Role client had resolved the new address. Only the owned
  Space consumer was restarted to refresh ordinary connections. This is an
  open recovery concern; steady-state acceptance cannot satisfy that gate.
  The following public attempt creates/invites/reads successfully and persists
  Role's actual fence receipt, then remains pending before Chat. A test-owned
  diagnostic confirms the aggregate reloads and constructs the Chat request.
  Chat's protected listener requires request_id equal deletion_operation_id,
  while the generic caller derived a separate namespace UUID. A new outgoing
  metadata regression fails before repair; the caller now preserves the
  business operation ID without relaxing listener checks or token replay.
  The actual combined flow still needs another run after the Space rebuild.
  The rebuilt generic caller passed focused metadata tests, Space's 353 short
  tests/15 packages and vet, with the same reviewer's clearance. The next
  public run recorded six receipts, then exposed Search manifest compatibility:
  zero-count rejection, source/global-root equality, and producer digest drift.
  Search now imports the canonical empty/nonempty Chat source independently
  while persisting and rechecking its authenticated Space root. Migration
  000010 retains both bindings, backfills only the previously enforced equality,
  refuses evidence-dropping rollback, and gates enabled startup. Independent
  zero/one golden regressions fail before and pass after repair; actual
  PostgreSQL restart/root-conflict/purge/replay/unrelated-document preservation
  and rollback refusal selection passes 3 tests, prior participant selection
  passes 17, short suite 151/11 and vet pass. The same reviewer cleared this
  batch. Only owned search_db received the migration (no prior rows), and local
  Search image `sha256:01db44d6eecd72a85591faad6be060ae065b0fae51135211b4b8f6bc55ee06a5`
  started healthy. Public run space `de4f7f29-0c40-452b-aba3-e66c1f0429cd`
  persisted participants 1–8; deletion stayed pending before Bot's handler.
  The manifest repair is now a dependency-closed nine-path checkpoint
  `fb91bf9a1`, independently built with the affected Search gRPC short suite,
  full module vet, and actual PostgreSQL empty/nonempty source/root restart,
  conflict, purge, exact replay, unrelated-index preservation and rollback
  refusal evidence. Unrelated managed-chat runtime/fixture edits remain WIP;
  only the root-binding main/docs/fixture portions entered the checkpoint.
  All 26 inherited generated-binding staged paths were preserved.
  That run exposed Bot's lifecycle interceptor selecting the generic GIS
  verifier instead of the separate Space verifier. The interceptor now requires
  `VerifySpace`; its regression stub rejects the same credential through
  generic `Verify`, preserving the issuer boundary. Focused principal transport
  and runtime tests and Bot vet pass; the same reviewer cleared the repair.
  Only the owned Bot container was rebuilt to image
  `sha256:abeec0c2d56f6b0e880b2c3adb741c18b291dde021fd74f0fcd102e24b360b5f`.
  The next actual Gateway-to-Space Compose test passed through all ten
  participants: regular owner/member/outsider reads, authenticated deletion
  proof, freeze, the owner's exact minimal frozen projection, nonowner denial,
  restore, exact retries and restored member visibility. This is an empty-chat
  steady-state freeze/restore slice on the owned stack. A new expired-deletion
  test first performs fresh public admission/freeze and verifies the seven-day
  deadline, then advances only that exact disposable aggregate's mutable dates
  with a guarded Space/operation/phase/generation SQL fixture. Original Auth,
  participant, File and outbox evidence remains untouched; the schedule event
  retains its original deadline, so no real seven-day wait is claimed. The
  production worker completed all ten generation-2 fences and all ten purge
  receipts, physically removed Space and retained the minimal tombstone. That
  first run exposed restore-after-purge returning HTTP 500: missing rows,
  nonowner and invalid lifecycle states fell through the generic internal-error
  mapping. A negative regression failed four cases before repair; scoped
  lifecycle mapping now returns NOT_FOUND, PERMISSION_DENIED and
  FAILED_PRECONDITION respectively, preserving fail-closed DB errors. Focused
  handler tests and vet pass. Owned Space image
  `sha256:121eac7f2d712a02932daea49213b2ed4985ea04a14f96616bb6dbfe54f074d6`
  was rebuilt/replaced; both public freeze/restore and expiry/purge tests pass,
  including post-purge GET/restore 404. The same independent reviewer cleared
  the fixture scope, status mapping and preserved immutable evidence.
  Nonempty messenger flows, explicit container-replacement fault recovery and
  the broader T40 matrix remain open. Some runtime images precede the master
  merge and must be rebuilt
  for its affected inputs before final combined runtime acceptance.
  Latest nonempty continuation: Chat, Messaging and Gateway were rebuilt from
  the combined master/WIP inputs and started healthy. Gateway's current random
  loopback port is 56040 (the earlier 54389 is stale); mail fixture remains
  54436. The public fixture now creates a real Space Chat/message and a separate
  control Space. Its tree decoder uses the canonical nested `linked_chat` ref.
  Nonempty freeze/restore passes and preserves the original message identity
  and content. Nonempty expired purge fails after 129.77 seconds: Space
  `bbdce4db-605e-4762-ac2b-4709bb219aa5`, operation
  `e2a04592-fede-479e-8d76-8722511c03fa`, remains `PURGING` at generation 2 with
  all ten fence receipts and only the Role retirement receipt. Messaging is
  the next purge participant and its nonempty child cleanup is the next
  investigation; no specific cause is established yet. Do not treat the
  earlier empty-manifest purge pass as nonempty acceptance. The exact binary
  run completed (1 passed, 1 failed); no test process remains active.
  New active image IDs: Chat
  `sha256:7082a090174f117b167f71795c1639f3b11ad04323ab67e82889584f6c179d40`,
  Messaging `sha256:393e79f8bcf2ae499b5861e6171560e97bc8ac9b1ce24e37c09097396c37e7f3`,
  Gateway `sha256:066b47796821361b9abd334d08ab55b57ed3e5c3d6af31038a08d5cd09f31777`.
  The primary writer stopped implementation for the owner's requested handoff.
  Master synchronization used a retained named stash plus hashes for all 1595
  inherited WIP paths and a staged patch. All original paths were restored;
  the 26 generated Dart paths remain the sole staged set. Manual resolutions
  preserve A1 chat-bound history cursors and block policies alongside GIS
  entitlement/consent checks, request-inbox discovery and lifecycle wiring.
  Generated Go/Dart bindings were rebuilt from the combined contracts. On the
  committed merge, Chat/Messaging/User/Voice compile, buf lint and 63 inbox
  Flutter tests pass. With restored WIP, Messaging short 412/16, Gateway 687,
  Realtime's complete short binary run, all three vet checks, 76 focused
  Flutter tests, scoped analyzer, buf lint and diff check pass. The same
  reviewer cleared manual compatibility resolutions. This does not replace
  the remaining PostgreSQL and final consolidated sprint gates.
  Following repeated Windows firewall prompts, owned Go tests can use the
  fixed-path runner under ignored `tmp/test-bin`; local-only inbound TCP rules
  are prepared in `Allow-LocalTests.ps1` for user execution as administrator.
  No firewall settings were changed by the agent. The pre-sync stash and
  `tmp/master-sync-20261002` recovery inventory remain retained.
  Only the named local Docker Desktop Compose project is rebuilt/restarted;
  no staging endpoint, deployment or A1 resource is used.
  Component PostgreSQL containers and
  JetStream servers were test-owned and cleaned up. A1 resources remain
  untouched. The bounded Graphify attempt stalled after extraction and was
  stopped; diagnostics remain under `tmp/graphify-continuation-20261002` and
  graph refresh is unverified. Master merge, staging, live providers, physical
  devices and the unavailable Windows-host iOS simulator remain excluded.
  After the master sync and current code repairs, one fresh Graphify attempt
  reached its 45-second bound; only its owned process tree was stopped.
  Diagnostics are retained in `tmp/graphify-after-sync-20261002`; graph freshness
  remains unverified and repeated stalled runs are not used as sprint evidence.

Each `T-*` is a reviewable work item, generally one PR or a narrow cluster of
related PRs. In each behavior PR: freeze docs/contract, write failing
contract/integration test, implement, run affected checks, update status and
acceptance evidence. `D:` lists hard predecessor tasks. Cross-service API
contracts land before consumers; activation lands after all consumers.

### P0 — base, queue, decisions, contract seams

- [x] **T00** `D: —` Rechecked the current feature target, exact implementation
  base, dirty root checkout, open game PR and active worktrees. The target for
  this isolated Auth slice is `codex/game-sdk-federation-docs` at
  `7dd013ce551867074ab148dabaedfd30c1f83478`; root `master` contains separate
  dirty user WIP and was not changed. PR #550 (`codex/t31-gis-orchestration-red`)
  remains active on the same feature base; its owner worktree has changes only
  in T31 orchestration/acceptance paths, and no ExecPlan diff. The Auth owner
  uses an isolated Treehouse slot. This inventory is time-scoped to that SHA.
- [x] **T01** `D: T00` The owner-authorized parallel game sprint decision and
  federation activation scope are recorded in [PLAN](../PLAN.md), retaining
  A1's status and gates, with a separate game queue and rollback owner and a
  hard hold on master merge/staging until A1 acceptance. Recorded in PR #552,
  now included in implementation base `7dd013ce551867074ab148dabaedfd30c1f83478`.
- [x] **T02** `D: T00` Re-audited exact base
  `7dd013ce551867074ab148dabaedfd30c1f83478`; evidence is source/test presence,
  not a claim that every listed suite ran on this SHA. The seven capability
  areas are partial, not absent end-to-end:
  - Bot proof exists in `src/backend/bot/internal/gameintegrationproof/handler.go`
    with `handler_test.go`; durable event/command/result completion remains
    open under T50/T51 and must not be inferred from this proof helper.
  - Auth identity/conversion groundwork exists in
    `src/backend/auth/src/main/java/voice/backend/auth/sdkidentity/` and
    `src/backend/auth/src/test/java/voice/backend/auth/sdkidentity/` including
    `GoogleOidcProofVerifierTest`, `SdkIdentityJdbcIntegrationTest`, and
    `SdkConversionJdbcIntegrationTest`; account conversion/authority receipts
    and live provider acceptance are not complete.
  - Chat/Space managed-chat groundwork is in
    `src/backend/chat/internal/grpcsvc/gameintegration_chat.go` and
    `src/backend/chat/internal/store/managed_chats_integration_test.go`;
    full game roster/roles and lifecycle vertical remains open.
  - Voice game principal/session scaffolding is in
    `src/backend/voice/internal/gameprincipal/` and
    `src/backend/voice/internal/grpcsvc/game_session_provisioning.go`, with
    `interceptor_test.go`, `game_session_contract_test.go`, and
    `gameprovision/store_integration_test.go`; native media/revoke acceptance
    remains open.
  - Federation authority and store scaffold are in
    `src/backend/federation/{authority.go,api.go,store.go}` with
    `authority_test.go` and `store_integration_test.go`; node consumers,
    provisioning and media enforcement remain open.
  - Gateway SDK route classification exists in
    `src/backend/gateway/sdk_authorization_routes.go` with
    `sdk_authorization_routes_test.go`; routes remain gated from public
    activation.
  - Flutter SDK authorization client/state/UI exist in
    `src/frontend/lib/backend/sdk_authorization_client.dart`,
    `src/frontend/lib/state/sdk_authorization_providers.dart`, and
    `src/frontend/lib/ui/sdk/sdk_authorization_screens.dart`, with
    `src/frontend/test/sdk_authorization_client_test.dart` and
    `sdk_authorization_widget_test.dart`; this is authorization UI, not a full
    game-integrations client.
- [x] **T03** `D: T01,T02` Choose technical defaults for G01–G13 and Q01–Q12
  in owning docs before dependent implementation; assign every value/algorithm,
  test and consumer. The owner has delegated these choices; escalate only a
  genuine product contradiction or unavailable external dependency. See the
  decision checklist below.
  - [x] T03 command delivery, v1 envelope/HMAC, retry/deadline, restart and
    Q05 risk-class defaults frozen in the linked API/feature docs.
  - [x] T03 permit epoch frozen: first committed admission mints
    `permit_issued_at`; t=15/t=31 first delivery may obtain a fresh epoch while
    authorized/unexpired; permit retries preserve one epoch; revoke shares its
    serialization point. T07a receiver permit/revoke and completion-window
    proof is complete; GIS online admission integration remains in T55.
  - [x] T03 G05/G06/Q11 decisions link to the existing `/api/v1`, Federation
    `/v1`, Google OIDC, GIS owner/operator bootstrap, and Federation mTLS
    contracts. Q11 clean-start, cross-scope negatives, fake-vs-real provider
    evidence are specified in acceptance; the API-only clean-start proof passed
    at feature commit `165a11e`. The real-Google/provider proof remains open.
  - [x] T03-Q11-FED: Federation HTTP request-ID normalization and the bounded
    Q11 denial-audit fields, append-only storage, and TLS-handshake exclusion
    are frozen in [Federation authority v1](../architecture/federation-authority-v1.md)
    and [Q11 acceptance](game-integrations-acceptance.md). The Q11 clean-start
    Federation HTTP path passed at `165a11e`; this closes no broad T03/T04 trust
    matrix or wider Federation runtime gate.
  - [x] T03 Q12 provisional one-host capacity/RPO/RTO qualification targets and
    restore/load measurement method recorded in the design audit. These values
    are proposals only; T08/T93 must replace them with measured evidence.
  - [x] T03-AUTH: freeze only the Auth identity proof rules in
    [GAME-AUTH-01](../architecture/game-integration-api.md#замороженный-auth-identity-slice-game-auth-01):
    Google OIDC + separate app/env game ticket, nonce/freshness, device proof,
    configured app/env admission, and no Gateway publication. This closes no
    conversion, transfer/recovery, or cross-service G01 decision.
  - [x] T03-AUTH-LIFECYCLE: the Auth identity/conversion defaults for Q03,
    Q07, Q08 and Q10 are recorded in the same owning API contract: no inferred
    subject transfer; one private app-scoped actor before conversion; explicit
    target profile and no profile-limit bypass; source Voice session fenced
    before conversion with target conflict/handoff; non-expiring pseudonymous
    anti-resurrection tombstone floor. Their owning-service runtime and restore
    acceptance remain open; this closes only the decision prerequisites for
    the Auth identity/conversion consumers (T13, T17–T19).
  - [x] T03-G02/Q03: one active character per app/env/sdk identity by default;
    opt-in multiple characters have independent bindings, consent, and grants;
    ownership never transfers without an authoritative provider signal. Tests:
    Q03/MMO02; consumers T17–T19, T37–T39.
  - [x] T03-G03/Q09: Owner loss/dissolution freezes privileged writes until a
    Voice operator verifies a named human and records a new owner generation;
    game leaders/node operators cannot assign Voice ownership. Personal block
    does not mutate membership; Voice ban remains authoritative. Tests Q09/MMO03;
    consumers T37–T39 and moderation.
  - [x] T03-Q02/G11: scope, Owner, operator, or destination changes increment
    consent revision and revoke old authority; queued actions are cancelled or
    revalidated, while admitted work follows G13. Notifications dedupe by
    recipient/category/source event and recheck consent after quiet hours. Tests
    Q02/BOT09; consumers T14, T58–T59.
  - [x] T03-G07/Q12: preserve the implemented 120 registration attempts per
    app/UTC-minute cap; all other sandbox capabilities require bounded quotas
    and size limits before enablement. Production, pricing, and SLA remain
    disabled pending T08/T93 measurement. Tests OPS02/Q12; consumers T08,
    T11–T12, T76, T93.
  - [x] T03-G08/Q06: canonical Federation authority contract sets the 500ms
    renewal, ≤2.0s publication or last-lease validity to node-side denial,
    ≤250ms uncertainty subtracted from expiry before monotonic conversion,
    ≤2.75s media enforcement, and ≤5.0s
    total revoke-to-eject; command drain retains the stricter 4.25s bound.
    Stale authority fails closed and revoke/freeze has reserved priority
    capacity. Tests FED02/FED03/Q06 under 2x qualified load; consumers T70–T78/T93.
  - [x] T03-G10/Q10: v1 node home is immutable; no online cross-node migration.
    Operators own encrypted backup/hardware recovery; Voice owns identity,
    consent, generations, and revoke ledger. Restore merges current Voice fences
    before serving; exports exclude secrets and unconsented personal data.
    Tests FED06–FED08/Q10; consumers T70–T78/T76.
  - [x] T03-G12/Q07: expose only app/env-scoped opaque profile references and
    user-selected NFC aliases (64 Unicode scalar values max); hidden profile
    fields require separate consent and app policy. Enforce profile caps across
    login/conversion and every public payload. Tests Q07/ID cases; consumers
    T14, T17–T19, T38–T39. Full adopted defaults and acceptance cases are in the
    [T03 cross-cutting API freeze](../architecture/game-integration-api.md#t03-cross-cutting-defaults-g02-g12-and-q02-q12).
- [x] **T04** `D: T03` Freeze trust matrix for game service, player, bot and node;
  principals, issuer/audience, scopes, credential storage, expiry, rotation,
  revoke, rate limit and direct-service negative cases.
  The executable cross-authority and SQL-tamper cases are listed in the
  [T04 trust-matrix acceptance](game-integrations-acceptance.md#t04-principal-trust-matrix).
  The principal matrix, including the 90-day absolute `vgi1` TTL and atomic
  T11 HMAC replacement rule, is frozen in
  [Game Integration API](../architecture/game-integration-api.md#t04-principal-trust-matrix).
  Contract and implementation verification are complete: PR #562 is merged;
  exact-head hosted CI and the full rollup passed at merge
  `865210494e5056ce38240272678ef6f9f21778a9`. This evidence closes T04 only;
  it does not close dependent runtime gates.
  - [x] T04-AUTH: the Auth-only provider/ticket issuers, audiences, key sources,
    proof binding, storage boundary, freshness and replay rules are frozen in
    GAME-AUTH-01 for T13a. Google proof and app/env game-ticket proof are
    independent; neither a developer credential nor `guest` conversion can
    establish player identity. T13a verifier, SQL and route tests exercise the
    bounded rules. This Auth-only slice remains unchanged by T04.
- [x] **T05** `D: T03,T04` Freeze resource/state model and ownership: account,
  selected profile/alias, character, app/env/installation, binding, party,
  match/fleet, corporation→Space, grant reasons, operations and tombstones.
  The target ownership and lifecycle model is aligned across DATA_MODEL,
  DATA_STORES, and CONTRACT_MATRIX against adopted T03/T04/G03/G04/G09/G12
  decisions. This closes the ownership-contract milestone only; schema and
  runtime implementation remains with T30+ consumers, and the T51 binding-event
  chat-selection rule remains explicitly open.
- [x] **T06** `D: T04,T05` Freeze and verify the contract package for the
  currently Gateway-published Game API. The route-aligned OpenAPI artifact at
  `docs/contracts/game-integrations-t06.openapi.json` and executable
  conformance coverage in `src/backend/gameintegration/internal/httpapi/t06_contract_test.go`
  cover all 14 currently published GIS/T31 operations, including request and
  response schemas, canonical IDs, idempotency, error mappings, and principal
  security scopes. The artifact resolves 137 component references. This closes
  only the current published-contract package; generated-code compatibility
  remains under existing CI gates, and unpublished/undecided surfaces remain
  explicitly recorded in the contract artifact. T16 producer/hosted acceptance,
  T31 consumer conformance, and T51 binding-event ingress remain owned by those
  open tasks; Auth routes remain unpublished.
  - [x] Command/result JSON v1 routes, envelope canonicalization, status/error,
    idempotency and receipt contract frozen in `game-integration-api.md`.
  - [x] T06-AUTH: Auth challenge/exchange/session/revoke paths, request and
    response fields, device proof bytes, credential semantics and denial policy
    are frozen in GAME-AUTH-01 for T13a. Gateway/OpenAPI publication, other
    resource APIs, generated contracts and executable compatibility checks
    remain open. Auth routes are still opt-in and unpublished from Gateway.
  - [ ] Other OpenAPI/proto/resource contracts and executable compatibility
    tests remain open; this docs decision does not claim routes are implemented.
- [x] **T07a** `D: T03,T06` Build controlled backend in a separate test-only Go
  module and dedicated test DB: durable command inbox/outbox and transactional
  effect/result receipt as specified in the v1 contract. Never register it in
  production GIS or use `game_integration_db` for game effects. Version and
  retain deterministic fixtures. Its internal test adapter is not a GIS HTTP
  contract: `PermitAuthority.Admit` receives command/operation IDs, exact
  app/environment/installation IDs, opaque actor-proof profile/proof IDs, and
  binding revision; it returns a stable permit ID and integer Unix-second
  `permit_issued_at`, or distinct denied/unavailable errors. A fake authority
  returns one permit per command ID, serializes new issuance against revoke
  (revoke-first denies; permit-first preserves only that permit's fixed window),
  and returns the same permit on retry/lost-response without extending it or
  reminting after the start bound. Start and completion bounds are exclusive at
  `permit_issued_at+10s` and `permit_issued_at+60s`; a missed start, denied or
  unavailable admission produces no game effect. The receiver persists the
  permit receipt with the command, runs a transaction-scoped local authorization
  check before the game mutation, and commits effect plus immutable result/outbox
  only before the completion bound. The local check binds the signed
  app/environment/installation and opaque actor proof to the game-owned profile,
  character and current binding revision, and requires the command's state
  version to match. Test fixtures model this mapping without defining a proof
  format or making a live GIS call. Testcontainers PostgreSQL is per-test and
  isolated; its receiver-owned schema is initialized only inside that database.
  This vertical is the durable receiver foundation consumed by T56; the GIS
  authority client and transactional game-owned profile/character/state checks
  are implemented and accepted under T55–T56.
  - [x] T07a local proof: `TestT07aPermitAuthoritySerializesIssueAndRevoke`,
    `TestT07aCallbackPersistsPermitAndImmutableReceiptThenReplays`,
    `TestT07aAuthorizationDenialAndAdmissionFailuresHaveNoEffect`, and
    `TestT07aOneShotEffectCannotBeConsumedByDistinctCommandIDs`,
    `TestT07aPermitStartAndCompletionBoundsAreExclusive`, plus
    `TestT07aCompletionDeadlineCheckedAtTransactionCommit` pass; the four
    callback integration tests use isolated PostgreSQL 16 containers. The
    receipt test rejects UPDATE, DELETE and TRUNCATE while preserving committed
    rows; the final pre-commit check rejects a deadline crossed during receipt
    persistence. Run from `src/backend/controlledgame/`:
    `rtk go test ./... -run '^TestT07a' -count=1`, then
    `rtk go test ./... -count=1`, `rtk go vet ./...`, and
    `rtk golangci-lint run ./...`. PR #508 merged after a separate Codex
    exact-head review returned CLEAR and hosted checks passed. Broad
    T03/T04 parents remain open; T06's current published-contract package is
    complete, while dependent T16/T31/T51 execution gates remain open under
    their owning tasks.
- [x] **T07b** `D: T03,T06,T15` Build independent protocol client with per-device
  keys only after T15 key proof/registration semantics are implemented.
  **Development evidence:** added the isolated test-only Go module
  `tests/sdk-protocol-client`: ephemeral independent P-256 key generation,
  public RFC 7638 JWK/thumbprint, RFC 8785 payload serialization for the T15
  ASCII claim profile, ES256 compact JWS signing and typed HTTP calls to Auth
  public challenge/rotate/recover/revoke routes. Route fixtures verify the
  exact methods/paths/JSON names; signatures are checked independently with
  Go's ECDSA verifier; redirect forwarding is denied. `rtk make
  sdk-protocol-client-acceptance` passed (11 tests); `rtk go vet ./...` passed
  from the module. This is test-only development evidence: no provider calls,
  Auth deployment, real device, production key, or live-provider acceptance.
- [ ] **T07c** `D: T03,T06,T36,T40` Build real-media clients only after the P2
  LiveKit/media and public-client acceptance paths exist. These are test
  infrastructure, not a public SDK.
- [x] **T08** `D: T03` Define the measurement record and harness for revocation,
  roster freshness, lease/skew/eject, retry/retention, restore RPO/RTO, and
  qualified-host load. For each item keep the contractual target, provisional
  qualification profile (if any), method, and observed result/status separate.
  No host SKU, general GIS route threshold, new latency/capacity/cost/SLA target,
  or measured Q12 result is adopted here. T03 freezes the measurement inputs;
  T08 supplies the evidence contract and T93 consumes it for runtime evidence.

### P1 — developer bootstrap and player identity

- [x] **T10** `D: T04–T06` Implement Game Integration Go service module,
  service-owned DB/migrations, health/metrics, config/secrets, deploy wiring,
  internal auth and least-privilege credentials. The Compose initializer and
  dev/CI migration helper share idempotent provisioning for a distinct GIS
  runtime login, and the GIS runtime URL uses it.
  PostgreSQL integration proves that credentials loaded through GIS config can
  insert GIS registry rows while an insert into `auth_db` is denied with
  SQLSTATE `42501`; it also checks restricted role attributes and default table
  and sequence grants for later migrations. No global `PUBLIC` ACL is changed.
  See the GIS service contract and `runtime_database_access_integration_test.go`.
- [x] **T11** `D: T10` Complete the developer registry lifecycle in development.
  The implementation covers owner-derived application creation, separate operator
  sandbox approval, app/environment-scoped policy and installation
  configuration, provider, redirect/origin and callback configuration, and
  credential issue/retry, rotation and revoke. Before enabling `game.events.write`,
  each event-enabled installation must also be bound to a live Bot ID owned by
  the application owner. Empty-database API bootstrap
  and cross-scope denials pass without direct SQL or a portal. Its T10 database
  isolation prerequisite is now proven. Development lifecycle acceptance is
  complete. The staged operator route creates only a pending production
  environment and owner policy; it cannot activate production. The runtime
  gate remains OPEN / NOT RUN: production activation, live provider/user proof,
  and out-of-band production secret provisioning are outside this development
  scope and are not claimed. The seeded-fixture test denies production
  credentials and makes no claim of production onboarding. See the GIS service
  contract and
  [Q11 T11 evidence](game-integrations-acceptance.md#q11-bootstrap-evidence-api-only-clean-start-passed-real-google-gate-open).
  The sandbox installation authority slice is implemented: registration takes
  a Bot ID, derives owner only from the application row, and persists the
  binding only after a protected, replay-resistant GIS→Bot owner/live proof.
  BOT11 acceptance covers the exact request/response authentication, owner and
  lifecycle denials, durable binding/idempotency, and clean-bootstrap flow.
  Proof failure may write a sanitized denial audit, but never an installation
  or successful idempotency result. This development completion does not close
  the runtime gate above.
- [x] **T12** `D: T11` Add app-scoped quotas, suspension, diagnostics and
  provenance audit; deny cross-app/env IDs and SSRF destinations. Distinguish
  provider admission from developer assertion.
- [ ] **T13** `D: T04,T06` Implement Java Auth `sdk-account` type and schema,
  independent provider proof verifier (real chosen provider plus fake),
  issuer/audience/nonce/replay checks, admission cap and app/env-scoped account
  uniqueness. Never route via `guest`/`ConvertGuest`. Parent remains open for
  the full identity vertical and its broader acceptance.
  - [x] T13-DEV: exact-base Auth implementation has a distinct `sdk-account`
    principal/schema, fixed-Google OIDC verifier plus separate app/env game
    ticket, nonce/freshness/replay and issuer/audience checks, app/env
    uniqueness, configured cap, device/session/revoke lifecycle, and current
    signed GIS policy admission. On `7dd013ce551867074ab148dabaedfd30c1f83478`,
    focused Auth identity/conversion suites passed 169 tests and the full Auth
    Maven suite passed 890 tests, both with 0 failures/errors/skips. This is
    fake-provider development evidence only. Keep parent T13 open until the
    separately required real-Google exact-release proof and production
    admission gates pass; those were not run in this environment. Surefire
    emitted a post-test fork-JVM cleanup warning after `System.exit(0)`; Maven
    still exited 0 with BUILD SUCCESS.
- [x] **T13a** `D: T03-AUTH,T04-AUTH,T06-AUTH` Implement the bounded Auth-only
  GAME-AUTH-01 foundation in Java: `sdk-account` schema/principal, paired Google
  OIDC and app/env game-ticket verification, challenge/device proof, replay and
  freshness checks, operator-configured app/env admission, cap of 1,000
  identities per app/env and 10 active devices per identity, uniqueness,
  bootstrap/session/revoke. Deterministic fake-provider tests pass; Auth routes
  remain opt-in and are not published in Gateway. Do not integrate GIS
  registry/Gateway or implement conversion, browser/PKCE, User profile
  selection, per-device actor keys, recovery, or wider G01 trust policy.
  Consumer: remaining T13/Auth identity work and the later T14 authorization
  flow. Required acceptance is the T13a
  module suite described in [Q11](game-integrations-acceptance.md#q11-bootstrap-evidence-api-only-clean-start-passed-real-google-gate-open);
  the separate API-only clean-start gate passed at feature commit `165a11e`;
  the real-Google gate is OPEN / NOT RUN, and production admission remains open.
  T13a module and
  full Auth Maven checks passed on feature base `df0e084383ce1f9915d0bbc59651ae0227b1c75d`;
  see the exact counts and scope in [Q11 evidence](game-integrations-acceptance.md#q11-bootstrap-evidence-api-only-clean-start-passed-real-google-gate-open).
- [x] **T13b** `D: T10,T13a` Gate Auth SDK identity challenge, exchange, and
  session admission on a fresh signed GIS environment policy with exact
  application/environment IDs, positive revision, and Google provider. Share
  the existing signed GIS policy client across the opt-in Auth identity and
  authorization configurations; retain Auth-owned operator client ID and game
  key configuration. Missing, mismatched, malformed, empty-provider, or
  unavailable policy fails closed. Full Auth Maven passes 848 tests with 0
  skipped (including PostgreSQL/Testcontainers); focused T13b/Auth integration
  suite passes 154 tests with 0 skipped; Q11 API-only clean-start acceptance
  passes. This closes only the T13b admission slice: parent T13 remains open for
  its broader identity vertical and real-provider acceptance gate.
- [x] **T14** `D: T13-DEV` Add browser/device authorization, PKCE, explicit selected
  profile/scopes, consent revisions, bindings challenge/exchange, returning
  login, no account enumeration; unauthenticated game ticket cannot mint a
  player token. Add device-code route only if required by frozen capabilities.
  Auth's request-bound `service:auth` principal and User profile eligibility
  adapter are one security prerequisite for this vertical and are implemented
  with focused tests in this slice; hosted checks and exact-head review remain
  required before merge. Their accepted
  keyset/transport contract is in [Auth Service](../microservices/auth-service.md#t14-auth-to-user-selected-profile-authority-accepted-target-signerclient-implemented)
  and [Deployment](../DEPLOYMENT.md#auth-to-user-sdk-profile-principal). At
  approval, validate exact response IDs and positive revision, deny deleted or
  frozen profiles, and persist `profile_revision`. At exchange, re-read and
  require the same eligible IDs and revision; otherwise deny with existing
  `invalid_sdk_identity` and require a fresh authorization. No new external
  error code is introduced. This completes only the Auth→User prerequisite;
  the broader T14 browser/device authorization work and operational rollout
  remain pending. The development vertical is complete on the scoped T13-DEV
  prerequisite: T14-DEV's exact-head Auth/GIS/Flutter evidence is supplemented
  by `SdkIdentityJdbcIntegrationTest.providerEmailAndDisplayClaimsNeverMergeOrSplitSdkAccounts`
  and `SdkAuthorizationJdbcIntegrationTest.unknownAndRevokedLinkedCredentialsHaveTheSameCoarseDenial`.
  The focused Auth run `rtk mvn -B '-Dtest=SdkIdentityJdbcIntegrationTest,SdkAuthorizationJdbcIntegrationTest' test`
  passed 100 tests with 0 failures/errors/skips. T14 still does not publish the
  routes (T20 owns Gateway activation); live Google/provider and production
  acceptance remain open, and conversion-specific email/nickname conflict
  behavior remains under T17/T18.
  - [x] **T14-DEV** `D: T13-DEV,T10,T04-AUTH,T06-AUTH` Close the bounded,
    provider-independent development vertical for explicit Voice profile and
    scope consent: Auth authorization request, PKCE callback/exchange and
    returning linked-session; current User profile eligibility/revision check;
    and GIS persisted binding challenge plus one-use Auth-code exchange. The
    implementation is present at the feature base, with existing Auth, GIS,
    and Flutter test sources mapped in [T14-DEV acceptance](game-integrations-acceptance.md#t14-dev-source-and-test-map).
    Auth's full Maven suite passed 890 tests with 0 failures/errors/skips at
    `7dd013ce551867074ab148dabaedfd30c1f83478`; the current merged feature
    tree at `0a9d62994343763f173adcba361730e9c6b3f660` contains the same
    bounded Auth/GIS/Flutter slice. Exact-head hosted CI and Docs link check
    passed on PR #550 head `02b73018a8a94b07556147816bf04c51ad78fc40`
    (merged as this tree); Auth, GIS binding, Messaging, Flutter, and the
    independent exact-head review were green. This development subgate does
    not close parent T14, publish Auth
    routes through Gateway, prove real-provider behavior, or pass production
    admission; those gates remain open.
- [x] **T15** `D: T13-DEV,T14-DEV` Implement the frozen Auth-owned ES256/P-256 device
  key lifecycle, RFC 8785/JWS v1 message bytes, Auth RS256 status assertions,
  Messaging EdDSA tombstones, revision chain, File manifest and exact-receipt
  ordering from the API contract. Decision gate (enrollment/recovery/rotation/
  revoke, receiver verification, fail-closed clock/outage behavior and ≤4.25s
  node revoke ceiling) is closed; provider-independent conformance vectors and
  development receiver behavior are verified. Runtime and measured revoke
  propagation remain open. Auth authority issuance also consumes the
  T16 Auth/GIS execution-permit producer and must fail closed until it is available; never
  synthesize `binding_id` from another identifier. Messaging must additionally
  consume the T30/T31 exact app/environment/binding/chat resource mapping;
  Auth binding authority and Chat membership alone cannot prove that link. The
  owner producer and fail-closed consumer are implemented and have focused
  checks below. Each
  message requires a permit capped at `min(issued_at + 3750ms, assertion.exp)`;
  Messaging enforces a 250ms transaction maximum plus 250ms clock margin and
  records completion atomically with the message receipt. Auth/GIS revoke
  succeeds only after issued permits drain; unknown completion remains pending.
  - [x] **T15-DEV** `D: T13-DEV,T14-DEV` Verify provider-independent Auth key
    lifecycle/status assertions and the Messaging/File signed-message receiver,
    then add full RFC 8785 vectors for binary64 number rendering, UTF-16 key
    ordering, escaping, and invalid surrogate rejection. Auth-focused tests
    passed 64 tests; the pre-canonicalizer four-package Messaging receiver run
    passed 434 tests across `gameprotocol`, `s2s`, `store`, and `grpcsvc`; after
    canonicalizer changes the complete modified `gameprotocol` package passed.
    The exact current acceptance map and commands are recorded in
    [T15-DEV acceptance](game-integrations-acceptance.md#t15-dev-jcs-wire-conformance).
    Real-provider/runtime admission and measured node revoke propagation remain
    excluded or open; this subgate does not close the T16 or T30/T31 producers.
- [x] **T16** `D: T14-DEV,T15-DEV` Implement limited delegated grants, binding/authority
  The original `D:T14,T15` edge is resolved here against their completed
  provider-independent development subgates; real-provider/production evidence
  remains separately excluded and does not block this development behavior.
  reads, selected profile/alias serialization, direct-call scope checks and
  account/profile/app/device revocation epochs. Bind execution to the selected
  profile and current player-scope policy. Hidden-profile fanout through
  roster/cards/search/presence is owned by T38-T39 consumers. **Development
  implementation:** GIS
  has durable binding state, an Auth-workload-protected owner read, and a short
  operation-scoped permit ledger with revoking/drain semantics. Auth now has a
  dedicated handoff signer, opt-in private mTLS claim/completion routes, and a
  transactional claim ledger that rechecks grant/profile/policy/device
  authority and records idempotent GIS completion receipts. Auth now reads the
  persisted GIS challenge over WorkloadProof and consumes
  a challenge-pinned T14 approval code only on private GIS mTLS exchange; its
  PostgreSQL proof covers current device/profile/policy checks, exact JWS replay
  and handoff PoP pinning. GIS also has a strict Auth mTLS exchange client.
  GIS now has a regular-bearer exchange route that persists the canonical
  operation before Auth calls, online-claims the Auth handoff, atomically
  creates a pending binding plus completion outbox, and activates only after
  Auth completion receipt; focused handler and PostgreSQL lifecycle tests pass.
  T14 `approve` now creates the persisted challenge through the private Auth
  WorkloadProof route, after current source/profile/policy validation. GIS
  exact-operation replay returns the original challenge without extending its
  expiry; Auth exact approval retry returns the same encrypted one-use code and
  challenge receipt only while unconsumed/unexpired. Focused PostgreSQL tests
  cover exact retry, changed-profile conflict and consumed-code rejection.
  Gateway does not yet publish the API. The T16 Auth→Messaging execution-
  permit bridge is implemented on the feature integration branch: Auth issues
  and completes the short-lived permit; Messaging verifies it, checks the GIS
  app/environment/binding/chat mapping before permit issuance, and atomically
  stores the message receipt with the completion outbox. Messaging uses the
  private GIS lookup over mTLS plus WorkloadProof v1, verifies the response
  with the request's same key, enforces a 2-second timeout, and fails closed
  when the complete client configuration or mapping authority is unavailable.
  Scoped Auth and Messaging tests, including an ApplyGameMessage-through-client
  mTLS allow/deny test with local PostgreSQL, pass. **T16-DEV required a hosted
  Linux Compose gate** that starts the production Auth and GIS services
  with disposable PostgreSQL state and invokes Messaging's production gRPC
  ingress, exact principal verification, permit client, GIS mapping client,
  and receipt transaction. Synthetic mappings are seeded only in that
  disposable database and do not close T30/T31 producer gates. It records
  deny-before-permit/write cases, exact retry after response loss, message and
  completion receipts, revoke timing from the GIS binding's durable
  `active → revoking` commit to the last durable terminal state (`committed`,
  `aborted`, or `expired` after the 500ms drain margin) for every earlier
  permit. Endpoint request-to-transition and Auth grant's `revoking` timestamp
  are reported separately. It also covers
  relink under a fresh binding/assertion, including a read-only exact receipt
  retry after revoke and a distinct new-operation deny with old proof. The
  development proof gate passed. Restore's ≤5.0s monotonic target is
  a new provisional developer SLO from activated new binding plus current
  grant/consent to first accepted fresh-assertion operation; old binding
  proofs remain denied. The hosted Linux T16-DEV gate passed on code commit
  `e1fbe949e440f0ddfc2d1e5788b7af2f689901da`:
  [workflow run 36609945598](https://github.com/Poryadok/VoiceRoot/actions/runs/36609945598)
  completed successfully, including GIS-unavailable denial, GIS restoration,
  and integrated production-handler Messaging acceptance. Supplemental Auth
  PostgreSQL coverage then closed exact concurrent permit replay against
  revoke, direct-call current-scope denial, and selected-profile account/profile
  equality between current authorization and the durable GIS grant. The focused
  JDBC/controller/issuer run passed 65 tests with no skips; see the T16-DEV
  supplemental evidence map. Selected-profile/alias fanout and hidden-profile
  non-leak across roster/cards/search/presence are owned by T38-T39 consumers.
  The T30/T31 mapping producer and Messaging consumer now exist; focused checks
  passed with `rtk go -C src/backend/gameintegration test ./internal/httpapi
  ./internal/registry -run 'TestMessagingResourceMappingAuthorization|TestAuthorizeChat|TestResourceMapping' -count=1`
  and `rtk go -C src/backend/messaging test ./internal/s2s ./internal/grpcsvc
  -run 'TestGameResourceMapping|TestApplyGameMessage.*Mapping|Test.*ResourceMapping' -count=1`.
  Together with the hosted T16-DEV and supplemental Auth PostgreSQL results,
  this closes the specified T16 development scope. Client `game.chat.read`
  authorization remains T35 pending the player/session claim contract; hidden-
  profile fanout remains T38–T39. Real provider, live deployment and production
  admission are not claimed.
  - [x] **T16-DEV** `D: T14-DEV,T15-DEV` Complete the hosted Linux Compose
    cross-service acceptance described in [game-integrations acceptance](game-integrations-acceptance.md#t16-dev-cross-service-permit-acceptance).
    The production Auth/GIS services, GIS PostgreSQL handlers/store, and
    Messaging gRPC path passed on commit
    `e1fbe949e440f0ddfc2d1e5788b7af2f689901da` in [run 36609945598](https://github.com/Poryadok/VoiceRoot/actions/runs/36609945598).
    This hosted proof alone does not close T30/T31 producer work.
- [x] **T17** `D: T13–T16` Implement durable sdk→new-permanent conversion:
  operation/status, source proof, registration, preview/consent, authority
  freeze, owner receipts, grant recompute, new credentials, retired source
  reference. Retry every crash stage with same operation ID.
- [x] **T18** `D: T13–T17` Implement sdk→existing-permanent conversion:
  proof both identities, explicit profile, conflict preview, no implicit
  friends/role/history union, occupied binding reject/resolution, profile
  limit and sanctions. Ensure one active owner of binding at each stage.
- [x] **T19** `D: T17,T18` Close conversion races with voice/reconnect, old
  device/token, repeated provider login, unlink, deletion and backup restore;
  retain historical author without old access or cross-app identity leak.
- [x] **T20** `D: T11–T19` Expose versioned Gateway routes, errors and capability
  responses; run public client conformance plus forged subject/code/env,
  service-token-as-user, wrong device and direct gRPC bypass tests.
  **Development evidence for T17–T20:** durable conversion owner
  receipts/fences/tombstones, Voice activation fence, User/Auth orchestration,
  GIS activation callback and Gateway principal routes are implemented. Focused
  checks passed: Voice `go test ./internal/grpcsvc ./internal/gameprovision
  -run 'TestFenceSdkConversion|TestPostgresSdkConversionFence|TestGameSessionProvisioningContract|TestVoiceAdmissionGuard' -count=1` (6), Voice `go test .` (11), GIS `go test ./internal/httpapi ./internal/sessionowners ./internal/registry -run 'TestInternalConversionVoiceFence|TestPlayerBindingConversion|TestSessionOwner' -count=1` (4), Gateway `go test . -run 'TestSDKConversionGatewayPrincipalPolicies|TestSDKAuthorizationGatewayPrincipalPolicies' -count=1` (21), plus GIS activation callback tests (2). One `authority_epoch` legacy fixture mismatch remains queued for the final audit/repair batch. Live-provider and production admission remain excluded.

### P2 — session and managed-community verticals

- [x] **T30** `D: T05,T10,T16` Persist app/env/external resource keys,
  operations, request hashes and state transitions. Stable retries return the
  same result; conflicting body/revision returns 409; tombstoned key is fenced.
  - [x] Concurrent distinct operations targeting one app/environment/external
    key are serialized before the mapping lookup. The losing operation returns
    typed `ErrResourceMappingConflict`; only one row and receipt persist. The
    PostgreSQL regression holds the scoped advisory lock with an uncommitted
    competing row and waits for the exact lock tag, so removing the production
    lock makes the test fail deterministically. Focused registry tests passed
    (17), including three repeated race-test runs; independent review CLEAR.
  **Development evidence:** selected GIS operation/tombstone checks passed (8);
  stable idempotency, conflict and no-recreation tombstone behavior is covered.
- [x] **T31** `D: T30` Implement the normative durable orchestration contract in
  `game-integration-api.md` and `game-integration-service.md`: scoped JCS
  idempotency and read-only operation status; party-stable Chat, child-owned
  parent-party Chat reuse and self-owned Chat for an unparented match/fleet;
  durable Chat/Voice/Role operation IDs and receipts; Chat roster before Voice
  admission; forward recovery across lost replies/restarts; and an atomic
  active/outbox commit consumed through the app/environment-authenticated GIS
  HTTPS claim/ACK API by the game-server inbox. Accept
  only after durable GIS commit (≤2s with
  healthy GIS DB); reclaim expired work within 6s after restart when DB/owners
  are healthy; exponential retry from 1s capped at 30s. These are engineering
   bounds, not outage availability SLAs. Voice close is owner-idempotent and
   fences admission/media; Role grants have durable complete-set apply/revoke
   receipts. Close/revoke affects only the addressed session and never a
   referenced party resource. T31 includes new Voice close proto/server/Postgres
   fence and receipt/migration work, plus Role grant proto/server/ledger and
   migration work; neither exists at base
   `c31a8e0b771f99d00f00a760a2c6e2f7ec67cbcd`. Chat managed provisioning and
   roster sync already exist. Add a typed `SessionPrincipal` verifier and
   injectable GIS orchestration driver with a one-stage test/worker advance;
   drive Chat-create, Chat-roster, Voice, and Role receipt fakes independently
   and observe the GIS transaction that writes active state plus its outbox row.
  Operation responses freeze stable resource/receipt keys in the API contract.
  See T31 owner/service contracts and SE01–SE03.
  - [x] GIS binding↔Chat relation is published atomically only after the
    complete Chat roster receipt is accepted. GIS derives active same-scope
    bindings from the accepted session profile roster, stores the roster
    revision and 60-second lease, and revokes removed relations. Parent-owned
    Chat writes the party roster; child sessions reuse it without changing the
    relation set. RED/GREEN coverage includes lost Chat roster response and
    exact retry, T30 mapping retained before roster acceptance, scope/member
    filtering, and stale-worker lease loss rollback. Exact-head GIS registry
    tests: 82 passed; HTTP API: 94 passed; `go vet` and `git diff --check`
    passed. Independent review clear. The Compose test against actual Chat
    services remains `NOT RUN`: its required `DATABASE_URL` was not configured.
  - [ ] **T31 HTTPS event pull/inbox acceptance** — docs-first red-green plan:
    - Sources of truth: `game-integration-api.md` §T31;
      `game-integration-service.md` §T31; `game-integrations-acceptance.md` SE02;
      and T07a's controlled receiver boundary above. The receiver is the
      test-only controlled game backend in `src/backend/controlledgame/`, with
      an independent PostgreSQL schema and durable inbox/effect. It polls the
      GIS HTTPS API in isolated Compose and is not a public SDK product.
    - Receiver-owned paths: `src/backend/controlledgame/` pull client, inbox
      schema/migration, atomic effect and isolated DB tests. GIS-owned paths:
      `src/backend/gameintegration/internal/httpapi/` claim/ACK handlers and
      credential verification; `internal/registry/session_outbox.go` scoped
      claim/ACK store operations; `internal/registry/session_orchestration.go`
      one-time event body construction; runtime wiring; and the next migration
      under `src/backend/migrations/game_integration_db/`. Retire the T31
      JetStream publisher in `internal/sessionevents/jetstream.go` from this
      lane. `Store.ConsumeActiveSessionEvent` is not evidence because it writes
      to the GIS database.
    - Freeze claim as empty-body
      `POST /api/v1/session-events/claim`, using HTTPS and the existing
      app/environment credential with `game.sessions.manage`. Scope comes only
      from the verified credential. Filter app/environment before row locking
      with `FOR UPDATE SKIP LOCKED`; claim one undelivered row, with lease UUID
      and 30-second expiry stored per event in `claim_lease_id` and
      `claim_lease_until`, replacing rather than reusing the former relay-global
      lease fields. Return 200 with the exact body bytes and event ID,
      SHA-256, lease ID, and lease-expiry headers; empty is 204 with
      `Retry-After: 1`. Each re-claim after expiry gets a new lease and the same
       stored body bytes. Never expose master NATS credentials or streams.
     - Gateway implementation/test ownership: `src/backend/gateway/routing.go`
       and focused `src/backend/gateway/routing_test.go`. Add first-segment
       aliases `sessions`, `operations`, and `session-events` to the existing
       `game-integrations` upstream. Exempt only these exact GIS app/env
       credential routes from Voice-JWT validation, preserving the original
       `Authorization` header byte-for-byte for GIS on all published endpoints:
       `POST /api/v1/sessions`, `GET /api/v1/operations/{id}`,
       `POST /api/v1/session-events/claim`, and
       `POST /api/v1/session-events/{id}/ack`. Assert unrelated routes still
       require Voice JWT; verify absent, malformed, expired, revoked, or
       wrong-scope GIS credentials are rejected by GIS.
     - Public acceptance exercises all four endpoints using the configured
       HTTPS Gateway hostname and trusted certificate with hostname validation.
       Verify request/response headers survive the Gateway hop, no direct public
       GIS HTTP endpoint exists, and scoped credentials cannot claim, read
       operations, or ACK events from another app/environment.
    - Freeze ACK as
      `POST /api/v1/session-events/{event_id}/ack` with the same scoped
      credential and JSON `{lease_id,payload_sha256}`. First ACK requires the
      current unexpired lease and digest; atomically record `delivered_at`, ACK
      lease ID and digest, then clear claim lease. Exact post-commit ACK retry
      returns 200 idempotently; differing ACK data returns 409. Unknown/foreign
      event is 404, bad credential 401, insufficient scope 403, expired or
      superseded lease/digest mismatch 409. The receiver commits inbox key
      `(application_id,environment_id,event_id)`, SHA-256 of exact response body
      bytes, and game activation effect in one own-DB transaction before ACK.
      Same key+digest is a no-op; changed digest conflicts.
    - In the active transition, generate the exact seven-field UTF-8 JSON body
      once (`event_id`, `application_id`, `environment_id`, `session_id`,
      `operation_id`, `kind`, `active_at`), persist `payload_bytes bytea` and
      its 32-byte `payload_sha256 bytea` atomically with active/outbox. `payload
      jsonb` is inspection-only. Keep claim lease state per app/env/event, plus
      durable `consumer_ack_lease_id` and `consumer_ack_payload_sha256`. Backfill
      pending legacy rows with the frozen v1 encoder before API startup and
      requeue all rows whose old `delivered_at` meant only publisher PubAck.
    - Red: write failing tests first for (1) GIS constructs/persists the full
      seven-field body once and pull retries return byte-identical body bytes
      and hash; (2) two app/env principals cannot claim each other's rows and
      concurrent claims cannot lease the same row; (3) empty claim is 204 with
      `Retry-After`, lease expiry permits a new lease, and the event bytes remain
      unchanged; (4) ACK rejects wrong event scope, lease ID, expired lease,
      revoked/expired/wrong-scope credential, and mismatched digest without
      setting delivered; (5) same ACK after commit/lost response is idempotent;
      (6) receiver transaction rollback has no effect/ACK, same key+digest
      redelivery no-ops, and same key+different digest rejects. Then implement
      GIS body persistence/migration and app/env-scoped claim/ACK, receiver DB
      inbox/effect, and finally Compose HTTPS acceptance. Inject claim timeout
      after lease commit, receiver DB failure, lease expiry between commit and
      ACK, ACK timeout after GIS commit, and restart; verify reclaim, no duplicate
      effect, and eventual `delivered_at` only after valid ACK. Operation polling
      does not satisfy SE02.
    - Focused verification when implementation starts: `rtk go test ./...` in
      `src/backend/controlledgame/`; focused GIS registry and HTTP API tests in
      `src/backend/gameintegration/`; then isolated T31 Compose HTTPS pull/commit/
      ACK acceptance and failure matrix. Update SE02 evidence only from the
      independent receiver DB and authenticated pull/ACK path.
    - Hosted isolated Compose entrypoint:
      `.github/workflows/t31-session-events-e2e.yml` runs on `ubuntu-latest` for
      relevant pull requests or by `workflow_dispatch`. It generates Phase0/TLS
      fixtures, starts the app-profile owner graph and independent receiver DB,
      rebuilds Gateway from the exact worktree before bootstrap, then explicitly
      rebuilds/recreates only GIS with the operator allowlist. It bootstraps
      verified sandbox identities through public APIs. Before any claim, it
      builds one executable test binary into the per-run state volume and
      verifies the file; all four one-shot receiver processes invoke that same
      binary, so cold Go compilation cannot consume a claim lease. The first
      process loses the claim response after GIS commits its lease, persists
      the intercepted response's event bytes/hash/lease checkpoint, confirms
      an immediate follow-up claim is still held, and exits with that lease
      active. GIS restarts; the second process's first operation is an HTTPS
      claim before state-file or receiver-DB reads and proves the active lease
      survived. It then waits for expiry, reclaims identical bytes, forces a
      receiver effect insert failure and confirms both inbox and effect rolled
      back with no ACK, then commits the effect and delays ACK until GIS returns
      409 for the expired lease. GIS restarts while the receiver has a durable
      pending ACK. The third process reclaims with the already committed
      effect, GIS commits the ACK, and the runner loses that response. A fourth
      process proves exact ACK replay without a duplicate effect. No provider
      secrets are configured. The workflow saves only sanitized acceptance
      IDs/hashes/leases and tears down its unique Compose project. A passing
      hosted run is required before closing the HTTPS acceptance gate.
  - [x] Historical pre-HTTPS T31 Compose evidence on the exact base above:
    public app/environment/credential bootstrap; Party then parented Match
    activation after Chat, roster, Voice, and Role receipts; two stable
    JetStream active events under the earlier transport; child close with media fence and grant revoke;
    parent remains active; parent close and exact replay preserve receipts;
    both sessions finish closed with no grants. Evidence and resource/receipt
    IDs are recorded in `tmp/slave-driver/game-integrations-2026-09-27/STATE.md`.
    This historical evidence does not satisfy the HTTPS claim/inbox/ACK gate
    above; T31 active events now use the GIS HTTPS outbox API, with no JetStream
    game-server consumer lane.
    Voice close retry after persisted CLOSING was fixed test-first and verified.
  - [x] T31 retry bounds verified against PostgreSQL: healthy-DB acceptance
    completed under 2s; a restarted worker reclaimed the real five-second
    lease and advanced the stage within 6s; transient owner failures retained
    the same stage and stable owner request while per-stage delays followed
    1/2/4/8/16/30/30s, resetting after stage success. Focused evidence is in
    `session_retry_bounds_integration_test.go`.
  - [x] Authenticated consumer inbox and HTTPS restart/fault matrix passed on
    2026-09-28 in hosted run `36481450312` at exact PR head
    `a15fefc6d41e27534996f8dceb049ea47566d463`: scoped app/environment claims,
    lost claim response and GIS restart with an active lease, expired lease
    reclaim with identical bytes, receiver transaction rollback, commit-before-
    ACK lease expiry, GIS and receiver-process restart, lost committed ACK
    response, and exact ACK replay with one inbox/effect row. Sanitized IDs,
    event/payload hash, lease IDs, and row-count evidence are recorded in
    `tmp/slave-driver/game-integrations-2026-09-27/STATE.md`; unique Compose
    cleanup passed. Hosted HTTPS event pull/inbox/ACK restart-fault acceptance
    passed in run `36481450312` on `a15fefc6d41e27534996f8dceb049ea47566d463`.
    Real-provider/runtime gates remain open.
- [x] **T32** `D: T31` Implement stable party/match lifecycle, host transfer,
  explicit close, late join and roster freshness under the frozen contracts: a
  complete roster lease is 60s from GIS DB commit; retries do not renew; host
  transfer requires a complete next-revision CAS and existing successor; exact
  expiry denies admission/reconnect/governed reads+writes and fences Voice/media
  within 5s. A failed match cannot delete shared party chat. See API/GIS T32
  contract sections and SE03/SE04/SE07 acceptance. GIS registry+HTTP checks
  passed (36 tests / 2 packages), and Voice roster/session checks passed (14 / 3
  packages). Live runtime remains open.
- [x] **T33** `D: T31,T32` Implement per-member entitlement intervals for
  since_join/rejoin across message history, search, quotes, threads, files and
  signed download URLs; enforce retention and deletion policy. **Development
  progress:** Chat's `managed_chat_member_intervals` ledger and inclusive-join /
  exclusive-revoke store decision are implemented; internal
  `ChatService.CheckMessageReadEntitlement` is restricted to a single trusted
  Messaging/Search/File caller. Messaging history/message/thread reads, Search
  indexed-result filtering, and File message-reference metadata/URL issuance
  now route through the timestamp decision. File authorization-path test now
  proves the selector path checks entitlement before DB access. Focused verification so far:
  Chat RPC + interval/sync = 3 passed / 2 packages; Search entitlement = 1
  passed / 2 packages; File Messaging-to-Chat resolution + helper + selector
  authorization path = 3 passed / 2 packages; Messaging timestamp forwarding
  = 1 passed / 2 packages. Search missing-timestamp fail-open was reproduced
  with a RED test and fixed; `rtk go test ./internal/grpcsvc -run
  'TestFilterManagedHistoryHits' -count=1` passes (2 tests). Search now fails
  closed when an immutable timestamp is unavailable. File message URLs now use
  a short-lived HMAC capability bound to the authenticated profile, exact
  reference, variant and expiry. File rechecks Chat message-time entitlement and
  the live exact reference on every fetch; the PostgreSQL acceptance proves an
  issued URL stops working after reference release. Focused revocable-download
  checks passed 4 File tests and capability-path redaction passed 1 middleware
  test. T33 acceptance continued (2026-10-01): Search now has an API-path check
  proving a Markdown quoted snippet before the join timestamp is removed while
  an item exactly at the inclusive join boundary remains visible. Messaging now
  checks the same boundary for thread roots, replies and GetMessage, and filters
  every shared-media attachment category (image, video, document, other, audio,
  voice message and sticker) before requesting File metadata; a Chat entitlement
  outage prevents metadata lookup. Search Markdown quote snippets and Messaging
  thread roots/replies are checked at the same message-time boundary; the
  product has no separately referenced quote-message field, so Markdown quote
  content remains governed by its containing message. GIS retention signal and
  immutable retry are covered by
  `TestTerminalManagedMatchSchedulesImmutableChatHistoryCutoff` and
  `TestTerminalManagedMatchRetriesRetentionWithTheSameCutoff`; Chat cutoff and
  replay are covered by `TestManagedChatRetentionIsImmutableAndCutsOffHistoryAtDBDeadline`.
  Focused Search, Messaging shared-media, thread, message-read, GIS-retention and
  Chat-retention suites pass. Isolated Compose acceptance proves the frozen
  cutoff, File object deletion, Search tombstone, and final Messaging payload
  deletion under partial progress and lost-response retry. Live-provider/device
  gates remain excluded. The Search module also declares the generated Game
  Integration proto required by Messaging's expanded protobuf. Chat, Messaging,
  Search and File owner packages compile. Full Search gRPC suite passes
  (45.775s); full Messaging gRPC suite passes (518.974s) after the shared-media
  entitlement change.
- **T33 retention signal contract selected (2026-09-30):** Chat's dedicated GIS
  listener accepts an immutable `SetManagedChatRetention` receipt keyed by the
  GIS operation ID and exact absolute `purge_after`. Chat evaluates member
  history entitlement against its DB clock and denies at cutoff equality. GIS
  schedules only dedicated terminal match/fleet chats at the frozen
  `terminalized_at + 30 days`; Party and child sessions sharing Party Chat are
  excluded. Chat's RPC and immutable durable receipt, GIS's retryable terminal
  owner stage, and the mTLS owner adapter are implemented. Focused PostgreSQL
  checks passed for cutoff scheduling/retry, bound GIS principal, and immutable
  cutoff admission (`go test ./internal/registry -run
  '^TestTerminalManagedMatch(RetriesRetentionWithTheSameCutoff|SchedulesImmutableChatHistoryCutoff)$'`,
  Chat `internal/grpcsvc` bound-principal test, and Chat `internal/store`
  immutable-cutoff test). The GIS worker advances the same durable stage. A
  full GIS registry package run produced no output and was interrupted after
  about a minute; it is unverified. Physical Messaging/File purge receipts and
  cross-service acceptance remain open, so T33 is not complete.
- **T33 physical purge contract selected (2026-09-30):** GIS remains the
  durable coordinator and may begin content purge only after the immutable
  Chat `purge_after` cutoff. GIS sends an authenticated, request-hash-bound
  purge operation to Messaging; Messaging owns the message work set and
  durable receipt. Messaging freezes exact message IDs and File reference
  tuples before side effects, then requires idempotent File release and Search
  projection deletion receipts for that frozen set before physically deleting
  message payloads and completing its receipt. File's receipt must distinguish
  accepted reference release from completed object GC; shared blobs remain
  when another live owner reference exists. GIS advances its purge stage only
  after the immutable Messaging completion receipt. Every stage replays by
  stable operation ID and changed request bytes conflict. Chat entitlement
  remains fail-closed from the original cutoff while purge retries. Acceptance
  must cover before/equal/after cutoff, exact retry after response loss, changed
  replay, File/Search outage, partial progress, shared blob survival, and final
  absence of message payload/search documents/unreferenced object bytes. This
  is a technical owner protocol for T33 and does not alter retention duration,
  user consent, or Party-chat exclusions. Implementation follows in this
  dependency order: Messaging frozen work-set/receipt and failing PostgreSQL
  tests; Search and File exact-set receipts; GIS mTLS purge stage; then a real
  multi-service PostgreSQL/JetStream/ObjectStore acceptance.
- **T33 Messaging durable work-set stage (2026-09-30; incomplete):** Added the
  `MessagingService.PurgeManagedChatContent` protobuf contract and generated
  Go outputs. Messaging validates the canonical operation/chat identities and
  request-hash-bound GIS service principal, validates the owner response
  evidence, and exposes the method only on the dedicated GIS mTLS listener.
  Migration `000020_managed_chat_purge` and the owner store now freeze the exact
  message IDs and attachment File IDs at `created_at <= purge_after`, reject
  early starts using the Messaging DB clock, conflict on changed operation
  replay, and expose a completion transaction that stores 32-byte File/Search
  evidence alongside deleting that frozen set. PostgreSQL tests prove cutoff inclusion/exclusion,
  immutable replay after live-row mutation/deletion, changed-request conflict,
  completion evidence replay, and physical row absence (2 tests pass).
  Messaging principal/handler and listener-boundary tests pass (4 tests across
  2 packages); root `make buf-lint buf-go-pb-check` passes. The processor is not
  wired to File/Search, the GIS purge stage is not implemented, and no
  multi-service acceptance is claimed; this does not close T33.
- **Search owner verification (2026-09-30):** After correcting the R23 migration
  fixture and the Search entitlement fixture inputs, `rtk go test ./... -count=1`
  from `src/backend/search` passed all 201 tests across 11 packages. This closes
  the earlier Search full-suite verification gap; it does not close T33 because
  Messaging still does not invoke Search/File, GIS has no purge stage, and the
  required physical-object-GC receipt and multi-service acceptance remain open.
- **T33 GIS purge-stage implementation (2026-09-30; incomplete):** GIS now
  advances terminal match/fleet sessions from the immutable Chat retention
  receipt to a durable `content_purge_pending` owner stage. It waits for the
  saved `history_expires_at` using the GIS database clock, sends a stable
  request-hash-bound operation over the existing protected Messaging mTLS
  listener, and accepts only a matching Messaging completion receipt. The
  session owner config supports the optional Messaging client; Phase 0 maps it
  to the existing GIS certificate and Messaging protected listener. Red test
  first reproduced the absent owner adapter. Focused GIS terminal lifecycle
  checks passed 3 tests; the Messaging GIS client/config checks passed 2 tests.
  A full GIS registry run was attempted but interrupted after remaining silent
  for over a minute; no full-suite result is claimed. Messaging still does not
  execute File and Search, and the physical File GC completion evidence and
  multi-service acceptance remain open.
- **T33 continuation (2026-10-01):** Messaging now wires the durable purge
  coordinator to the protected gRPC service. The coordinator freezes the
  Messaging work set, releases its exact MESSAGE references through File,
  waits for terminal `GC_COMPLETE` / `RETAINED_SHARED` evidence, purges the
  frozen message IDs through Search, validates both receipts, and deletes the
  Messaging payload only after both owners succeed. Exact completed replay
  returns persisted File/Search receipt hashes without calling either owner.
  File/Search client TLS, Messaging signer/JWKS, Story/Space HTTPS JWKS,
  Phase 0 certificates and Compose trust wiring are now present. Focused
  verification passed: Messaging coordinator/store 5 tests, Search purge 3,
  File GC 2, GIS sessionowners 2; the focused Messaging `go vet`, Compose
  config, Phase 0 bootstrap and fixture checks passed. A uniquely named local
  Compose stack reached healthy services, and HTTPS/mTLS probes passed for
  Story and Space JWKS, Messaging mTLS JWKS, and File/Search protected gRPC
  TLS. The stack was removed after probing. The Messaging full gRPC package
  run emitted no output for five minutes and was stopped, so that suite remains
  unverified. GIS retention cleanup initially failed because its test helper
  did not advance the database cutoff after reaching `content_purge_pending`;
  the helper now matches the existing registry lifecycle test behavior, and
  all 3 `TestSessionRetention*` tests pass. T33 remains open until real seeded
  GIS→Messaging→File/Search/PostgreSQL/ObjectStore acceptance proves early and
  exact cutoff, lost-response replay, changed replay, owner outage/partial
  progress, shared-blob survival, and final content/object absence.
- [x] **T34** `D: T31,T33` Implement individual keep-group consent, continuing
  existing permanent party where valid; preserve only consenting members and
  authorized history. No auto-friend/DM or silent transcript copy.
  **Development contract resolved:** SE08 and the API canon require a separate
  participant-authenticated consent receipt bound to Auth account/profile,
  app/environment, session, exact complete roster revision, destination,
  operation and policy revision. Game-server roster or owner approval cannot
  substitute. GIS now exposes a separate Voice-bearer POST route, revalidates
  token epoch/revocation, derives account/profile only from Auth claims, checks
  the active app/environment binding and exact complete current roster/lease,
  resolves destination/policy revision server-side, and persists an immutable
  receipt by operation ID. `CurrentSessionConsentProfiles` intersects receipts
  with the current roster, binding, lease and policy revision. HTTP acceptance
  passed 2 tests; PostgreSQL receipt acceptance passed 1 test covering exact
  retry/restart, account mismatch, outsider, stale revision, policy change,
  expiry and exclusion of non-consenting roster members. Gateway route
  acceptance passed 1 test and confirms the player JWT is preserved; it is not
  a `vgi1` exemption. GIS now persists the keep-group owner operation, consumes
  only current consenters, creates a separate permanent group when no eligible
  party exists, or uses Chat's additive membership operation to preserve an
  existing party roster. Stable Chat operation IDs and immutable owner receipts
  survive exact replay; changed operation hashes conflict. PostgreSQL acceptance
  passed for both new-group and existing-party paths, including recovery after
  a Chat-create outage while the durable operation still has a NULL Chat ID;
  retry uses the same stable owner operation ID. The expanded T40 Compose
  public-client acceptance passed (1 test, 6.707s) through Gateway → GIS → Chat:
  it verifies player JWT routing, only the consenting profile in the keep-group
  receipt, the same party Chat and its existing roster, three distinct match
  Voice rooms, and party survival after child close. The membership-only Chat
  owner effect has no transcript-copy, DM, or friend operation. T34 development
  is complete; live provider/device and staging gates remain excluded.
- [x] **T35** `D: T31,T16` Wire text/send/read, Realtime events and reconnect:
  session snapshot, selected-chat cursor fetch, duplicate-free retry and scoped
  cache; no global WS history replay. Test lost ACK and connection restart.
  **Development evidence:** the no-store player route uses a verified Voice
  JWT and GIS checks the signed account/profile pair against an active player
  binding, current complete roster membership/revision, active session and
  database-clock lease. Unknown/foreign IDs are indistinguishable; revoked
  binding and lease equality deny. An internal conformance client revalidates
  the snapshot before fetching only that session's Chat with a typed Messaging
  cursor, partitions cache by app/environment/account/profile/session/chat/
  roster revision, evicts stale same-session revisions, and retries sends with
  the caller's stable client message ID. Checks: GIS HTTP 4, GIS PostgreSQL 1,
  Gateway JWT route 1, client conformance 3, and Messaging cursor/idempotency
  3 tests passed. Windows follow-up (2026-09-30): the documented
  hash-verified `make flutter-windows-prefetch-sqlite3` target restored the
  local native asset. The reconnect resume regression plus Chat panel suite
  passed 37 tests; Game Integration client and Realtime protocol suites passed
  17 tests. This confirms client snapshot, consent/unlink, cursor protocol and
  transport resume paths run on Windows. The app-level event resubscription
  and selected-chat history reconciliation after connection restart remain
  `t103_reconnect_history_delta_red_test.dart` passed and verifies a real
  `RealtimeHub` reconnect performs one REST delta for the selected chat after
  the accepted hello, while a mounted passive room remains history-silent.
  Together with the client conformance lost-ack/stable-message-ID checks, T35's
  development scope is complete. Realtime `s` remains transport-only; no global
  WebSocket history replay. Live service/device acceptance stays in the final
  gated audit.
- [x] **T36** `D: T31,T16` Wire real Voice/LiveKit admission, mute/deafen/PTT,
  read-only/listen-only, organizer restrictions, capture handoff and one
  account-wide active session. Prove actual media and active revoke/eject.
  **Development evidence:** migration `000008_account_voice_fence` durably binds
  each profile to its User-resolved account and atomically reserves one account
  across profiles/devices. Voice compares the verified Gateway account claim
  against User's exact profile lookup; request bodies and GIS/game client IDs
  cannot establish ownership. Direct caller+callee, group and managed-session
  admission, existing-call token/reconnect, and explicit Accept/Decline/Leave/
  End/expired-call release use this fence. The existing user-visible Leave/End
  action is the explicit transfer step; Voice never silently ejects the old
  call. `rtk go test ./internal/gameprovision -run
  TestPostgresAccountVoiceFence -count=1` passed 2 PostgreSQL tests for
  concurrent profiles, restart visibility, immutable mapping, explicit release
  and re-admission, explicit room transfer/replay, expired reservation recovery
  and active-fence rejection;
  `rtk go test ./internal/authctx ./internal/s2s ./internal/gameprovision
  ./internal/grpcsvc .` passed 178 tests across 5 packages, including trusted
  User profile resolution and cross-profile/account-claim Voice admission.
  T36 development authority is now defined as a durable,
  Auth/User-owned profile-to-account mapping, with admission serialized by
  account across profiles/devices; client IDs never establish the fence.
  Refreshed bounded checks
  `rtk go test ./internal/grpcsvc ./internal/gameprovision -run
  'TestProvisionedManagedGameSession|TestManagedGameSessionUsesLiveUserAdmission|TestVoiceGRPCVoiceRoom_(joinTokenPublishGrantFollowsVoiceSpeakPermission|voiceSpeakDenialBlocksUnmuteButAllowsSelfMute|unmuteFailsClosedWithoutOrWithUnavailableRoleChecker|nonMutePatchDoesNotRequireVoiceSpeak)' -count=1`
  passed 22 tests / 2 packages. PTT and cross-client capture-transfer behavior
  `flutter test --no-pub test/active_call_panel_test.dart test/call_providers_test.dart test/push_to_talk_keybind_test.dart` passed 27 tests on Windows after the verified sqlite native-asset prefetch. This covers app-level active-call controls, reconnect/reconciliation and PTT state wiring; cross-client capture transfer and real media remain open.
  Group voice start/join now also retain the requested target through the same
  user-confirmed Leave/End flow as direct calls and Space voice rooms;
  `flutter test --no-pub test/t053_cycle5_passive_voice_binding_red_test.dart`
  passed 13 tests, including both explicit group handoff retries, and
  `flutter test --no-pub test/call_providers_test.dart` passed 20 tests.
  This proves app-level transfer initiation/retry; local LiveKit media and
  ejection are verified by the acceptance below. Physical-device capture
  ownership and production revoke timing remain excluded runtime gates.
- **T36 local LiveKit media and ejection acceptance (2026-10-01):** Added a
  Chrome `integration_test` that connects two `LiveKitVoiceRoom` clients to the
  local LiveKit SFU using synthetic Chrome microphone tracks. It waits for the
  publisher's local audio publication, verifies the second participant receives
  the remote audio publication, invokes LiveKit's room-scoped participant
  removal API with a room-admin token, and verifies the subscriber observes the
  publisher leave. Against the existing local Compose LiveKit service, the
  command `flutter drive --no-pub --driver=test_driver/livekit_media_test.dart
  --target=integration_test/livekit_media_test.dart -d chrome
  --browser-name=chrome --headless --timeout=90
  --web-browser-flag=--use-fake-device-for-media-stream
  --web-browser-flag=--use-fake-ui-for-media-stream
  --web-browser-flag=--autoplay-policy=no-user-gesture-required` passed; scoped
  Flutter analysis reported no issues. This verifies a real local SFU media
  path and participant ejection, not a physical-device capture handoff or
  production revoke latency. T36 development criteria are complete; the
  excluded physical-device and production gates remain open.
- [x] **T37** `D: T05,T10,T16` Provision corporation→Space binding with protected
  human Owner bootstrap, role/template allowlist, no game Owner mutation and no
  Voice ban override. Include ownership-loss/dissolution recovery.
  **T37-BOOTSTRAP development subgate complete:** GIS derives account/profile
  from the verified human Owner bearer, checks application owner and active
  sandbox, enforces the reviewed `guild-default` template allowlist, opens an
  exact durable operation, and requires a distinct allowlisted regular Voice
  operator to approve it for 15 minutes. The Owner's exact retry calls Space's
  protected `CreateCommunityBootstrap` RPC; Space atomically persists the
  private Space, sole initial Owner membership, request hash and operation
  receipt, and GIS persists the returned Space ID receipt. Space rejects
  unreviewed templates and game-rank/role input is not accepted. Focused
  evidence: `rtk buf lint` passed; `rtk proxy go -C src/backend/gameintegration
  test ./internal/httpapi ./internal/registry ./internal/communityspace . -run
  'TestCommunityBootstrap|TestClientSendsExactOperation|TestLoadConfigRejectsMissingAuthorityAndDatabase|TestLoadConfigRequiresCommunitySpace' -count=1`
  passed all four packages (including PostgreSQL operation/approval/expiry and
  GIS receipt replay); `rtk proxy go -C src/backend/space test
  ./internal/authctx ./internal/grpcsvc -run
  'TestVerifiedGameIntegrationIdentity|TestCreateCommunityBootstrap' -count=1`
  passed both packages, including PostgreSQL Space-create/replay acceptance.
  **T37 development implementation complete:** GIS recovery derives the
  replacement account/profile from a regular Voice bearer; binds reason and
  opaque SHA-256 evidence digest to app/environment/corporation, old generation,
  replacement and exact operation; and immediately freezes the GIS authority.
  A separate allowlisted regular operator approves the exact operation for 15
  minutes. The exact replacement retry calls protected Space `RecoverCommunityOwner`;
  Space reserves the same generation, uses Role V2 Prepare/Finalize with the
  same operation ID, saves the Role receipt, membership/Space owner, and
  generation increment atomically. GIS advances its authority only after the
  Space receipt. A distinct operator may cancel only a never-approved pending
  operation; cancellation restores the old generation and a later request uses
  a new ID. Expired approval stays frozen and can be renewed by a distinct
  operator; an approved operation cannot be cancelled. Stale old-owner and
  operator actions conflict after generation advance.
  Focused evidence: GIS `rtk proxy go -C src/backend/gameintegration test
  ./internal/communityspace ./internal/httpapi ./internal/registry -run
  'TestClientSendsExactOperation|TestCommunityOwnerRecovery|TestCommunityBootstrap' -count=1`
  passed all three packages, including PostgreSQL freeze/cancel/reopen/expiry,
  generation-2 takeover and stale-action tests. Space `rtk proxy go -C
  src/backend/space test ./internal/authctx ./internal/grpcsvc -run
  'TestVerifiedGameIntegrationIdentity|TestCreateCommunityBootstrap|TestRecoverCommunityOwnerUsesRoleV2ReceiptAndFencesGeneration' -count=1`
  passed both packages, including protected RPC, Role outage/retry and durable
  Space receipt. `rtk buf lint` passed after proto update. Staging, production,
  real-provider and real-device acceptance remain excluded.
- [x] **T38** `D: T37` Ingest complete signed/versioned roster snapshots with
  page hash/checksum and CAS. Keep per-character grant reasons, rank policy,
  ownership generation, stale-source lease and negative revocations.
  **Focused development evidence (2026-09-30):** GIS registry selection
  `TestT32AcceptedInitialRosterPersistsDigestAndLease`,
  `TestT32CreateSessionHostMustBelongToRoster`,
  `TestT32ExpiredInitialRosterNeverPublishesActiveAndClosesResources`,
  `TestT32ExpiredActiveRosterIsSelectedForDurableClose`, and
  `TestRosterOwnerReceiptTransactionCannotCommitAfterLeaseIsReplaced` passed
  (5 tests / 1 package). Continued audit (2026-10-01) found the page ingestion
  and negative-grant implementation already present: `TestCommunityRosterSnapshotRequiresCompleteCanonicalPages`,
  `TestApplyCommunityRosterSnapshotPersistsOwnerFencedGrantsAndNegativeRevocations`,
  and `TestCommunityRosterSnapshotRejectsChangedPageOrAggregateChecksum` pass
  (3 registry tests), including canonical page hashes, aggregate digest, reason
  persistence and negative revocation. The owner-facing HTTP route test also
  passes. The cross-service acceptance now passes: GIS→Space uses the protected
  RPC with exact scope and opaque binding references; Space persists
  generation/lease-fenced projections and applies only baseline role.
  `go test ./internal/communityspace -count=1` passes; Space gRPC/store roster
  projection tests pass. T38 local acceptance is complete; staging, production,
  real-provider and real-device acceptance remain excluded.
- [x] **T39** `D: T38,T33,T36` Enforce effective permissions inside and outside
  game for public/officer/local-zone chat and media; apply bans, moderation,
  privacy and block/report policy. Close access after roster loss on REST,
  history, WS, search, file and media.

  Development scope includes the app-scoped player card/search/presence read
  surface. Public routes are `GET /api/v1/game-integrations/applications/{app}/environments/{env}/profiles/{opaque_ref}`,
  `GET .../profiles/search?q=...&limit=...`, and
  `GET .../profiles/{opaque_ref}/presence`. GIS derives viewer account/profile
  only from the validated Voice JWT; app/environment are selectors checked
  against active bindings and current owner-authorized roster state. Both the
  viewer and target must be in the same current accepted roster with an active
  binding and unexpired owner generation/revision lease. Search returns only
  explicitly consented NFC aliases permitted by current app policy. Card and
  presence omit fields without their separate consent and existing User
  privacy permission. Search, REST, history, WS, file and media deny on stale
  roster, revocation, block/ban, ambiguity or unavailable authority; responses
  are `private, no-store`. No account/profile/character IDs, owner roster,
  source rank/reason, game title, or last-seen value is exposed by this surface.

  **Local acceptance complete (2026-10-01):** GIS alias consent,
  opaque references, card/search/presence route, live owner roster resolver,
  User visibility/presence adapter and exact-Space account-ban recheck are
  implemented. Focused package tests: `go test ./internal/httpapi -run
  '^TestPlayerProfile' -count=1 -v`,
  `go test ./internal/registry -run
  '^TestPlayerProfileAliasesRequireCurrentConsentRosterAndPolicy$' -count=1 -v`,
  `go test ./internal/userpresence -run '^TestReader' -count=1 -v`, and
  `go test ./internal/communityspace -run
  '^TestClientRechecksCoMembershipWithinExactSpaceScope$' -count=1 -v`.
  Gateway player-profile coverage proves Voice-JWT preservation, rejection of
  game-server credentials, and `private, no-store`/authorization variation.
  GIS HTTP, registry, User presence and Space exact-co-membership/ban/privacy
  tests pass. Chat's immutable membership-interval entitlement gates Messaging
  history/threads/shared media, Search results/snippets and File metadata and
  download fetches; T33 acceptance also proves retryable payload/object purge.
  The Search suite and Messaging gRPC suite pass, including quoted snippets
  and every shared-media attachment kind. Voice's provisioned-session
  acceptance proves current Chat+Role admission and fences a revoked Role grant
  from warm CallStore joins and token issuance. T38 supplies generation/lease-
  fenced Space grants and negative revocation. Production, live provider/device,
  cross-host timing and deployment acceptance remain excluded. Parent checklist
  is **43/60**; end-to-end scenario matrices remain under T40 and release gates.
- [ ] **T40** `D: T30–T39` Public client + messenger E2E for party→three matches,
  keep-group, corporation rank change, two characters, offline access,
  stale roster, banned user and lifecycle freeze/restore.
  **T40-SE03-DEV subgate:** the GIS PostgreSQL orchestration acceptance now
  creates one party and three parented matches; all children retain the exact
  party Chat and durable Chat receipts, while each receives a distinct Voice
  room. A child roster update does not mutate party Chat membership, closing a
  child preserves the party and its receipts, and the test verifies only one
  Chat creation. `rtk go test ./internal/registry -run
  '^TestParentedChildKeepsPartyChatAndReceiptsAfterClose$' -count=1 -v` passed
  (1 Testcontainers/PostgreSQL case). The public-client + actual Chat Compose
  subgate is now recorded below. Keep parent T40 open for lifecycle freeze/
  restore and remaining messenger scenarios.
  An internal `controlledgame.PublicSessionClient` now drives only the fixed
  public create, operation-poll, roster, participant-consent, keep-group, and close routes;
  separate scoped game credential and Voice player JWT are explicit, redirects
  are rejected, and the focused httptest contract passed 2 cases. This client
  is acceptance tooling, not a distributable SDK. The integrated HTTP client →
  GIS → Chat/Voice/Role → messenger development subgate now passes locally.
  `T40-PUBLIC-DEV` provisions a sandbox app with an explicit Owner approval,
  creates a party and three matches through the public Gateway client, polls
  operation status, verifies exact shared Chat ownership and durable Chat
  receipts plus distinct Voice-room receipts, posts participant JWT consent
  for two players, executes keep-group for one current consenter, checks the
  actual `chat_db.chat_members` rows and preserved party roster, closes one
  child and proves the party remains active. The Compose bootstrap seeds only
  synthetic active GIS player-binding prerequisites for the disposable test
  app; no external provider call is made. The test passed with
  `VOICE_T40_COMPOSE_PUBLIC_SESSIONS=1 go test -v -count=1 -run
  '^TestT40ComposePublicClientPartyThreeMatchesAndChatMembership$' .` inside
  `t31-session-event-runner` against local Compose (`voice-t40-local`), in
  6.707s on the updated keep-group acceptance. Compose needed a unique host-port range because another local project
  owns default ports, and `VOICE_LIVEKIT_PUBLIC_URL=ws://livekit:7880` so the
  Voice container reaches its LiveKit service. This exposed and fixed a shared
  `/api/v1/operations/{id}` dispatch bug: verified `game.sessions.manage`
  credentials now reach session operation reads, while event credentials
  retain the event-operation route. The public T40 subgate is complete, but
  follow-up acceptance now includes the rank/two-character Space projection:
  three character references across two player bindings, rank downgrade that
  revokes only the officer reason, and two unique player memberships at source
  revision 2. This exposed a GIS–Space receipt-hash mismatch; GIS now hashes
  the same application/environment/corporation/Space/generation/revision/
  digest/lease/profile fields as Space. The regression first failed with
  `ErrIdempotencyConflict`, then passed after the fix, and the full public
  Compose test passed again in 6.68s. The focused GIS acceptance also passed.
  The public Compose path now submits a new operation with stale source
  revision 1 after revision 2 was accepted, requires HTTP 409
  `STALE_REVISION`, and rereads the roster to prove revision 2 and all three
  character references remain unchanged. `TestT40ComposePublicClientPartyThreeMatchesAndChatMembership`
  passed with both subcases (7.12s) against `voice-t40-local` after refreshing
  its disposable identities. The same test now fetches party Chat history through
  Gateway as the second Voice-authenticated member without establishing a
  Realtime connection; the GET succeeds and returns the history envelope.
  Managed party Chat correctly rejects member main-feed posts, so this case
  verifies offline history access and Chat authorization, not user posting.
  The same acceptance validates the second member's current account through
  Auth, bans that account/profile through the public Gateway Space route, and
  checks Space persisted the ban and evicted the profile while the independent
  owner roster snapshot remains unchanged. This proves the Space ban wins over
  a still-current source roster. Parent T40 stays open for lifecycle freeze/
  restore and remaining messenger scenarios. Synthetic binding rows do
  not count as public provider-binding acceptance. Revalidated on 2026-10-01:
  the fixture bootstrap completed and the public Compose acceptance passed both
  subcases (7.76s) against `voice-t40-local`. The legacy `DeleteSpace` hard-
  delete handler now returns `UNAVAILABLE` until scheduled lifecycle activation;
  its preservation and non-owner regressions both pass (2 tests). This closes
  the unsafe legacy entry point but does not implement lifecycle freeze/restore.
  The Space→Auth deletion-proof adapter now submits the exact operation-bound
  consume request, looks up the committed receipt after ambiguous transport
  failure, verifies the canonical binding/factors/timestamp, and returns the
  deterministic wrapped response expected by lifecycle persistence. Its two
  focused tests pass. It is not yet wired into the public deletion path or a
  recovery worker; lifecycle freeze/restore remains open.
  Gateway now projects the validated JWT `session_epoch` as
  `X-Voice-Session-Epoch`, and Space's auth context accepts only one positive
  canonical decimal value. Gateway auth and Space auth-context tests pass; the
  protected deletion path still requires the lifecycle coordinator and
  participant barrier before activation.
  Chat's first participant persistence slice is now implemented: a PostgreSQL
  transaction fences a Space while capturing raw-UUID-byte-sorted Chat IDs,
  persists the deterministic manifest root and immutable 1000-item pages, and
  recovers the exact prepare receipt on replay. The page API verifies the saved
  operation/root and derives deterministic page hashes and bound continuation
  tokens. `go test ./internal/store -run
  '^TestPrepareSpaceDeletionManifestPersistsAndReplaysOrderedPages$' -count=1`
  passed against PostgreSQL, covering 1001 chats, two pages, exact replay and a
  changed-generation conflict. This does not close T40: Chat RPC activation,
  request-bound Space principal transport, mutation/read gates, downstream
  manifest import, restore and ten-participant purge recovery remain open. The
  parent checklist remains 43/60.
  Revalidation on 2026-10-01: the complete Chat Go module passes with
  `rtk go test -p 1 ./...` (**288 tests across 8 packages**). This confirms
  the current Chat worktree remains internally green, including the manifest
  persistence slice; it does not provide cross-service lifecycle activation
  evidence or close T40.
  Chat Space-chat creation now takes the same transaction advisory lock used
  by manifest capture and rejects creation against frozen/terminal lifecycle
  fences. The initial implementations used different key derivations; a
  PostgreSQL contention regression exposed the mismatch for both Space-chat
  creation and manifest capture. A shared `voice/backend/pkg/spacemutationlock`
  key implementation now drives Space decisions, Chat creation and manifest
  capture. PostgreSQL regression coverage verifies both group and channel
  creates leave no rows after freeze, and both Chat operations wait while the
  canonical Space lock is held. The focused test
  `rtk go test ./internal/store -run
  '^(TestSpaceChatCreationUsesCanonicalSpaceMutationLock|TestSpaceChatCreationIsFencedAfterLifecycleFreeze)$' -count=1`
  passes; Space's existing same-Space serialization regression also passes.
  This closes the create-versus-freeze race only; T40 and its remaining
  lifecycle mutation/read gates remain open. Chat gRPC integration fixtures
  now apply all migrations through `000018_space_lifecycle`; the full module
  run exposed five fixtures that stopped at `000014`, and all five affected
  PostgreSQL regressions pass after correcting the shared migration helper.
  The exact-SHA `rtk go test -p 1 ./...` rerun passes **292 tests across 8
  packages**. Graphify refreshed the current tree (AST warning remains for the
  existing Linux runner header; HTML visualization is omitted because the
  graph exceeds its node limit). This still closes only Chat's create/freeze
  serialization race, not T40's lifecycle coordinator or remaining scenarios.
  Follow-up (2026-10-01): Chat's mTLS lifecycle `PurgeSpace` now preflights its
  durable `PURGE_DECIDED` fence and exact saved Chat manifest before remote
  work, performs the read-only authenticated Messaging completion-receipt
  lookup, then retries File's exact idempotent `CHAT` producer release before
  the final atomic local delete. Migration `000020` stores both deterministic
  owner-receipt byte strings atomically with Chat's completion receipt; exact
  replay compares them byte-for-byte. Both owner calls use the Chat service issuer,
  operation-bound request IDs and hashes, mutual TLS, and fresh 10-second
  deadlines. Owner receipt mismatch stops before File/local deletion. The
  outbound TLS settings are all-or-nothing and documented in
  `docs/microservices/chat-service.md`. PostgreSQL purge tests pass (2), the
  handler ordering/signature/negative tests pass (3), runtime-config tests pass
  (2), and `go vet ./internal/grpcsvc .` plus `git diff --check` pass. Re-ran
  T53's exact PostgreSQL acceptance `TestGameActionChallengeCASReplayStaleExpiryAndCrossProfile`
  successfully (1 test). T40 remains open for the Space coordinator, freeze /
  restore, remaining participants and full messenger acceptance; parent count
  remains **43/60**. A fresh full `go test ./internal/grpcsvc -count=1` run
  produced no output and its Go process CPU stayed unchanged for more than a
  minute; it was stopped and is unverified. Scoped purge, PostgreSQL store,
  configuration and vet checks are the current evidence, not a full package
  pass.
  Follow-up (2026-10-01): Messaging now accepts Space-authenticated Chat
  manifest pages on a dedicated mutual-TLS listener. Migration `000022` stores
  the page bytes, page hash, imported Chat IDs, exact request and first receipt.
  The importer enforces page order/size and final-seal position, validates
  canonical Chat IDs and page hashes, then seals only after all pages match the
  exact Chat root hash. Exact page replay returns the original receipt;
  changed request bytes and page gaps fail closed. The 1,001-chat PostgreSQL
  acceptance verifies the unsealed partial state and final root seal; focused
  handler and principal-listener cases also pass. Messaging's lifecycle fence,
  mutation gates, File producer release and purge are still missing, and Space
  does not yet call this endpoint; therefore T40 and the parent checklist stay
  open at **43/60**. The full Messaging gRPC package and integrated Space
  coordinator are not claimed verified. Root `graphify update .` re-extracted
  146 uncached source files and reported the pre-existing Linux runner-header
  parse warning; graph construction produced no further output for more than
  two minutes and was stopped, so the repository graph refresh remains
  unverified.
  Follow-up (2026-10-01): Messaging now records Space participant FROZEN/LIVE
  fences only against the exact sealed imported Chat manifest, with monotonic
  generation checks, immutable request/receipt replay, and stale-generation
  acknowledgement. PostgreSQL acceptance verifies pre-freeze writes, blocked
  writes while frozen, restore reopening, exact replay, missing-manifest
  rejection, and stale acknowledgement without an old manifest. Migration
  `000024` additionally gates reactions, pins, read-position/read-receipt,
  scheduled messages, message hides, game-message revisions/receipts, game
  cards, and game-action results under the shared Space mutation lock; reaction
  insert/update, hide deletion, and read-position insertion regressions pass.
  Thread edits flow through the guarded message row and delivery cursors use
  guarded read receipts. File producer release, purge, and Space coordinator
  wiring remain open. Parent remains at **43/60**. Focused PostgreSQL
  store test and `git diff --check` pass. Graphify re-extracted all 137 uncached
  code files and emitted the known Linux runner-header parse warning, then
  stalled in graph construction beyond the bounded attempt; refresh is
  unverified.

### P3 — game events, effects, messenger

- [x] **T50** `D: T10,T11,T16` Repair existing Bot source gaps before game
  activation: durable slash enqueue/outbox, leased polling or disable reliable
  claim, membership/read/send scope at every route, uniform interaction-token
  lookup, thread parent propagation and bot-bound completion.
  **Development verification:** the current Bot path uses durable slash event
  enqueue/outbox, leased polling, route scope and membership checks, persisted
  interaction authority, thread-parent propagation, and bot-bound completion.
  The focused repair acceptance passed 15 tests across grpcsvc/store, including
  failed enqueue, restart recovery, lease fencing/retry/expiry, poll-delivery
  completion, wrong-bot/token, read/send scope, and thread propagation. T50's
  development slice is complete; downstream event/card/effect tasks remain.
- [x] **T51** `D: T50,T11,T16,T30,T31` Accept signed game event into durable
  inbox/outbox with stable event ID/payload hash, own installation/recipient/
  character validation, expiry, 409 mismatch and idempotent Messaging
  publication. GIS installation-to-Bot binding, active binding authority and
  app-linked chat mapping must be available before runtime activation.
  **Development evidence (partial):** GIS signed ingress and the PostgreSQL
  nonce/event/outbox transaction passed as recorded below. Typed
  `PublishGameEvent` and Bot-only `SendGameEventMessage` now connect GIS's
  leased publication worker to Messaging's idempotent message insert. Bot checks
  current installation ownership/scope/whitelist and uses its own actor profile;
  GIS→Bot and Bot→Messaging use mTLS plus request-bound, replay-protected service
  principals. The worker's lease fencing, retry/backoff, terminal rejection,
  expiry, and success receipts have focused fake-transport/registry coverage;
  Bot, Messaging, and GIS owner-package checks are recorded in the sprint
  roadmap. No single cross-process Bot/Messaging/PostgreSQL acceptance run is
  claimed. Operation GET revalidates a current scoped game credential and
  filters foreign operations; GIS snapshots application owner into durable event
  authority/outbox data, and Bot checks it against the live Bot owner. GIS now
  resolves `character_binding_id` only through the unique active Auth-authorized
  `permitted_character_context` in the exact app/environment, requires the event
  recipient binding to match, and derives exactly one live leased Chat mapping.
  Unknown, duplicate binding, duplicate Chat, expired lease and mismatched
  recipient cases deny before operation/outbox; exact replay validates current
  credential/nonce then returns the immutable stored operation before live
  mapping resolution. The focused Testcontainers acceptance and all T51 owner
  service checks below pass. This closes the T51 development implementation;
  production provider/device evidence remains excluded.
- [x] **T52** `D: T51` Persist versioned cards/actions, trusted app/character
  attribution, immutable arguments, media references, safe fallback and
  forwarding/copy-as-new with actions disabled. Apply search/history ACL.
  T52-DEV passes: GIS strictly parses v2 declarative cards and the outbox worker
  transports typed facts/actions/opaque media refs; Bot accepts v2 and forwards
  the exact verified intent; Messaging validates canonical argument JSON,
  persists immutable card JSON plus JCS SHA-256 and trusted app/env/installation/
  Bot/character attribution in `message_game_cards` in the message transaction,
  and returns it through existing ACL-filtered reads. `messages.content` remains
  the fallback/search projection; generic edits are denied for card messages;
  attributed forward and copy-as-new retain only fallback text and `[]`
  attachments, with no card/actions/arguments/media. All actions are returned
  disabled pending T53's execution policy.
  Evidence: `rtk buf lint` and `rtk buf build` pass; generated GIS and Messaging
  Go outputs are byte-identical to `gen/go`. GIS command
  `rtk go test ./internal/gameevents ./internal/gameeventdelivery -count=1`
  passed 21 tests / 2 packages; Bot command
  `rtk go test ./internal/grpcsvc -run '^TestPublishGameEvent' -count=1`
  passed 2 tests; Messaging command
  `rtk go test ./internal/grpcsvc -run 'TestSendGameEvent|TestGameCardReadProjection|TestMessagingForwardAndCopyStripGameCardActionsAndMedia' -count=1`
  passed 6 tests, including PostgreSQL-backed typed RPC persistence and real
  forward/copy ACL paths; store command
  `rtk go test ./internal/store -run 'TestInsertGameEventMessage|TestGameCardPersists' -count=1`
  passed 2 PostgreSQL tests. The one complete cross-service functional audit
  remains scheduled for T90–T94; no live provider/runtime acceptance is claimed.
- [x] **T53** `D: T52,T16` Add invoke authorization, action/actor/message/card
  revision and state-version checks, server-issued single-use confirmation
  challenge for dangerous actions, no direct-invoke bypass and dual-device CAS.
  Acceptance includes stale card and state version, expired/revoked binding or
  installation, profile/session mismatch, foreign chat, duplicate/conflicting
  idempotency, 5-minute boundary, and two-device concurrent confirm (one CAS
  winner). Messaging resolves immutable card data under its chat ACL; GIS stores
  only challenge hashes and atomically consumes a challenge with one accepted
  operation. No game effect is performed before T54.
  **T53-DEV evidence (2026-09-30):** GIS PostgreSQL acceptance passed exact
  idempotent replay, conflicting replay, dual-device CAS (one winner), stale
  state, cross-profile denial, five-minute challenge expiry, and rejection at
  the expiry equality boundary. Reconfirmed this PostgreSQL acceptance on
  2026-09-30 with `rtk go test ./internal/registry -run
  TestGameActionChallengeCASReplayStaleExpiryAndCrossProfile -count=1` (1 test,
  1 package). GIS resolver acceptance passed stale card
  revision, expired action, session-chat mismatch, and current binding authority
  checks. Player-session acceptance passed account/profile/session and live
  roster/binding/lease checks. Messaging `ResolveGameAction` acceptance passed
  request-bound GIS service principal, current membership, and foreign-profile
  denial. Focused commands passed:
  `rtk go test ./internal/registry -run 'TestGameAction|TestPlayerSession' -count=1`
  (2 tests), `rtk go test ./internal/httpapi -run
  'TestRegistryGameAction|TestPlayerSession' -count=1` (3 tests), Messaging
  `rtk go test ./internal/grpcsvc -run
  '^TestResolveGameActionRequiresGISPrincipalAndCurrentMessagingMembership$' -count=1`
  (1 test), and GIS `rtk go vet ./internal/registry ./internal/httpapi`.
  The full GIS HTTP package ran 122 tests but failed two unrelated contract
  assertions (`TestInternalBindingAuthorityRequiresAuthenticatedAuthWorkload`
  and `TestT06OpenAPIRecordsUnpublishedAndUndecidedBoundaries`); that broader
  package run is not claimed as green. No game effect is performed before T54.
  **T53 replay regression (2026-10-01):** The integrated confirmation replay
  exposed that consumed admissions omitted the character binding, binding
  revision, and accepted action type from the read model used to recompute the
  idempotency hash. `GetGameActionAdmission` now restores those persisted fields
  through the operation row; exact duplicate confirmation returns the original
  operation. The full T60 Compose path now exercises this regression.
  **T53 current-checkout revalidation (2026-10-01):** Re-ran
  `rtk go test ./internal/registry -run
  '^TestGameActionChallengeCASReplayStaleExpiryAndCrossProfile$' -count=1 -v`
  in the designated sprint worktree; the exact PostgreSQL acceptance passed
  (1 test, 1 package). T53 remains development-complete; its live-provider and
  real-device gates stay excluded from this sprint. Parent remains 43/60.
  **T53 sprint-handoff revalidation (2026-10-02):** Re-ran the same exact
  PostgreSQL acceptance in the designated worktree; it passed (1 test, 1
  package), and `rtk go vet ./internal/registry` passed. No checklist count
  changed.
- [x] **T54** `D: T53` Persist one operation→command mapping and delivery outbox;
  sign actor proof for exact app/env/installation; bound webhook retries, 429,
  expiry and DLQ, validate URL/DNS at registration and delivery.
  **T54-DEV evidence (2026-09-30):** confirmation atomically persists the
  immutable operation→command mapping and canonical V1 command outbox row,
  binding the actor proof, app/environment, installation, character binding
  revision, card/action and state version. Delivery rechecks current app,
  environment, installation and `game.commands.receive` credential authority;
  signs the exact canonical body hash and callback request; callback URL/DNS
  are validated at registration and resolved again by the safe callback
  transport at delivery. Retries use fixed slots, honor `Retry-After` only
  within the terminal deadline, bound responses, stop terminal receiver/security
  errors, dead-letter expired commands, and reclaim expired worker leases with
  stale-lease fencing. PostgreSQL action acceptance passed command/outbox
  atomicity, one command under exact replay, exclusive live claim, crash/lease
  reclaim, stale worker completion denial, and 202 completion (1 test).
  Focused game command canonicalization and delivery tests passed, including
  signed raw-body 202 delivery, 429 fixed-slot selection, and terminal 409;
  focused GIS registry/HTTP tests and `rtk go vet ./internal/gamecommands
  ./internal/gamecommanddelivery ./internal/registry ./internal/httpapi .`
  passed. Graphify refresh was attempted for 15 seconds and stopped without
  output; graph freshness remains unverified. No live game backend call was
  made. T55 online execution admission remains open.
- [x] **T55** `D: T54` Implement online execution admission serialized with
  revoke, one permit per command with fixed start/complete bounds, retry without
  extending window and narrow completion right for prior game commit.
  **Development evidence:** GIS stores one 10s-start/60s-completion permit per
  command under the same player-binding lock as revoke; retries return the
  original permit. HMAC-authenticated internal issue/completion routes are
  bridged by the controlled-game GIS client. The receiver commits result and
  immutable result outbox before reporting completion; command redelivery
  replays the saved result and retries the same completion proof after response
  loss. Revoke waits through `complete_before + 500ms` and remains retryable as
  pending. Focused GIS tests passed 7 cases across HTTP/registry, scoped GIS
  `go vet` passed, and controlled-game PostgreSQL acceptance passed 34 tests
  covering permit bounds, durable result replay, conflict and authorization.
  `graphify update .` was attempted and stopped after 15 seconds without output;
  graph freshness remains unverified. No live game backend call was made.
- [x] **T56** `D: T55,T07a` Controlled game backend verifies actor, binding,
  character, game state and permit, commits dedupe+effect+result outbox in one
  transaction. Different command IDs for same one-shot event cannot double
  effect. Reconcile lost ACK without inventing a new command.
  **Development evidence:** the GIS permit client validates the HMAC-signed
  admission response against command, operation, app/environment and binding
  revision, and reports committed completion only after the receiver transaction
  commits. Receiver-side PostgreSQL acceptance checks the signed actor proof
  against game-owned profile/character/binding revision and current state version
  under the transaction, then atomically persists permit, one-shot dedupe, effect,
  result and outbox. Added distinct-command replay coverage proves a second
  command for the same message/action/profile rolls back all rows and does not
  invoke the effect; same-command lost-ACK replay returns the original result and
  retries its completion receipt. Focused controlled-game PostgreSQL acceptance
  passed 35 tests; scoped GIS acceptance and both module vet checks passed as
  recorded under T55. Graphify refresh remains unverified (15-second bound).
  No real external game service was called.
- [x] **T57** `D: T56` Accept signed immutable result receipts via CAS; conflicting
  terminal receipts enter reconciliation and never overwrite success. Update
  operation/card and actor-visible status; unknown remains reconciling.
  GIS receipt persistence, conflict reconciliation, and JWT actor/profile-scoped
  `GET /api/v1/game-integrations/actions/operations/{operation_id}` are implemented.
  Focused GIS HTTP/registry tests passed (7 selected tests across 15 packages)
  and `go vet ./...` passed on 2026-09-30. Controlledgame focused acceptance
  passed 36 selected tests and `go vet ./...` passed; an unsigned GIS conflict
  response is rejected as unavailable instead of poisoning reconciliation.
  Chat now exposes the GIS-principal-bound `ProjectGameActionResult` RPC, stores
  first-terminal action projections separately from the immutable card, and
  hydrates them into ACL-checked reads. GIS writes a durable lease/retry outbox
  in the same transaction as result acceptance; actor status remains
  `reconciling` until projection delivery is acknowledged. Focused PostgreSQL
  evidence passed: GIS `TestGameActionChallengeCASReplayStaleExpiryAndCrossProfile`
  (1 test, including actor `reconciling`→terminal status and projection lease
  reclaim/stale-worker fencing), GIS GameAction/GameCommand handler tests
  (6 tests), Chat `TestProjectGameActionResultIsRequestBoundImmutableAndReadWithCard`
  and `TestResolveGameActionRequiresGISPrincipalAndCurrentMessagingMembership`
  (2 tests), and `go vet` on both affected modules/packages. `rtk make
  buf-lint` and `rtk make buf-go-pb-check` passed. The
  GIS `TestProjectionWorkerSendsRequestBoundGISPrincipalAndCompletesLease`
  acceptance verifies the worker issues the request-bound GIS service
  credential, sends the exact immutable result references, and only completes
  its lease after the Chat RPC responds. Chat's PostgreSQL acceptance
  independently verifies the saved result appears beside the unchanged card on
  read. Messaging now has a separate Game Integration mTLS gRPC listener with a
  strict allowlist for ResolveGameAction and ProjectGameActionResult; ordinary
  Messaging listeners reject both RPCs. The listener verifies the exact
  gameintegration issuer, service subject, messaging audience, method, request
  ID/hash and operation binding, and requires a shared Redis replay guard.
  Principal/interceptor tests, Messaging startup compile checks, GIS worker and
  registry checks, and Messaging PostgreSQL projection/read checks passed on
  2026-09-30. The composed exact-SHA GIS result-receipt → real GIS outbox
  worker → Messaging PostgreSQL read acceptance passed in the isolated Phase0
  Compose project `voice-t57-e2e` on 2026-09-30. The gated acceptance test
  seeded both GIS and Chat/Messaging PostgreSQL stores, accepted the receipt,
  delivered the GIS outbox through the real mTLS client/listener, and verified
  the result beside the unchanged card and succeeded actor status. T57 is
  development-complete; live provider/device acceptance remains outside scope.
  **T57/T60 Compose rerun (2026-09-30):** Using the existing healthy
  `voice-t60-e2e` stack's test runner, `TestT57ComposeResultReceiptProjectsToMessagingPostgres`
  and `TestT60ComposeSignedGameEventPublishesOneImmutableCard` both passed in
  the final run (2 tests). The runner's older log contains two prior T60
  readiness timeouts followed by a successful retry; the direct rerun passed
  without timeout. This verifies result projection and signed event→Bot→Chat
  membership→immutable card publication separately. It does not verify the
  full T60 event→action→single game DB effect→result chain.
- [ ] **T58** `D: T51–T57` Add separate app-conversation consent, category opt-in,
  quiet hours/DND/block, revisioned unsubscribe and queued push suppression.
  Master validates node notification references; push never executes action.
  GIS stores revision-checked consent per active player binding and exposes an
  authenticated owner update endpoint. Enforcement now carries GIS-verified
  app/environment scope on game-event `MessageSent` events and Notification
  checks current per-binding opt-in immediately before each device send. A
  signed, nonce-bound Notification→GIS lookup fails closed on missing consent
  authority; current account blocks in either direction also suppress the push.
  Existing Voice quiet-hours/DND policy runs first. Game pushes use generic copy
  and navigation references only; they do not carry or execute action commands.
  The Social protobuf dependency is included in the Notification module and
  Docker build context. Focused evidence: Notification `go test ./...` and
  `go vet ./...` passed; GIS consent/HTTP/PostgreSQL action-ledger tests passed,
  including `TestGameActionChallengeCASReplayStaleExpiryAndCrossProfile`;
  Messaging scoped publisher/game-event tests passed; `make buf-lint
  buf-go-pb-check` passed with 50 generated Go package trees synchronized.
  T58 remains open for the integrated T90–T94 audit and remaining product/event
  consumers. Implementation surfaces: `jetstream_events.proto`, Messaging's
  game-event publisher, Notification message consumer and MessagePusher, GIS
  consent registry/internal route, and compose/config wiring.
  **T58 contract reconciliation (2026-10-01):** The earlier in-app audit gap
  below is superseded by the later Realtime and social-event implementation
  entries in this plan and the accepted event/recipient/scope/deduplication
  contract in [game-bot-interactions](../features/game-bot-interactions.md#6-уведомления-и-consent).
  Realtime now applies current `in_app` consent to game-scoped message activity
  and explicit mention/reaction social alerts; Notification applies current
  `push` consent and normal Voice DND/block policy. Focused current-checkout
  reruns passed: Notification game routing/retry/consent selection (**4 tests
  across 18 packages**), Realtime consent/fanout selection (**5 tests**), and
  GIS authenticated consent routes plus PostgreSQL mapping/lease resolution
  (**7 tests across 2 packages**).
  T58 remains open for the wider product/platform matrix and T90–T94 audit; the
  former missing in-app contract is no longer a blocker.
  **T58 handoff revalidation (2026-10-02):** The scoped Realtime consent/fanout
  selection passed (10 tests, 1 package); Notification queued-consent, per-
  device suppression, chat-scope, quiet-hours, and block selection passed (10
  tests across 18 packages); and GIS consent/lease plus T53 action acceptance
  passed (4 tests across 2 packages). T58 remains open pending its wider
  product/event matrix and final integrated audit; parent remains 43/60.
  A fresh Windows rerun on 2026-09-30 passed the focused Notification dispatch
  and signed GIS-consent client suites (43 tests) and `go vet` for both packages.
  The broader `go test ./...` completed 262 tests but two Testcontainers tests
  panicked because this Windows Docker setup reports `rootless Docker is not
  supported on Windows` (`fcm/TestFCMSender_InvalidTokenDeletesFromDB`,
  `grpcsvc/TestGetNotificationSettings_DefaultScope`); those two results are
  environment failures, not passing evidence.
  **Realtime in-app slice (2026-10-01):** game-scoped `MessageSent` now carries
  its GIS-verified app/environment scope into Realtime's optional personal
  notification fanout. Realtime performs a signed, nonce-bound GIS lookup with
  a distinct `realtime` key for the recipient's current `game_activity`/`in_app`
  consent; denial/unavailability suppresses only that personal row, preserving
  chat member delivery. Compose injects the independent key into GIS and
  Realtime. Focused client, GIS HTTP and config tests pass. This does not cover
  mention/reaction event scope, the remaining T58 consumer/platform matrix, or
  T90–T94, so T58 remains open.
- **T58 social-event continuation (2026-10-01):** The feature contract now
  scopes mention/reaction only from app/environment IDs persisted on the source
  message by GIS ingress; mention recipients are explicit profile IDs, reaction
  recipient is the source author, and both use `game_social` plus the original
  `MessageStreamEvent.event_id` for replay identity. Messaging copies scope from
  the stored row into both event payloads; Realtime attaches category/scope and
  source event ID, then uses its existing signed GIS per-recipient current-consent
  gate. The client notification center deduplicates by type/source event ID.
  Red/green evidence: the new Realtime social-scope tests failed before the
  implementation and then passed; scoped Messaging JetStream producer tests
  passed (2), the complete Realtime package passed (378), Realtime `go vet`
  passed, Messaging event tests passed (22), the affected Messaging gRPC test
  package compiled, Flutter in-app notification tests passed (18), and scoped
  Flutter analysis found no issues. Buf Go parity/lint passed; Dart output was
  regenerated and exact parity is pending rerun from the repository root.
  Realtime now resolves ordinary game-managed chat scope for in-app delivery;
  Notification push now uses the same signed GIS `profile + chat + category`
  authority when `MessageSent`, reply, or mention lacks persisted app scope.
  GIS selects the `notification` verifier for `push`; consent and block policy
  are checked per device, game-managed push copy stays generic, and scope lookup
  errors retry the JetStream event. T58 remains open for the broader
  event/product matrix and T90–T94 audit.
- [ ] **T59** `D: T52–T58` Flutter messenger: Connected Games/consent/unlink,
  app/character labels, card/detail/confirmation, pending/unknown/result,
  retry/expiry/forwarded fallback, report/block, multi-profile and accessibility.
  Document supported Web/Windows/mobile delivery gates separately.
- **T59 platform delivery gates (separate evidence; 2026-09-30):** Web uses
  PR `make flutter-ci` (analyze + VM tests), plus `flutter build web` and the
  targeted Chrome tests (for example, `flutter test test/deeplink_web_chrome_test.dart -d chrome`)
  when card/session/browser semantics change. Windows uses the Windows-hosted `flutter build windows --release`
  smoke after `make flutter-ci`. Android uses the Linux CI release APK build;
  iOS uses the macOS simulator debug build with no code signing. These Tier 2
  jobs run on `master` or explicit full workflow dispatch, not ordinary PRs;
  record each platform result separately and do not infer physical-device or
  live-provider acceptance from a compile smoke. User-excluded staging, live
  provider and real-device gates remain open.
- **T59 web delivery evidence (2026-09-30):** The selected Game Integration
  client/settings Flutter analyzer scope passed with no issues; the focused
  client and Connected Games settings tests passed (7 tests total); and
  `flutter build web --release` completed successfully. The web build emitted
  the existing 17 untranslated Russian-message warnings. This is a Web release
  compile smoke. Full frontend `flutter analyze --no-fatal-infos` passed with
  nine existing informational diagnostics; full `flutter test` passed 1,185
  tests with 96 opt-in live/provider tests skipped; and
  `flutter test test/deeplink_web_chrome_test.dart -d chrome` passed. The
  design-token, contrast-token, Flutter UI color and axe landmark gates also
  passed. The aggregate `make flutter-ci` stopped at `buf-dart-check` because the sprint's
  authorized uncommitted proto/generated Dart outputs differ from committed
  HEAD; `make buf-generate-dart` itself completed with `Dart protobuf codegen
  OK`. This expected dirty-baseline comparison is not evidence of generator
  failure. Added a delayed-invoke widget regression: switching the active
  profile while invoke is in flight prevents the returned challenge from being
  confirmed; the same card can then be used successfully under the restored
  profile. The targeted chat test passed and the full `chat_panel_test.dart`
  file passed (29 tests). This behavior is currently enforced by card-flow
  lifecycle cancellation; no extra production state was needed. T59 remains
  open for forwarded fallback, pending/unknown/error UI cases, broader
  multi-account invalidation and accessibility behavior, plus Windows, Android
  and iOS platform builds.
- **T59 pending-action recovery (2026-09-30):** Added a widget regression for
  an operation whose first status response is `reconciling`. A later tap now
  resumes actor-scoped status lookup with the retained operation ID, and the
  test verifies it reaches `succeeded` without another session lookup, invoke,
  or confirm request. Pending attempts now live for the chat panel lifetime,
  survive profile-driven card unmounts, and are bound to the originating
  account/profile. Polling stops when that identity changes; the regression
  verifies the alternate profile gets no status read and the original profile
  resumes successfully without a duplicate invoke. The focused widget test,
  full `chat_panel_test.dart` suite, focused Flutter analyzer scope, and
  `git diff --check` passed. This covers pending retry/profile isolation; it
  does not close T59's remaining multi-account cases, accessibility, and iOS
  platform gap.
- **T59 game-card accessibility correction (2026-09-30):** Enabled action
  buttons no longer announce that a session must be opened, while disabled
  buttons expose the session restriction as their screen-reader hint and
  tooltip. The `chat_panel_test.dart` game-card scenario asserts button/enabled
  semantics and absence of the incorrect restriction hint. All 29 tests in
  that file, scoped `flutter analyze --no-pub` (no issues), and targeted
  `git diff --check` passed.
- **T59 forwarded-card authority guard (2026-10-01):** `VoiceMessage.fromJson`
  now forces forwarded game-card actions disabled even if a payload incorrectly
  sets `game_card_actions_enabled: true`; safe card text/actions still render.
  A regression first failed on the enabled flag, then the full
  `messages_client_test.dart` suite passed (19 tests) and scoped Flutter analyze
  passed with no issues. This client-side defense supplements Messaging's
  forwarding/copy stripping contract; it does not close T59's remaining
  remaining multi-account, accessibility, or platform gates.
- **T59 protobuf forwarded-card regression (2026-10-01):** The new rendered
  widget test exposed a second conversion path: protobuf `MESSAGE_KIND_FORWARD`
  was converted to `VoiceMessageKind.forward`, but `voiceMessageFromProto`
  copied `game_card_actions_enabled` without applying the forwarded-message
  guard. It now disables actions after protobuf mapping and maps
  `MESSAGE_KIND_GAME_CARD` to the regular display kind. The failing public
  widget regression now proves title/facts/forward attribution remain visible,
  the action control is disabled, and no invoke/confirm route is called. The
  forwarded-card render/authority fallback subgate is now covered. The widget
  test, model regression, full `chat_panel_test.dart` (30 tests), full
  `messages_client_test.dart` (19 tests), scoped Flutter analysis, and
  `git diff --check` pass. Graphify
  re-extracted all 134 uncached code files but stalled during graph construction
  after the bounded attempt; graph refresh is unverified.
- **T59 current client acceptance (2026-10-01):** The profile-switch regression
  now covers switching accounts while invoke is pending, and expired actions
  disable on expiry with localized accessible feedback. The combined Connected
  Games, integration-client, messaging, chat-card, and in-app-notification
  suites passed 74 tests; scoped analysis of their production/test surfaces
  found no issues. The previously documented Windows release and Android APK
  compile smokes remain applicable. T59 stays open for its iOS simulator build
  and the T58/T59 dependency plus final integrated audit; no physical-device or
  live-provider acceptance is claimed.
- **T59 Windows/Android build evidence (2026-09-30):** Android release APK
  compiled successfully after running the repository's SHA-verified
  `flutter-linux-prefetch-sqlite3.sh android` asset setup. The local Android
  Kotlin incremental cache hit a cross-drive path error (`C:` package cache,
  `D:` worktree); `flutter build apk --release --no-pub` passed with the
  Gradle project property `kotlin.incremental=false`. Windows sqlite native
  assets passed the repository `make flutter-windows-prefetch-sqlite3` hash
  check, then `flutter build windows --release` passed and produced
  `voice_frontend.exe`. These are compile smokes, not device acceptance. The
  iOS simulator build remains unverified because this host is Windows.
- [ ] **T60** `D: T50–T59` Live event→card→action→one game DB effect→result tests:
  duplicate event/click/receipt, crash before/after game commit, stale/foreign
  card, revoke race, backend outage, DLQ, opt-out and multiple devices.
- **T60 composite acceptance (2026-10-01):** The opt-in local Compose runner
  now seeds an active app/environment/installation, scoped event and command
  credentials, player binding/session, and isolated Messaging chat membership;
  posts a signed event and verifies one immutable card; invokes/confirms through
  the real GIS action handler and GIS-to-Messaging mTLS resolver; delivers the
  persisted command through the real GIS delivery worker to the real
  controlledgame callback handler; commits one game-owned PostgreSQL effect;
  and sends the immutable result through the real GIS receipt and Messaging
  projection clients/workers. It verifies duplicate confirmation replay, the
  actor-visible succeeded status, the callback effect, and unchanged card JSON.
  The controlled-game callback first returns HTTP 503: GIS keeps the outbox row
  pending with `receiver_unavailable`, no game effect is committed, and the
  exact command succeeds after the retry is made due.
  The acceptance then simulates a lost HTTP acknowledgement after the game DB
  transaction commits: GIS keeps the exact command retryable, callback replay
  returns accepted, and the game DB still contains exactly one effect.
  `TestT60ComposeEventActionEffectAndResult` passed in 1.01s in the isolated
  `voice-t60-e2e` Compose project. The Compose override disables the shared GIS
  Messaging action workers so the acceptance runner owns command/result
  delivery; the runner executes the production workers directly. Component
  tests also prove duplicate signed-event idempotency and duplicate callback
  receipt behavior. A follow-up run also rejects stale card revisions and
  foreign/missing message IDs before persisting an admission; the complete
  T57/T60 Compose runner passed (0.57s, 0.46s, and 0.73s respectively). A
  subsequent revoke-race case revokes the player binding and increments its
  authority revision after challenge issuance; confirmation is denied without
  an action-operation or command-outbox row. The fixture then uses the new
  revision for a fresh successful challenge, and the full Compose runner passes
  (0.47s, 0.36s, and 0.87s). T60 remains open for a process-kill boundary
  before commit, opt-out, and multiple-device cases; do not mark the parent
  complete from partial matrix coverage.
- **T60 terminal delivery checkpoint (2026-10-01):** Added a second immutable
  card action, confirmed it through GIS, then expired its command before
  delivery. The production GIS worker marks it `dead_letter` with
  `delivery_deadline_elapsed`, does not invoke the game callback, and leaves
  the game database with only the first action's effect. The configured
  T57/T60 Compose runner passed all three tests (0.30s, 0.38s, 0.93s).
- **T60 pre-commit rollback checkpoint (2026-10-01):** The callback applies a
  database INSERT for a third action, then injects a transaction failure before
  returning a result. The real callback returns HTTP 500; PostgreSQL contains
  no partial effect and GIS keeps the command pending. Retrying the exact
  command succeeds and commits one effect. Callback statuses are verified as
  `503, 503, 202, 500, 202`; the T57/T60 Compose runner passes all three tests
  (0.25s, 0.48s, 0.83s). This proves rollback/recovery, while an actual process
  kill at the pre-commit boundary remains open.
- **T60 worker crash and fixture isolation checkpoint (2026-10-01):** The
  Compose acceptance now launches the production GIS delivery worker in a
  child process and kills it while the controlledgame transaction is still
  open, then verifies rollback and exact-command recovery. A second kill occurs
  after the game DB commit but before the HTTP acknowledgement; lease recovery
  replays the receipt without a second effect and projects the succeeded result
  to the actor. The fixture now explicitly changes the pgx pool's database
  after parsing the shared connection settings and asserts that it connected to
  its newly created isolated database. Before this fix the pool silently
  remained on `chat_db`, allowing stale result-outbox rows from the shared
  stack to leak into the test. The complete Compose runner passed all three
  tests, including `TestT60ComposeEventActionEffectAndResult` (1.35s); the
  helper test and package `go vet` also passed. T60 still needs opt-out and
  multiple-device coverage, and any remaining documented matrix gaps.
- **T60 distinct-invocation one-shot fence (2026-10-01):** Extended the real
  Compose chain to invoke and confirm the same immutable card action again
  using a distinct invocation and idempotency key, producing a distinct GIS
  command. The real controlledgame receiver detects the already-consumed
  installation/message/action/profile tuple, returns HTTP 409, and commits no
  second effect; GIS stops automatic delivery retries in `reconciling` for
  operator reconciliation. The original effect remains singular. The full
  T57/T60 Compose runner passed all three tests, including
  `TestT60ComposeEventActionEffectAndResult` (0.80s). This is distinct-command
  replay evidence, not a real multi-device run; process-kill, opt-out, and
  device-level acceptance remain open.
- **T60 queued opt-out consumer regression (2026-10-01):** Added a
  Notification message-event routing test that creates a scoped game event
  while consent is enabled, revokes consent before the consumer handles it,
  then confirms both device tokens perform a current GIS consent lookup and
  neither receives a push. The focused regression passed; the message-event
  routing selection passed 17 tests and the existing per-device dispatch plus
  quiet-hours selection passed 2 tests. This is route/dispatch acceptance, not
  persistent JetStream enqueue/ack/restart evidence or a device run. T60 stays
  open for process-kill and full integrated device cases.
- **T60 durable queued opt-out acceptance (2026-10-01):** Added an in-process
  NATS JetStream test that provisions the real `message_events` stream and
  `message` durable, starts `runMessageEventsConsumer`, persists a scoped game
  message, forces the first GIS consent lookup to fail so JetStream redelivers
  it, then revokes consent before allowing the retry to finish. The retry
  performs current consent checks for both device tokens, sends no push, and
  acknowledges the durable event (`NumPending=0`, `NumAckPending=0`).
  `TestMessageEventsJetStreamRedeliveryRechecksQueuedGameConsentAndSuppressesEveryDevice`
  passed. This proves persistent broker redelivery/ack for queued opt-out;
  process-kill recovery and real-device acceptance remain open.
- **T60 actual worker crash acceptance (2026-10-01):** The opt-in Compose chain
  now launches the production GIS delivery worker in a child process and kills
  it while the controlledgame transaction is still open, then verifies rollback
  and exact-command recovery. A second kill occurs after the game DB commit but
  before the HTTP acknowledgement; lease recovery replays the receipt without a
  second effect and projects the succeeded result to the actor. The fixture
  asserts its pgx pool connects to its newly created isolated game database; it
  previously remained on `chat_db`, contaminating result delivery with stale
  rows from the shared stack. The T57/T60 Compose runner passed all three tests,
  including `TestT60ComposeEventActionEffectAndResult` (1.35s); the helper test
  and package `go vet` passed. Opt-out/multi-device development coverage is in
  `TestMessageEventsJetStreamRedeliveryRechecksQueuedGameConsentAndSuppressesEveryDevice`
  (passed) and `TestMessagePusherRechecksGameConsentForEachDeviceAndFailsClosed`
  (focused regression passed). Every T60 matrix case now has development
  coverage; T60 remains unchecked until the T58/T59 dependency audit and final
  integrated T90–T94 pass. Live-provider and real-device acceptance remain
  excluded by the sprint scope.
- **T60 fresh isolated Compose rerun (2026-10-01):** The old project's named
  principal volume lacked the newer Space signing-key files, so its runner
  failed during Compose startup before tests. Rebuilt the current services and
  runner in a new `voice-t60-e2e-retry` project. Principal bootstrap completed
  and all three runner tests passed:
  `TestT57ComposeResultReceiptProjectsToMessagingPostgres`,
  `TestT60ComposeSignedGameEventPublishesOneImmutableCard`, and
  `TestT60ComposeEventActionEffectAndResult` (0.41s, 1.16s, 3.25s). The
  composite includes actual GIS worker-process termination before and after
  game DB commit, recovery, and single-effect/result assertions. T60 stays
  unchecked pending the dependency audit and T90–T94 integrated matrix.
- **T60 composite acceptance implementation plan:** Build one opt-in local
  Compose acceptance runner that seeds an active app/environment/installation,
  scoped event and command credentials, player binding/session, and isolated
  Messaging chat membership; post a signed event and verify the one immutable
  card; invoke and confirm through the real GIS action handler with the real
  GIS-to-Messaging mTLS resolver; deliver the persisted command using the real
  GIS command-delivery worker to the real controlledgame callback handler;
  apply one game-owned PostgreSQL effect inside its callback transaction; then
  deliver the immutable callback result through the real GIS result receipt and
  Messaging projection workers and verify actor status plus unchanged card.
  Keep the controlledgame store and effect in a separate DB/schema from GIS.
  The acceptance boundary must not introduce a production dependency from GIS
  to controlledgame. If the callback's exported configuration cannot express a
  binding authorizer outside its package, expose a minimal typed public command
  view and prove it with an external-package API test before composing the
  runner. Then extend the same controlled fixture with the T60 fault matrix.
  Do not mark T60 complete from component-only evidence.
- **T60 callback consumer seam:** `controlledgame.HandlerConfig.Authorize` was
  publicly configurable only from inside the `controlledgame` package because
  its function parameter used a private command type. Exported
  `CallbackCommand`/`CallbackActorProof` now make the verified app, environment,
  and actor available to external game backends without changing callback
  parsing or transaction behavior. The external-package regression first
  failed to compile on the missing type, then passed after the export. The
  complete controlledgame module passed (`rtk go test ./... -count=1`, 65 tests)
  and `rtk go vet ./...`. This unblocks the planned T60 real callback fixture;
  it does not verify the composite event→effect→result flow.
- **T60 component evidence (2026-09-30; not the composite acceptance):**
  controlledgame's PostgreSQL callback tests pass for durable receipt replay,
  transaction rollback, lost ACK after committed effect, and concurrent
  duplicate delivery (`TestCallbackDurableAcceptanceReplayAndBodyConflictAcrossRestart`,
  `TestCallbackRollbackAndLostAckRecoverAcrossHandlerRestart`,
  `TestCallbackConcurrentDuplicateDeliveryAppliesEffectOnce`). GIS/PostgreSQL
  action admission, current message/session/binding revalidation, actor-scoped
  status and command delivery focused tests pass. Messaging card persistence,
  immutable result projection/readback and forward stripping tests pass. Bot's
  GIS-only event publish boundary and GIS event ingress/resolution replay tests
  pass. An isolated Phase0 Compose acceptance now also proves a signed GIS event
  is accepted, delivered over the Bot mTLS/JWKS trust boundary, authorized by
  Chat membership, and persisted by Messaging as one immutable game card despite
  duplicate event submission (`TestT60ComposeSignedGameEventPublishesOneImmutableCard`,
  paired with the passing T57 result projection test). This exposed and fixed
  Bot JWKS client-certificate wiring, payload-hash prefix translation, and
  trusted Messaging-to-Chat internal membership context. The full event→action→
  one controlled-game DB effect→result chain and the listed crash/revoke/outage/
  DLQ/opt-out/device matrix remain unverified; keep T60 open until those pass.
  **2026-10-01 Compose rerun:** Against the still-running `voice-t60-e2e`
  services, an ephemeral Go runner on the Compose network passed
  `TestT60ComposeSignedGameEventPublishesOneImmutableCard` and
  `TestT57ComposeResultReceiptProjectsToMessagingPostgres`. The latter exposed
  a race with the independent projection worker: operation status can remain
  `reconciling` briefly while result delivery finishes, so the acceptance now
  waits for the terminal `succeeded` state. This confirms the two Compose
  subgates independently; the callback's actual game database effect is still
  not joined to the public action/card/result flow.

### P4 — federation and Voice Node

- [x] **T70** `D: T04–T08,T37` Implement master node registry/enrollment,
  ownership approval, mTLS identities, node-scoped S2S grants, key/cert
  rotation/revoke and installation/Space placement. No master NATS credential
  or arbitrary subject selection reaches node. **Development evidence
  (2026-09-30):** Federation authority issues a node-only 24-hour bearer on
  approval/rotation; every node request also requires the registered mTLS
  certificate and exact node/Space placement. The node surface is limited to
  signed snapshot fetch and lease renewal; suspension/rotation/defederation
  revoke by clearing the credential and advancing its authority epoch. No NATS
  credential or caller-selected subject is issued. The Federation PostgreSQL
  lifecycle/Q11 suite passed `rtk go test ./... -count=1` (38 tests / 1 package),
  and `rtk go vet ./...` passed. This closes T70 only; consumer-side signed
  snapshot activation, revision streaming, media enforcement, bundle and
  federation fault matrix remain T71–T78.
- [x] **T71** `D: T70` Implement immutable hosted resource mapping and routing
  generation for Space content/media; canonical IDs remain stable. Master owns
  Chat metadata; node owns hosted Messaging/File/Search/Voice data only.
  **Development evidence (2026-09-30):** Federation migration `000003` adds an
  append-only canonical resource route ledger with stable resource UUIDs,
  Space/type binding, immutable home node, per-generation lifecycle state and a
  closed capability set. Operator registration requires an active home node and
  an existing master Space placement; a lifecycle/capability change appends
  exactly the next generation on the same home node. Online relocation,
  same-generation conflicts, stale/skip generations, unknown nodes and
  node-authored route assignment are denied. PostgreSQL triggers reject updates
  and deletes of historical routes. Focused PostgreSQL acceptance covers exact
  retry, same-node generation advance, rejected relocation, conflict, placement
  scope and physical immutability. `rtk go test ./... -count=1` passed 39 tests /
  1 package and `rtk go vet ./...` passed. Client route discovery and scoped
  grants remain T74.
- [x] **T72** `D: T70,T71` Build signed complete authority snapshots, atomic page
  staging, revision stream, gap/conflict detection, applied-revision ACK and
  lease renewal. Unknown authority fields fail closed; reconnect can resnapshot.
  **Development evidence (2026-09-30):** Federation accepts one canonical
  complete snapshot, returns a signed manifest and signed 256-entry pages, and
  persists each sequential revision with a bounded event history. The nodecache
  verifies scope/signatures/page and total hashes, activates only after every
  page arrives, fails closed on gaps/conflicts and requires full resnapshot
  when history is unavailable. Lease ACK is emitted only for the activated
  revision/hash with a fresh nonce. PostgreSQL Q11 acceptance exercised a
  257-entry/two-page snapshot through the actual API and nodecache, then applied
  a newer revision and exact ACK. `rtk go test ./... -count=1` passed 42 tests
  across 3 packages; `rtk go vet ./...` and `rtk git diff --check` passed.
- [ ] **T73** `D: T72,T08` Enforce the measured propagation+lease+skew+eject ≤5s
  budget at reads/writes/subscriptions and **media path**. Add LiveKit admission
  verifier/watchdog that rejects an unexpired stale bearer after authority
  expiry, including SFU alive/controller dead and control-plane overload.
  **Development progress (2026-09-30):** Federation nodecache now requires an
  accepted signed lease for operation authorization, checks profile/resource/
  session-epoch/action, subtracts up to 250ms clock uncertainty and denies when
  either snapshot or lease expires. A bounded 250ms media watchdog interface
  rechecks connected participant grants and requests eject on revoke/expiry;
  focused tests cover exact permission and expiry behavior. Federation suite
  passed 45 tests across 3 packages, with `go vet` and `git diff --check` clean.
  A continuation hardens the watchdog supervisor: transient participant-list
  or eject errors are reported and retried on the bounded interval instead of
  permanently terminating revocation enforcement. `go test ./nodecache
  -count=1` passes 7 tests, including recovery after an injected transient list
  failure. This improves worker resilience but does not provide a LiveKit
  consumer or measured propagation/ejection latency.
  T73 remains open until the actual node request/subscription paths and LiveKit
  admission/list/eject clients are wired, worker failure/readiness is supervised,
  and FED02/FED03/Q06 timing is measured under control-plane overload.
  Current integration trace (2026-10-01): `nodecache.Cache.Authorize` and
  `MediaWatchdog` have no production callers. Voice `GetJoinToken` still issues
  the standard one-hour LiveKit JWT, and Voice startup does not load a Federation
  policy cache or supervise a media watchdog. The Voice call model also has no
  canonical Federation resource ID/session-epoch grant to bind into admission.
  Do not infer federation authorization from `SpaceID`, LiveKit room name, or
  the existing one-hour JWT. The missing resource/grant projection and node
  runtime wiring must be implemented before exposing federated media.
- **T73 authenticated epoch transport (2026-10-01):** Gateway now forwards the
  verified access-token session epoch to downstream gRPC metadata, and Voice
  `authctx.SessionEpoch` accepts exactly one positive integer value. Focused
  Gateway and Voice tests pass. This supplies one input needed by nodecache
  authorization; no Voice operation consumes it for Federation policy yet.
- **T73 watchdog startup validation (2026-10-01):** Added a regression proving
  the media-watchdog worker fails immediately with
  `ErrMediaWatchdogUnavailable` when its policy or LiveKit adapters are
  missing. `Run` and `Sweep` now share validation for required dependencies,
  interval, operation timeout, clock-skew bound, and clock function while
  retaining retries for transient participant-list/eject errors. The focused
  nodecache suite passed 8 tests; Federation `go test ./... -count=1` passed
  47 tests / 3 packages; `go vet ./...` and `git diff --check` passed. This
  closes only the watchdog configuration/readiness subgate. Request-path and
  subscription integration, the concrete LiveKit adapter, and measured FED02/
  FED03/Q06 remain open. Graphify re-extracted 131 files and stalled during
  graph construction after the bounded wait.
- [ ] **T74** `D: T71–T73,T35` Implement scoped client grants, route discovery,
  hosted Messaging/File/Search/Voice and messenger access, lost-response
  idempotency, cache partition by env/profile/node/generation, partial search
  and current ACL for snippets/downloads.
- [ ] **T75** `D: T72–T74` Integrate lifecycle freeze/purge/restore receipts with
  existing Space participant manifest and permanent fences. Offline node stays
  pending; old backup never resurrects Space, role, key, grant or command.
- [ ] **T76** `D: T70–T75` Build single-host Voice Node bundle: one versioned
  manifest/config, one instance of each required service/infrastructure,
  isolated stores/credentials, external ports only, install/register/status,
  update/drain, backup/restore and diagnostic commands. No manual per-service
  configuration or HA claim.
- [ ] **T77** `D: T76` Preflight and migration compatibility, maintenance state,
  component restart/failure, recovery reconciliation before serving,
  explicit upgrade/rollback rules and target-generated backup restore proof.
- [ ] **T78** `D: T73–T77` Fault injection with master+node+SFU and two Space:
  wrong audience/Space/node, stale cert, S2S partition/snapshot gap, flood+
  revoke, controller death, send ACK loss, node reboot, purged-backup restore,
  defederation and no insecure/shadow fallback.

### P5 — integration, release and evidence

- [ ] **T90** `D: T20,T40,T60,T78` Run ID, SE, MMO, BOT, FED, OPS matrix from
  `game-integrations-acceptance.md` via public API on actual services; map every
  ID to test name, logs/measurements and build SHA. Required skips are failures.
- [ ] **T91** `D: T90` Run relevant proto/buf/breaking, Go full affected modules,
  Java Maven/Flyway/Testcontainers, Flutter CI, compose and real-media checks
  per TESTING; repeat exact-SHA after integration. Preserve failure excerpts.
- [ ] **T92** `D: T91` Install bundle on clean host; register two Space; exercise
  SDK protocol client and messenger text/file/search/voice; upgrade, fail
  component, restore backup and verify fences, versions, ports and credentials.
- [ ] **T93** `D: T90–T92` Run capacity/revoke/partition timings against T08
  targets, record measured limits, operator playbook, capability matrix,
  support owner, RPO/RTO and remaining constraints. Do not claim SLA from a
  passing functional test.
- [ ] **T94** `D: T90–T93` Update PLAN/FEATURES/status and runbooks to match actual
  enabled behavior; complete rollout/rollback and committed-result
  reconciliation. Deliver exact commit/artifact versions and reproducible
  evidence. Mark sprint done only when all Voice-owned gates pass.

## Decision checklist before dependent code

| IDs | Concrete value or rule to freeze | First consumer |
|---|---|---|
| G01, Q03, Q07, Q08, Q10 | provider and subject ownership, recovery/retirement, sdk profile/alias and limits, conversion conflicts/history, voice handoff, tombstones | T13–T19 |
| G02, G03, Q01, Q09 | alt-rank merge rule, protected Owner recovery, Q01 frozen by T32 as immutable `created_at` inside membership interval `[joined_at, revoked_at)`; rejoin opens a new interval with no gap access; block/report in shared chat | T32–T33,T37–T39; SE04 |
| G04, G09 | G04 frozen by T32 using elapsed UTC time from the GIS DB clock (30×24h): closed/failed session history is available only while `now < session.terminalized_at + 30×24h`; each operation receipt has an immutable first-terminal `terminalized_at` and is retryable only while `now < operation.terminalized_at + 30×24h`, with purge at equality; a contentless external-key tombstone persists indefinitely. G09 frozen by T32: complete accepted roster lease is 60s from GIS DB commit; exact retry inert, stale lower rev no-op, same-rev changed body conflicts, incomplete/failed fetch is never empty, expiry fails closed with ≤5s media fence. | T30–T39; SE03/SE07 |
| G05, G06, Q11 | `/api/v1` and Federation `/v1`; proposed 12-month v1 support after successor-major GA; GIS-owned store; distinct Voice app owner/operator; Google OIDC; clean DB/host bootstrap and negative cases | T06,T10–T14,T70,T76; acceptance Q11 |
| G07, Q12 | Quotas and supported host/runtime; provisional capacity/RPO/RTO plus restore/load harness | T08,T11–T12,T76,T93; proposals remain unmeasured |
| G11, G12, Q02 | opt-in routing, alias visibility, scope/owner-change reconsent | T14,T58–T59 |
| T14 Auth-to-User principal | Dedicated Auth RS256 current+next keyset and JWKS separate from client JWTs; exact request-bound claims; User TLS `:9094`, shared User replay Redis; 30s credential/5s skew/35s key overlap; selected-profile IDs and positive revision checked at approval and exchange, revision change requires fresh authorization | Auth signer/client implemented and focused tests pass locally; [Auth Service](../microservices/auth-service.md#t14-auth-to-user-selected-profile-authority-accepted-target-signerclient-implemented), [Deployment](../DEPLOYMENT.md#auth-to-user-sdk-profile-principal) |
| G13, Q05 | read-only no-confirm allowlist; all risky/unknown classes require single-use challenge (fail closed if unavailable); permit +10s start/+60s commit and revoke race UX | T53–T57 |
| T15 / Q04 | Auth-owned ES256 keys and RS256 status assertions; JCS/JWS v1 exact content bytes, Messaging EdDSA tombstones, revision chain, immutable File manifest, receipt-before-freshness retry, recovery/revoke/rotation and ≤5s node admission-close ceiling | T07b, T15, T70–T78 |
| G08, G10, Q06 | control-plane capacity, export/loss/defederation terms, media lease/eject budget | T70–T78 |

No numerical default in the right column is inferred from a proposed target.
For a genuinely unspecified product choice, record a narrow question in the
owning document and continue independent tasks. A frozen decision becomes a
test assertion, not just prose.

## Validation

- [ ] `rtk buf lint`, `rtk buf format -d --exit-code`, breaking/regeneration
  where proto changes; consumer contract tests for REST/S2S and old clients.
- [ ] `rtk go test ./...` in every affected Go module; testcontainers with real
  Postgres/Redis and no skipped required suites.
- [ ] Auth Maven/JUnit/Testcontainers and Flyway validation with non-skipped
  reports; verification of all new Auth grants and conversion stages.
- [ ] `rtk make flutter-ci`, targeted widget/integration and live public client
  ↔ messenger checks on supported platforms.
- [ ] Compose/master+node+SFU with actual media, fault injection and game
  transactional effect; exact commands added as suites are implemented.
- [ ] Clean-host Node install/upgrade/restore, negative network/credential
  inspection, measured revoke and RPO/RTO, exact artifact/version manifest.

## Progress

- Active continuation (2026-10-02): primary implementation owner owns only
  Treehouse slot 24, `codex/game-sdk-federation-docs`, HEAD
  `eb830b38052c6ae182dabdd89989f7a07e59d3fc`. Fetch confirms feature remote
  equals HEAD; `origin/master` is `0282c47e3cb08b743116c8a04ca5c9ee13e1ee6e`.
  Preserve all inherited tracked/untracked sprint WIP and staged Dart outputs.
  Previous Game Integration chat is idle/interrupted; no Go/Graphify/Flutter
  check process remains. The running `voice-a1-flutter-handoff-*` Compose project
  belongs to A1 and is excluded from this owner's resources. One read-only
  Luna auditor reviews Notification security/retention; it owns no runtime.
  Notification's dedicated listener and retention worker are wired in `main.go`.
  Accepted current evidence: 305 short tests / 20 packages; vet; 3 real
  PostgreSQL lifecycle/manifest tests (empty/multi-page, exact replay, scope
  conflicts, admitted-dispatch drain, restore, purge, retention and permanent
  late-delivery fences); real Redis + HTTPS JWKS + TCP gRPC mTLS test with 3
  cases including replay/body/audience rejection, missing/foreign client CA,
  ordinary-listener denial and dependency failure. A clean-HEAD snapshot
  overlaid with the Notification batch and its event protobuf dependency also
  passes 305 short tests. Phase0 fixture checks pass 16, with the expected
  Windows POSIX-permission skip.
  Linux packaging also passes from that clean checkpoint snapshot: Notification
  Docker image `voice-game-notification-checkpoint:20261002`, digest
  `sha256:c68e63fc6f30ae58063b098c7fe09db34cda20bbca0955d28c6b13235926e048`.
  Generated Notification/event Go copies match fresh local generation; event
  scoped breaking passes, and the new Notification protobuf module compiles
  independently against the checkpoint's Chat module. This is packaging
  evidence, not a live service/provider claim.
  Source/consumer repairs also pass Chat 120,
  Messaging 400, Space 335 short tests; Space manifest coordinator 17 focused
  cases; canonical Chat root/page goldens and focused PostgreSQL transitions.
  Independent Luna risk review found no remaining blocker in the repaired
  Notification/Chat manifest, generation, purge and consent paths; it ran no
  checks. FROZEN keeps schedule generation G; restore/purge use G+1, retaining
  source schedule G. Exact request digests are checked before owner receipts
  can advance Space. Parent gates remain open pending production integration.
  Protobuf lint passes repository-wide; Notification format/scoped breaking
  and Dart generation pass. Full breaking against fetched master reports
  newer A1 Chat/User/Social fields absent on this older branch: integrate
  accepted upstream through normal merge after coherent WIP checkpoints.
  Bounded Graphify update stopped after 45 seconds without completion; stdout
  and stderr are retained in `tmp/graphify-continuation-20261002/`. No Graphify
  process remains. The ignored clean snapshot owns no running resources.
  Next: save the verified Notification checkpoint, then activate
  Space's ten-participant coordinator and continue all remaining mandatory
  development gates. Parent checkboxes stay unchanged. Master merge/staging,
  live providers and physical devices remain excluded/open; A1 hold remains.

- [x] Read pasted scope, repository instructions, PLAN, target docs, acceptance
  and design audit; fetch and compare specification branch with origin/master.
- [x] Record initial source inventory and plan/architecture contradictions.
- [x] Begin T10 with a separate Go module, service-owned migration and
  database, fail-closed token/session-epoch checks, owner-derived draft app
  creation, Gateway/Compose/CI wiring and a first registry contract. The
  registry and public game capability remain incomplete.
- [x] Bot T50 durable slash admission/lease slice committed on
  `codex/game-bot-durable`; remaining Bot work continues on that branch.
- [x] Freeze the T51 Game Event v1 contract in the API, feature, service,
  acceptance and plan docs. See the [Game Event v1 contract](../architecture/game-integration-api.md#t51-game-event-v1-ingress-and-publication-contract).
- [x] T51 GIS ingress slice: strict RFC 8785/schema parsing and signed HTTP
  verification; current `game.events.write` credential check; one transaction
  for active installation/mapped chat resolution, 300-second credential nonce
  replay fence, immutable `(app, env, event_id)` tombstone/operation and one
  publication outbox row. Exact same nonce/event/hash replays the operation;
  changed payload conflicts; nonce collision rejects; already-expired events
  persist terminal `expired` without outbox. Migration:
  `000018_t51_game_events`. Focused parser+HTTP run passed 18 tests / 3
  packages. PostgreSQL acceptance passed 2 Testcontainers tests covering replay,
  hash mismatch, nonce collision, one outbox, and expired tombstone. The ingress
  slice is development-complete. Scoped operation status acceptance
  `rtk go test ./internal/httpapi -run
  'TestGameEventOperationReadRequiresScopedGameCredentialAndIsNoStore' -count=1`
  and durable operation/outbox acceptance `rtk go test ./internal/registry
  -run 'TestAcceptGameEventPersistsOneOperationAndOutboxWithReplayFences'
  -count=1` each passed (1 test).
- [x] T51 publication subgate: GIS leases durable outbox rows and signs
  request-bound `PublishGameEvent` calls over mTLS; Bot enforces service
  principal, installation owner/scope/whitelist, and Bot-owned sender identity;
  typed `SendGameEventMessage` is Bot-principal-only, checks current chat send
  authority, and inserts by the stable client message ID so retries do not emit
  duplicate messages. GIS completion/retry/rejection uses lease fencing,
  bounded backoff, terminal expiry, and an operation receipt. Focused evidence:
  Bot `rtk go test ./internal/grpcsvc ./internal/principalgrpc ./internal/principalruntime -run 'TestPublishGameEvent|TestConfigFromEnv' -count=1`
  (4 tests / 3 packages) and `rtk go test ./internal/principalruntime -count=1`
  (3 tests); Messaging `rtk go test ./internal/grpcsvc ./internal/store -run 'TestSendGameEventMessage|TestInsertGameEventMessage' -count=1`
  (3 / 2); GIS `rtk go test ./internal/gameeventdelivery ./internal/registry -run 'TestWorker|TestAcceptGameEvent' -count=1`
  (4 / 2) and `rtk go test . -run 'TestLoadConfigGameEventPublisherRequiresMutualTLSAndGISPrincipal' -count=1`
  (1). These checks are bounded owner-package/fake-transport/DB checks, not a
  cross-process full-path run.
- [x] T51 remaining development behavior: persist an owner-authorized active
  GIS character-to-player-binding relation and resolve each event to exactly
  one active GIS Chat mapping. Unknown, ambiguous, inactive or foreign-scope
  identities reject before outbox commit; retries keep the saved resolution.
  `rtk go test ./internal/registry -run '^TestAcceptGameEvent' -count=1`
  passed 3 Testcontainers/PostgreSQL cases, covering first resolution, exact
  replay after lease expiry, new-event expiry denial, unknown character,
  duplicate active character binding, duplicate live Chat mapping, payload
  conflict, nonce reuse and outbox fencing. `rtk go vet ./internal/registry`
  passed. The full registry suite emitted no output for about 90 seconds and
  was stopped; it remains unverified and is not reported as a failure. No live
  provider call is part of development evidence.
- [x] Run T11's bounded lifecycle verification on the feature branch after
  T15 merge: `rtk make game-integration-bootstrap-acceptance` and
  `rtk make game-integrations-q11-acceptance` pass. GIS `rtk go test ./...`
  passes 92 tests in 4 packages; `rtk go vet ./...` and
  `rtk golangci-lint run ./...` pass. Hosted PR #518 checks passed on code head
  `ec281db115f46dcc46f14842ad677f481f4e9505`; this is CI evidence for that
  exact head, not for later documentation commits. No live provider, staging,
  or deployment proof is claimed.
- [x] Run required GIS PostgreSQL integration assertions in the supported
  workspace: full `rtk go test ./...` passes 92 tests across 4 packages,
  including the API-created empty-database bootstrap and SQL-backed registry
  lifecycle. The previous Windows Testcontainers attempt failed before
  assertions and is superseded by this successful GIS run.
- [x] Historical pre-HTTPS T31 local E2E on 2026-09-28: Party + parented Match
  activated only after all four owner receipts; exactly two JetStream active
  events under the earlier transport; child close
  fenced Voice and revoked child Role grants while parent stayed active; parent
  close and replay returned stable receipts; public API revoked the disposable
  credential. Full details, IDs, and scoped test results are in `STATE.md`.
  This proves orchestration/close behavior only; it does not satisfy the current
  HTTPS claim/inbox/ACK acceptance gate.
  `rtk go test -timeout 120s ./internal/store` in Role timed out during
  `TestOwnershipTransferV2_AbsentFinalizeCannotMoveOwnerOrCreateReceipt`
  cleanup (`StartPostgres.func1` at `src/backend/pkg/integrationtest/postgres.go:100`);
  the six focused `TestGameSessionGrant` store tests passed. This fixture cleanup
  timeout is recorded as a suite limitation, not a T31 grant assertion failure.
- [x] T31 retry and recovery bounds on 2026-09-28: focused PostgreSQL tests
  verified healthy-DB acceptance under 2s, per-stage exponential owner retry
  delays capped at 30s, and fresh-worker reclaim of a real five-second lease within
  6s. Both focused tests passed.
- [ ] The unified sprint remains open: T00–T94 implementation and acceptance
  gates are not complete. T00–T02 above record only the audited base/scope and
  do not imply the sprint or any downstream vertical is accepted.

## Decisions

- Scope is the user's unified Voice sprint, including `sdk-account`, both
  conversions, federation and Voice Node. Engine SDK and developer tools are
  separate; internal acceptance clients remain in this sprint.
- The owner already accepted the 15 product directions in game-integrations.
  G01–G13/Q01–Q12 are detail/verification work, not 25 automatic user blockers.
- A separate Game Integration Go service and single-host, single-instance Node
  bundle are fixed directions. Existing domain ownership remains intact.
- The owner authorized implementation alongside A1 and supplied the game-docs
  branch as the canonical source. Existing master federation prose is legacy
  material and must not constrain the new design. Update PLAN at sprint start;
  hold all master merges and staging deployment until A1 acceptance completes.
- The owner delegated all routine G01–G13/Q01–Q12 technical choices to this
  implementation team. Record choices before dependent code without asking for
  another approval.
- T14 Auth→User selected-profile authority is fixed to a dedicated Auth
  service-principal keyset and JWKS (separate from client JWTs), User TLS
  `:9094`, and User-owned shared Redis replay admission. The exact request-bound
  credential, two-key rotation and 35-second overlap, and approval/exchange
  profile-revision checks are the accepted target in the Auth service and
  deployment docs above; implementation and rollout evidence remain open.

## Risks and follow-ups

- T33 Compose continuation (2026-10-01): `TestT33ComposeManagedChatPurgeRetriesRealMessagingAndSearch` initially exposed a hash-contract mismatch after File had persisted a terminal shared-blob receipt. Messaging compared File's domain-separated lifecycle `request_sha256` with the generic principal protobuf hash. The coordinator now validates the File lifecycle hash, and its owner stub returns the same contract. Focused Messaging coordinator/handler tests (2) and `go vet ./internal/grpcsvc` pass; the real isolated Compose acceptance passes (1 test, 0.420s), proving cutoff rejection before workset persistence, GIS→Messaging mTLS, exact Message reference release with an unrelated Story reference preserving the shared LIVE blob, Search tombstone persistence, final Messaging payload deletion, lost-response retry with a stable Messaging receipt, and changed-cutoff conflict. The isolated Compose project, volumes, and generated Phase0 fixture were removed. T33 remains open for outage/partial-progress recovery and non-shared physical ObjectStore deletion evidence; parent checklist remains 38/60.
- T33 outage and physical-GC continuation (2026-10-01): File production startup now runs a retrying reference-GC worker (`FILE_REFERENCE_GC_INTERVAL`, default 1m); `RunReferenceGCOnce` fails closed if no ObjectStore deleter is supplied. Regression test `TestR23FileGC_MissingObjectDeleterCannotMarkBlobComplete` and focused `TestR23FileGC_*` checks pass; File `go vet ./internal/grpcsvc ./internal/store ./internal/jobs .` passes. Extended the isolated Compose acceptance with a seeded MinIO object and a Search `BEFORE INSERT` fault: after File commits its reference release and physical deletion, Search persistence fails, Messaging remains `PENDING`, and no Search receipt exists; removing the fault allows exact retry, completion, and lost-response receipt recovery. `TestT33ComposeManagedChatPurgeRetriesRealMessagingAndSearch` passes (1.109s), `t33-minio-verify` confirms the seeded object is absent, and Compose service resolution passes. The verifier accepts only MinIO's explicit “Object does not exist” response; auth/network/other errors fail. Messaging fixture follow-up moved the explicit read-entitlement stub into the shared integration helper and added both game-card migrations to 46 direct migration setup sites across 11 gRPC test files; the previously failing entitlement, DM send, history, moderation, reaction, and read-state cases pass, and the complete `go test ./internal/grpcsvc -count=1` suite passes (510.661s). The shared-media, thread and GetMessage cutoff tests and Search quoted-snippet API test now pass, including every attachment type and File metadata suppression for denied rows. GIS retention scheduling/retry and the Chat cutoff/replay tests also pass. T33 is complete for local acceptance; parent checklist is **39/60**. CodeGraph remains frozen under its writer lock; Graphify re-extracted 143 uncached files, then its graph build remained silent and was stopped, so refresh is unverified.
- T38 cross-service acceptance continuation (2026-10-01): GIS canonical pages, aggregate checksum, owner-fenced per-character grants and negative revocations pass focused registry tests; the protected GIS→Space exact-scope route and Space generation/lease-fenced projection tests pass. T32 terminal test-helper cutoff regression also passes with the T38 registry selection. T38 local acceptance is complete; parent checklist is **40/60**. Production/provider/device gates remain open.

- `ARCHITECTURE_REQUIREMENTS.md` still contains legacy federation fallback
  `5–10 min` auth cache and in-memory notification retry. The game-docs branch
  supersedes that text for this sprint. Remove or mark obsolete references in
  the implementation docs; implement and measure the new ≤5s authority/media
  budget and durable event/command semantics.
- T10 database write isolation now has focused PostgreSQL integration proof:
  the dedicated `gameintegration_runtime` login writes a GIS-owned row and is
  denied an insert into an Auth-owned database table. The runtime login is
  provisioned by the privileged Compose database initializer and receives
  only GIS table/sequence privileges from the service-owned migration. The test
  does not assert that connecting to another database is denied; it verifies
  the required cross-service write boundary without changing global `PUBLIC`
  connection ACLs. T11's parent lifecycle remains open for its own remaining
  acceptance and production-admission work.
- T02's Bot evidence is a current-source proof-helper/test pair at the recorded
  implementation base, not a durable delivery acceptance. T50/T51 retain the
  remaining event/command work and its runtime gates.
- The current Federation service has a bounded HTTPS authority foundation and
  service-owned schema, but `federation_db` is not provisioned and there is no
  Voice Node consumer, owning-service snapshot publisher, or media enforcement.
  `FED10` cannot be passed by issuing a shorter LiveKit JWT alone; enforce
  active media-path fencing and measure it.
- Google OIDC and operator/bootstrap contracts are already frozen in the API,
  GIS and Federation docs. Fake JWKS tests prove verifier behavior only; a real
  Voice-owned Google client must pass acceptance on the release SHA before
  production admission. Q12 targets in the design audit are proposed and
  unmeasured until T08/T93 produce restore/load results.
- This plan contains no calendar estimate: no team size, compatible host target
  or measured capacity exists yet. Those are T08/T93 outputs.
- T58 current-checkout follow-up (2026-10-01): focused app-notification tests pass: Notification dispatch/event routing (5 tests across 18 packages) and GIS consent routes (3 tests across 2 packages). They cover current per-binding consent recheck, fail-closed authority, quiet-hours priority, bilateral block suppression, generic push copy, and authenticated/revisioned consent. T58 remains open for the larger event/product matrix and T90–T94 audit.
- T58 implementation continuation (2026-10-01): add an authenticated GIS `chat + profile + category` in-app consent resolver. GIS treats a chat as game-managed when any current active `game_resource_binding_chats` lease joins an active player binding and active chat resource mapping; recipient consent must come from that same recipient's mapped active binding. This keeps scope even for an unbound chat member and denies inherited consent. Multiple distinct app/environment scopes are game-scoped but denied as ambiguous. An unmapped chat remains ordinary Voice; GIS errors suppress personal notification and direct mention delivery. Realtime invokes this lookup for personal chat notifications without persisted app scope, closing the mapping consumer gap without using chat visibility as authority. Signed route/client tests and PostgreSQL mapping/lease/consent/unbound-recipient integration pass; Realtime full module (**380 tests**) and `go vet ./...` pass; GIS full module (**377 tests / 15 packages**) and latest focused PostgreSQL+HTTP tests pass; GIS `go vet ./...` passes. Graphify extracted 140 uncached source files with the existing Linux runner header parse warning, then stalled during graph construction and was stopped at the bounded window; refresh remains unverified. Keep T58 open for the broader event/product matrix and final sprint audit.
- T58 Notification push continuation (2026-10-01): Notification now sends an authenticated `profile + chat + category + push` lookup to GIS for unscoped `MessageSent`/reply (`game_activity`) and mention (`game_social`) events. GIS verifies the `notification` workload, and its existing registry resolver remains the single mapping/lease/recipient-consent authority. Per-device rechecks suppress opted-out devices, current game blocks still apply, authority errors retry JetStream, mapped pushes use generic copy, and unmapped Voice chats retain normal preview delivery. Red/green evidence: client test failed before the signed chat lookup existed; GIS route test failed before push channel support; after implementation, Notification full module passes (**336 tests / 18 packages**) and `go vet ./...` passes, including dispatcher and end-to-end event-routing cases. GIS full module passes (**378 tests / 15 packages**), `go vet ./...` passes, and focused route/registry tests pass (**5 tests / 2 packages**). Graphify extracted 139 uncached files with the existing Linux runner header parse warning; graph construction remained idle for a bounded window and was stopped, so refresh is unverified. T58 remains open for its broader acceptance matrix and T90–T94.
- Full GIS module continuation (2026-10-01): the serialized `rtk go test -p 1 ./...` run completed successfully with **375 passed across 15 packages** after the four targeted regression repairs recorded in the active worktree. This is GIS module evidence only; it does not close any sprint acceptance item by itself.
- T40 lifecycle source audit (2026-10-01): durable Space lifecycle aggregate/store, Auth proof adapter, Chat manifest contract and File/Search/Subscription participant handlers exist, but Space has no coordinator runtime and the remaining participant handlers are not implemented. The Chat purge proof contract is now fixed in `docs/microservices/chat-service.md` and `space-service.md`: Chat performs read-only authenticated Messaging receipt lookup, then exact idempotent File release for producer CHAT, validates both owner receipts and only then deletes the saved Chat manifest. Missing/mismatched evidence leaves rows and the purge fence untouched. This removes the contract blocker; coordinator/transport implementation and lifecycle acceptance remain open.
- T40 File manifest-binding reconciliation (2026-10-01): the Space aggregate receives the canonical Chat manifest from the schedule operation and sends that same binding to every participant; Space has no endpoint to fetch File's private producer-reference root. File now verifies and retains the full `SPACE`/`CHAT`/`MESSAGING` root internally while its fence/purge request and participant receipt use the shared Chat manifest. File cannot freeze, restore, or complete purge unless its saved root still matches every sealed producer declaration and all three releases. Red/green File PostgreSQL cases passed for shared-manifest restore, purge barrier, receipt recovery, generation fencing, and immutable replay. This resolves the manifest transport mismatch; the ten-participant coordinator and the remaining T40 handlers/acceptance remain open. Parent checklist stays **43/60**.
- T40 coordinator implementation plan (2026-10-01): implement the production Space lifecycle path against the accepted P3 contract above. First add failing tests for authenticated schedule admission, durable freeze receipts, completion only after all ten exact participant acknowledgements, fresh-DB-time restore vs purge decision, and restart recovery. Then build owner RPC adapters and the coordinator around the existing `spacecore` aggregate and `SpaceStore` locked transitions; wire DeleteSpace/RestoreSpace/status and a retry worker from `space/main.go`. Keep missing participants fail-closed and preserve exact request/receipt bytes. Verify focused Space store/service tests, participant contract tests, `go vet`, and a bounded Graphify refresh. Update the parent checkbox only after the complete documented development matrix passes; live-provider/device gates remain excluded.
- Flutter continuation (2026-10-01): fixed all nine `flutter analyze` infos found in the active tree, including the friend-request element comparison; analyzer now reports no issues. `rtk flutter test test/connected_games_screen_test.dart test/game_integrations_client_test.dart` passed, and the full `rtk flutter test` run passed **1,189 tests** with 96 opt-in live tests skipped. These are local frontend checks, not T59 cross-service acceptance or the T91 complete sprint gate.
- T40 fence-barrier core (2026-10-01): `spacecore.LifecycleAggregate.FenceRequest` now reconstructs the phase/generation-bound generic request using the exact wrapper hash expected for each fixed participant. `internal/lifecyclecoord.Coordinator.ApplyFenceBarrier` calls all ten participants in canonical order with fresh 10-second deadlines, persists each receipt before continuing, and resumes after restart by skipping already persisted receipts. A failure/retry test proves the schedule completion is withheld after a participant timeout and exact requests are reused for progress recovery. Focused coordinator/aggregate tests, existing PostgreSQL concurrent fence/restore lifecycle tests, legacy DeleteSpace disabled regressions, related Community bootstrap/recovery/roster tests, and scoped Space `go vet` pass. Graphify extracted modified Go files but stalled during final graph construction in the bounded 60-second attempt; refresh remains unverified. This is an orchestration core only: owner RPC adapters, manifest capture/import, production runtime wiring and full T40 acceptance remain open; parent count remains **43/60**.
- T40 manifest capture/import core (2026-10-01): `lifecyclecoord.Coordinator.PrepareFreeze` now checks for the durable Auth deletion-proof receipt before remote side effects, prepares Chat's immutable root, fetches and validates ordered manifest pages, imports each exact page into Messaging (including final sealing), asks File to prepare the shared binding, and persists Space's freeze manifest only after all three owner receipts validate. `SpaceStore.LifecycleDeletionProofRecorded` reads the receipt under the shared Space lock. The Space module now depends on the Messaging generated proto module and its transitive Game Integration proto module. Scoped `go build ./internal/lifecyclecoord ./internal/store` passes. Tests were not run in this pass; authenticated client adapters, runtime wiring, freeze barrier activation, restore/purge handlers and T40 acceptance remain open. Parent remains **43/60**; Graphify re-extracted 133 files but stalled during graph construction, so refresh is unverified.
- T40 authenticated schedule path continuation (2026-10-01): added a request-bound Space service principal adapter for Chat manifest prepare/page, Messaging page import, and File reference-manifest prepare, plus a ScheduleDeletion coordinator entry that runs manifest preparation before the ten-participant freeze barrier. DeleteSpace now reserves the immutable operation, replays an exact completed outcome, consumes and persists Auth proof evidence, then resumes the coordinator using the same operation ID. The endpoint remains fail-closed when the coordinator/Auth transports are absent. Scoped Space go build ./internal/lifecyclecoord ./internal/store ./internal/grpcsvc, go vet for those packages, and git diff --check pass; tests were not run. Main-runtime transport wiring does not exist yet, the participant map is incomplete, and Restore/status/purge/outbox recovery remain open; parent stays 43/60.
- T40 fixture regression repair (2026-10-01): a scoped Space run completed with 573 passed and 19 failed. Seventeen lifecycle-scope failures were caused by store test fixtures stopping before the 000019–000021 community migrations; the fixtures now apply those current schema dependencies, and the 17 affected cases pass. Two legacy DeleteSpace assertions were sending neither the required session epoch nor the current request binding; the tests now send a canonical operation, confirmation/proof fields, and session epoch, and both pass while the endpoint remains fail-closed without configured transports. `go test ./internal/lifecyclecoord -count=1`, scoped `go vet ./internal/lifecyclecoord ./internal/store ./internal/grpcsvc`, and touched-file `git diff --check` pass. Graphify re-extracted 130 Go files and reported the existing Linux runner header parse warning, but stalled at graph construction during the bounded 60-second attempt; refresh remains unverified. No parent item advanced: 43/60, 17 open; production runtime wiring, missing participants, restore/purge/status workers, T40 integrated matrix, and remaining sprint items remain in scope.
- T40 purge request reconstruction (2026-10-01): the aggregate now rebuilds immutable generic participant purge requests and the typed Role retirement request from its persisted PURGING decision; Role is rejected from the generic API. New tests assert protocol fields, generation, exact shared manifest, decision timestamp and Role separation. The regression test failed before implementation because both APIs were absent; afterward the focused regression and full Space spacecore/lifecyclecoord suites pass (60 tests), and git diff --check passes. Graphify re-extracted source but remained in graph construction until the bounded attempt was interrupted; refresh remains unverified. This enables a restart-safe coordinator purge barrier but does not implement participant transports, purge orchestration, fresh PostgreSQL time, runtime wiring, or full T40 acceptance. Parent remains 43/60.
- T40 restartable remote purge barrier (2026-10-01): the Space coordinator now requires all nine non-Role purge owners, retires Role first, purges Messaging and File before Chat, and records each exact receipt immediately. It skips durable receipts after restart and applies fresh 10-second RPC contexts. The red/green test injects a lost File response and verifies Role and Messaging are not repeated and File receives the same request bytes. The affected Space core/coordinator packages pass 61 tests; serial reruns of the five community fixture cases pass. A parallel four-package run first reported 986 passes and five unrelated shared-migration fixture failures; test fixtures now avoid applying the same community migrations twice, and the five cases pass serially. Scoped go vet and diff check pass. Graphify update completed (2217 nodes, 8662 edges). This is still only the remote barrier: decision scheduling from fresh DB time, local purge/tombstone HMAC keyring, all participant RPC clients/handlers, runtime activation, public status/restore/purge, retry worker and full T40 acceptance remain open; parent stays 43/60.
- T40 Messaging participant purge continuation (2026-10-01): Messaging now exposes `PurgeSpace` on the Space-authenticated lifecycle listener. It validates the exact RPC principal and request, reads the sealed Chat manifest one bounded page at a time, derives stable child operation IDs, verifies each managed-chat receipt including File and Search evidence, then atomically saves its exact Space purge receipt and `PURGED` fence. The real-store test covers the final manifest page read, atomic completion, and exact retry; handler tests cover the principal binding, stable children, and failure before participant completion. Focused lifecycle tests pass; the full Messaging store and gRPC service package run completed with 369 passing tests before the latest final-source focused rerun. The Space coordinator and a distinct File Space-producer release acceptance remain open, along with other participant implementations and runtime wiring; parent remains 43/60.
- T40 restore RPC continuation (2026-10-01): Space now exposes an authenticated `RestoreSpace` handler that replays durable outcomes, reserves the restore operation, invokes the coordinator, and returns only the persisted restore response. The coordinator checks the durable phase is `RESTORE_DECIDED` before applying the participant fence barrier. The unauthenticated-request regression passes; lifecycle coordinator tests pass (2), and scoped Space gRPC/coordinator build and vet pass. A broader gRPC package run stalled in integration startup and was interrupted, so no full-package pass is claimed. Production lifecycle transports/runtime wiring, expiry-to-purge recovery, status RPC, and full T40 acceptance remain open; parent remains 43/60.
- T40 coordinator status continuation (2026-10-01): Space now serves the proto-defined status RPC. It requires a regular authenticated account/profile/session, validates protocol version and canonical Space/operation IDs, and reads participant receipts only for the original account/profile and deletion operation. The store returns a consistent locked lifecycle snapshot, including after the Space row is purged while lifecycle evidence remains, and projects completed fence or purge receipts in canonical participant order with request/receipt hashes and timestamps; unrecorded participants remain `NOT_STARTED`. Owner-scope/partial-fence and terminal-purge projection tests pass. The focused lifecycle store selection passes 31 tests, gRPC selection 2 tests, coordinator suite 2 tests, and scoped build/vet pass. This closes the missing status RPC slice only; production lifecycle transports/runtime wiring, expiry-to-purge recovery, participant gaps, and full T40 acceptance remain open; parent remains 43/60.
- T40 participant transport correction (2026-10-02): `GRPCParticipant` now uses each generated service's exact protobuf full name. The Voice service is `voice.calls.v1.VoiceService`, not `voice.voice.v1.VoiceService`; a regression test failed before the correction and passes after it. `go test ./internal/lifecyclecoord -count=1` passes 10 tests and `go vet ./internal/lifecyclecoord` passes. This repairs the Voice descriptor lookup in the reusable transport only; owner RPC implementations and production runtime wiring remain open, and the parent checklist remains 43/60.
- T59 current-checkout continuation (2026-10-02): `make buf-generate-dart` regenerates the changed and newly added Game Integration Dart bindings successfully; after staging only `src/frontend/lib/gen`, `make buf-dart-check` passes against the current proto sources. Before staging, that check compared the intentionally dirty generated output with the Git index and failed as designed; it was not a generator mismatch. `flutter test test/chat_panel_test.dart test/messages_client_test.dart` passes all 50 tests, and scoped `flutter analyze --no-pub` across the chat panel, message/Game Integration clients, and their tests reports no issues. Full `make flutter-ci` now passes, including design-token, contrast, UI-color, Dart parity, SQLite prefetch, full analyzer, and full Flutter test gates. T59 remains open for the remaining acceptance matrix and separate platform delivery evidence; the parent checklist remains 43/60.
- T59 platform compile evidence (2026-10-02): `flutter build windows --release` succeeded and produced `build/windows/x64/runner/Release/voice_frontend.exe`; `flutter build apk --release` succeeded and produced `build/app/outputs/flutter-apk/app-release.apk` (124.3 MB). These are compile smokes only; they do not imply physical-device or live-provider acceptance. iOS simulator evidence is unavailable on this Windows host. T59 remains open for its remaining acceptance matrix and iOS delivery evidence; the parent count remains 43/60.
- T40 Bot lifecycle RPC continuation (2026-10-02): Added Bot's proto-defined `ApplySpaceLifecycleFence` and `PurgeSpace` service methods as a typed boundary to the existing durable Bot lifecycle store. Calls require the exact verified Space service principal, audience, full method and deterministic request hash; unknown fields are rejected; absent storage or mismatched durable state fails closed. Red/green handler tests pass (3), Bot lifecycle store tests pass (2), and scoped `go vet ./internal/grpcsvc ./internal/store` passes. The combined full Bot gRPC/store package run stayed live for over five minutes without visible test progress and was interrupted; no full-package pass is claimed. Bot's Space-principal mTLS listener/runtime and other missing lifecycle participants remain open. Graphify completed AST extraction (132 files) but stalled in final graph construction through the bounded attempt and was stopped; graph freshness is unverified. T40 remains open and the parent count remains 43/60.
- T40 Bot lifecycle transport continuation (2026-10-02): Added optional Bot Space-principal runtime settings, HTTPS JWKS bootstrap through the Phase0 Space JWKS proxy, bounded JWKS caching, issuer/audience/method/request/hash verification, Redis replay protection, and a dedicated mTLS gRPC listener. The listener requires the lifecycle client CA and serves only `ApplySpaceLifecycleFence` and `PurgeSpace`; protected methods fail closed on the ordinary listener. Phase0 exposes port 9444, mounts only the lifecycle CA for Bot, and wires the HTTPS Space JWKS endpoint. Focused Bot principalruntime/principalgrpc tests, the complete Bot `go test -short ./...` run (84 tests), Bot `go vet ./...`, and Phase0 fixture/compose checks (16, one POSIX-only skip) pass. Updating the old T04 test revealed that its expected `UNIMPLEMENTED` status was stale after T51 registration; the production ordinary listener now explicitly denies `PublishGameEvent`, and the regression verifies `UNAVAILABLE` before Gateway metadata processing. Space coordinator activation and other participant gaps remain; T40 and the parent checklist remain open at 43/60. Graphify re-extracted 137 uncached files but stalled during final graph construction after the bounded 60-second attempt; graph refresh is unverified.
- T40 Notification participant continuation (2026-10-02): Notification now implements the typed fence and purge handlers, validates the verified Space principal/request binding, persists deterministic exact owner receipts and transitions in notification_db, blocks Space-scoped settings reads/writes outside LIVE, and atomically removes only matching Space-scoped settings at purge. The new PostgreSQL lifecycle integration test covers freeze, restore, a later deletion, purge, exact retries, and changed-request conflict. Focused integration plus lifecycle handler tests pass (3); the complete short Notification module passes 290 tests across 18 packages and go vet passes. T40 remains open: Notification still needs its dedicated mTLS listener/runtime, receipt-retention cleanup, and Space coordinator production wiring; other participant/restore-to-purge recovery and end-to-end acceptance remain open. Parent remains 43/60. Graphify re-extracted 140 uncached files and reported the existing Linux runner header parse warning, then stalled during graph construction; refresh is unverified.

## Master integration CI repair checkpoint — 2026-10-03

PR #603 integrates committed sprint checkpoints after explicit owner merge
authorization. The first hosted run exposed generated Dart/manifest drift,
incomplete Docker module closure, historical test schemas missing newer owning
tables, and the old R22 provisioning oracle rejecting the accepted checkpoint.
Repairs regenerate canonical stubs, close local module packaging, and update
fixtures without changing production authorization. The provisioning oracle
accepts only exact path/blob matches from ancestor checkpoint
`5ff441e04422e3ff1a4c0e1e7e81e3d17b8470eb`; altered/unapproved/staging paths remain
denied by explicit negative fixtures. No gate or deployment flag is bypassed.

Messaging concurrent attachment retry had a real pool-starvation deadlock:
lock waiters held pool connections while the lock owner borrowed again for the
saved-message lookup. The lookup and game-card hydration now use its existing
transaction. The existing six-caller regression was strengthened to one pool
slot and a 10-second deadline: red before the fix, green afterwards. The seven
attachment-intent tests and four Chat/Messaging regressions pass on local
PostgreSQL with zero skips; independent read-only concurrency review accepted
the change. Missing-message, conflict, expiry and commit semantics remain.

Other local evidence: protobuf contract suite 141 passed; Auth historical
schema selections 12 passed; File 25 passed followed by both remaining repaired
cases passed; Bot two, Notification one, Search two, Space resolver four and
store four repaired regression roots passed, all without skips. Flutter analyze
reports no issues. Required hosted CI for the final repair head remains the
merge gate. These results do not close full-feature acceptance or authorize
staging/provider/device activation. Original worktree WIP and its 26 staged
Dart blobs remain preserved separately.

The follow-up repair checkpoint also passes lint for all 22 Go modules (the
12 repaired modules were checked again with uncapped diagnostics). Phase0's
five Compose contract checks pass with explicit fixture-only TLS overrides
and exact per-service readonly mount allowlists. The cold principal initializer
previously omitted mounted Space/Story/Messaging signers and three service TLS
identities. A new isolated Compose project reproduced unreadable/missing keys
before the fix, then proved all six signer pairs and eight TLS identities
readable by UID 65532, with no mounted CA private key. A second initializer run
is idempotent; a pre-existing partial ready set fails closed without repair or
overwrite. This is local/CI bootstrap evidence, not live activation.

The bootstrap security recheck found another real fixture defect: service
identities were serverAuth-only although Compose also uses them for outgoing
mTLS. OpenSSL rejected the old File certificate for sslclient (error 26).
Fresh isolated identities now validate for both sslclient and sslserver across
all eight services. The readonly/private-volume rules remain unchanged.

Search's concurrent receipt test reproduced its CI barrier timeout with a
four-connection pool. A reserved observer now checks the exact operation and
message advisory keys in pg_locks, with signed keys converted into masked
32-bit OID halves. Five repeated PostgreSQL roots pass without skips, retaining
the eight-equal-receipts assertion; Search lint also passes. Production purge
behavior is unchanged. The exact-blob R22 scope oracle passes on this repair
state, including its changed-path/blob and staging-path negative fixtures.

The next hosted checkpoint exposed a separate Phase0 generator/overlay gap:
20 bind sources referenced by Compose were absent, including Space JWKS TLS,
Bot signer/TLS, Notification/Messaging TLS and purpose-bound client identities.
A real generated-fixture/rendered-Compose regression reproduces those missing
sources. The generator now issues all configured files, with a dedicated Space
lifecycle client CA, distinct serials per issuer, two independent Bot signing
keys, and no published CA private keys. The complete offline suite passes 17
tests; the POSIX permission test is reserved for Linux CI. Independent security
review accepts this fixture delta. These credentials remain disposable local/CI
fixtures and do not enable staging or a live provider.

Hosted Messaging integration also exposed a READ COMMITTED race: a receipt
query could miss a concurrent writer, while the following revision query saw
that writer's atomic commit and reported a false orphan. Both receipt paths
now read both dedupe tuples in one SQL snapshot. A deterministic PostgreSQL
test commits the concurrent writer after the reader's first Scan: red before
the repair, green afterwards. Five receipt/revision roots pass three times
without skips, retaining immutable results, changed-bytes conflicts and real
orphan denial. Messaging and shared-package lint pass.

The T16 first protected User admission returns dependency Unavailable; the
saved hosted log does not establish whether JWKS or replay Redis failed. The
existing internal diagnostic callback now receives the bounded rejection class
from VerifyService errors, with no claim, bearer, header or raw dependency error
logged. Its two dependency regressions are red before and green after; shared
principal/security short suites pass. Public statuses and admission policy are
unchanged. T16 and exact-head ci-gate remain open pending the next hosted run.
