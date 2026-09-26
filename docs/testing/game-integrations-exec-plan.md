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
  in PLAN without changing A1's status. Do not merge game work to `master` or
  deploy it to staging until A1 acceptance is complete.
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

Each `T-*` is a reviewable work item, generally one PR or a narrow cluster of
related PRs. In each behavior PR: freeze docs/contract, write failing
contract/integration test, implement, run affected checks, update status and
acceptance evidence. `D:` lists hard predecessor tasks. Cross-service API
contracts land before consumers; activation lands after all consumers.

### P0 — base, queue, decisions, contract seams

- [ ] **T00** `D: —` Recheck local/remote branch heads, dirty files, open PRs and
  active worktrees; identify overlapping A1/Auth/Voice/Space/Bot ownership.
  Record exact implementation base SHA; keep root `master` WIP separate.
- [ ] **T01** `D: T00` Update PLAN with the owner's parallel game sprint decision
  and federation activation scope while retaining A1's current status and
  gates. Track the hard merge/staging hold until A1 acceptance, plus a separate
  game integration queue and rollback owner.
- [ ] **T02** `D: T00` Re-audit source at exact base: Bot seven source gaps,
  Auth guest conversion, Chat/Space lifecycle, Voice admission/media, current
  Federation scaffold/protos, Gateway routing, Flutter surfaces. Mark each
  capability existing/partial/absent with file+test evidence.
- [ ] **T03** `D: T01,T02` Choose technical defaults for G01–G13 and Q01–Q12
  in owning docs before dependent implementation; assign every value/algorithm,
  test and consumer. The owner has delegated these choices; escalate only a
  genuine product contradiction or unavailable external dependency. See the
  decision checklist below.
- [ ] **T04** `D: T03` Freeze trust matrix for game service, player, bot and node;
  principals, issuer/audience, scopes, credential storage, expiry, rotation,
  revoke, rate limit and direct-service negative cases.
- [ ] **T05** `D: T03,T04` Freeze resource/state model and ownership: account,
  selected profile/alias, character, app/env/installation, binding, party,
  match/fleet, corporation→Space, grant reasons, operations and tombstones.
  Update DATA_MODEL/DATA_STORES/CONTRACT_MATRIX as schemas land.
- [ ] **T06** `D: T04,T05` Review OpenAPI/proto/JSON for public Game API and S2S:
  version negotiation, errors, canonical IDs, idempotency, pagination,
  capability fallback, signed envelope/revision, required security fields.
  Add contract tests and generated-code compatibility checks.
- [ ] **T07** `D: T03,T06` Build internal controlled game backend with durable
  outbox, command inbox and transactional effect store; build independent
  protocol client with per-device keys plus real media clients. Version and
  retain fixtures. These are test infrastructure, not a public SDK.
- [ ] **T08** `D: T03` Define measured targets and harness: revocation clocks,
  game→Voice roster latency, lease/skew/eject budget, retry/retention windows,
  RPO/RTO, host/runtime/version matrix, concurrency and capacity. Separate
  target, measurement method and observed result.

### P1 — developer bootstrap and player identity

- [ ] **T10** `D: T04–T06` Implement Game Integration Go service module,
  service-owned DB/migrations, health/metrics, config/secrets, deploy wiring,
  internal auth and least-privilege credentials. Prove it cannot write other
  service DBs.
- [ ] **T11** `D: T10` Implement app/env/installation registry, owner approval,
  sandbox/prod isolation, credential issue/rotation/revoke, allowed provider,
  redirect, origin and webhook configuration. Clean bootstrap works through
  API/operator process, without direct SQL or portal.
- [ ] **T12** `D: T11` Add app-scoped quotas, suspension, diagnostics and
  provenance audit; deny cross-app/env IDs and SSRF destinations. Distinguish
  provider admission from developer assertion.
- [ ] **T13** `D: T04,T06` Implement Java Auth `sdk-account` type and schema,
  independent provider proof verifier (real chosen provider plus fake),
  issuer/audience/nonce/replay checks, admission cap and app/env-scoped account
  uniqueness. Never route via `guest`/`ConvertGuest`.
- [ ] **T14** `D: T13` Add browser/device authorization, PKCE, explicit selected
  profile/scopes, consent revisions, bindings challenge/exchange, returning
  login, no account enumeration; unauthenticated game ticket cannot mint a
  player token. Add device-code route only if required by frozen capabilities.
- [ ] **T15** `D: T13,T14` Register per-device public keys after independent
  proof; sign canonical actor/chat/body/message envelope, verify at receiver,
  revoke/rotate keys, reject tamper/replay/same ID different payload. Separate
  service, bot and node credentials; cover versioned edit/delete/attachment
  provenance per Q04.
- [ ] **T16** `D: T14,T15` Implement limited delegated grants, binding/authority
  reads, selected profile/alias serialization, direct-call scope checks and
  account/profile/app/device revocation epochs. Verify hidden profiles cannot
  leak via roster, cards, search or presence.
- [ ] **T17** `D: T13–T16` Implement durable sdk→new-permanent conversion:
  operation/status, source proof, registration, preview/consent, authority
  freeze, owner receipts, grant recompute, new credentials, retired source
  reference. Retry every crash stage with same operation ID.
- [ ] **T18** `D: T13–T17` Implement sdk→existing-permanent conversion:
  proof both identities, explicit profile, conflict preview, no implicit
  friends/role/history union, occupied binding reject/resolution, profile
  limit and sanctions. Ensure one active owner of binding at each stage.
- [ ] **T19** `D: T17,T18` Close conversion races with voice/reconnect, old
  device/token, repeated provider login, unlink, deletion and backup restore;
  retain historical author without old access or cross-app identity leak.
- [ ] **T20** `D: T11–T19` Expose versioned Gateway routes, errors and capability
  responses; run public client conformance plus forged subject/code/env,
  service-token-as-user, wrong device and direct gRPC bypass tests.

### P2 — session and managed-community verticals

- [ ] **T30** `D: T05,T10,T16` Persist app/env/external resource keys,
  operations, request hashes and state transitions. Stable retries return the
  same result; conflicting body/revision returns 409; tombstoned key is fenced.
- [ ] **T31** `D: T30` Orchestrate party/match/fleet: Chat create, Voice room,
  member grants, stage receipts, compensation and reconciliation after each
  crash point. Publish active only after all required resources are ready.
- [ ] **T32** `D: T31` Distinguish stable party from repeated match, implement
  host departure/transfer, explicit close, invited late join and bounded
  roster freshness. A failed match cannot delete shared party chat.
- [ ] **T33** `D: T31,T32` Implement per-member entitlement intervals for
  since_join/rejoin across message history, search, quotes, threads, files and
  signed download URLs; enforce retention and deletion policy.
- [ ] **T34** `D: T31,T33` Implement individual keep-group consent, continuing
  existing permanent party where valid; preserve only consenting members and
  authorized history. No auto-friend/DM or silent transcript copy.
- [ ] **T35** `D: T31,T16` Wire text/send/read, Realtime events and reconnect:
  session snapshot, selected-chat cursor fetch, duplicate-free retry and scoped
  cache; no global WS history replay. Test lost ACK and connection restart.
- [ ] **T36** `D: T31,T16` Wire real Voice/LiveKit admission, mute/deafen/PTT,
  read-only/listen-only, organizer restrictions, capture handoff and one
  account-wide active session. Prove actual media and active revoke/eject.
- [ ] **T37** `D: T05,T10,T16` Provision corporation→Space binding with protected
  human Owner bootstrap, role/template allowlist, no game Owner mutation and no
  Voice ban override. Include ownership-loss/dissolution recovery.
- [ ] **T38** `D: T37` Ingest complete signed/versioned roster snapshots with
  page hash/checksum and CAS. Keep per-character grant reasons, rank policy,
  ownership generation, stale-source lease and negative revocations.
- [ ] **T39** `D: T38,T33,T36` Enforce effective permissions inside and outside
  game for public/officer/local-zone chat and media; apply bans, moderation,
  privacy and block/report policy. Close access after roster loss on REST,
  history, WS, search, file and media.
- [ ] **T40** `D: T30–T39` Public client + messenger E2E for party→three matches,
  keep-group, corporation rank change, two characters, offline access,
  stale roster, banned user and lifecycle freeze/restore.

### P3 — game events, effects, messenger

- [ ] **T50** `D: T10,T11,T16` Repair existing Bot source gaps before game
  activation: durable slash enqueue/outbox, leased polling or disable reliable
  claim, membership/read/send scope at every route, uniform interaction-token
  lookup, thread parent propagation and bot-bound completion.
- [ ] **T51** `D: T50` Accept signed game event into durable inbox/outbox with
  stable event ID/payload hash, own installation/recipient/character validation,
  expiry, 409 mismatch and idempotent Messaging publication.
- [ ] **T52** `D: T51` Persist versioned cards/actions, trusted app/character
  attribution, immutable arguments, media references, safe fallback and
  forwarding/copy-as-new with actions disabled. Apply search/history ACL.
- [ ] **T53** `D: T52,T16` Add invoke authorization, action/actor/message/card
  revision and state-version checks, server-issued single-use confirmation
  challenge for dangerous actions, no direct-invoke bypass and dual-device CAS.
- [ ] **T54** `D: T53` Persist one operation→command mapping and delivery outbox;
  sign actor proof for exact app/env/installation; bound webhook retries, 429,
  expiry and DLQ, validate URL/DNS at registration and delivery.
- [ ] **T55** `D: T54` Implement online execution admission serialized with
  revoke, one permit per command with fixed start/complete bounds, retry without
  extending window and narrow completion right for prior game commit.
- [ ] **T56** `D: T55,T07` Controlled game backend verifies actor, binding,
  character, game state and permit, commits dedupe+effect+result outbox in one
  transaction. Different command IDs for same one-shot event cannot double
  effect. Reconcile lost ACK without inventing a new command.
- [ ] **T57** `D: T56` Accept signed immutable result receipts via CAS; conflicting
  terminal receipts enter reconciliation and never overwrite success. Update
  operation/card and actor-visible status; unknown remains reconciling.
- [ ] **T58** `D: T51–T57` Add separate app-conversation consent, category opt-in,
  quiet hours/DND/block, revisioned unsubscribe and queued push suppression.
  Master validates node notification references; push never executes action.
- [ ] **T59** `D: T52–T58` Flutter messenger: Connected Games/consent/unlink,
  app/character labels, card/detail/confirmation, pending/unknown/result,
  retry/expiry/forwarded fallback, report/block, multi-profile and accessibility.
  Document supported Web/Windows/mobile delivery gates separately.
- [ ] **T60** `D: T50–T59` Live event→card→action→one game DB effect→result tests:
  duplicate event/click/receipt, crash before/after game commit, stale/foreign
  card, revoke race, backend outage, DLQ, opt-out and multiple devices.

### P4 — federation and Voice Node

- [ ] **T70** `D: T04–T08,T37` Implement master node registry/enrollment,
  ownership approval, mTLS identities, node-scoped S2S grants, key/cert
  rotation/revoke and installation/Space placement. No master NATS credential
  or arbitrary subject selection reaches node.
- [ ] **T71** `D: T70` Implement immutable hosted resource mapping and routing
  generation for Space content/media; canonical IDs remain stable. Master owns
  Chat metadata; node owns hosted Messaging/File/Search/Voice data only.
- [ ] **T72** `D: T70,T71` Build signed complete authority snapshots, atomic page
  staging, revision stream, gap/conflict detection, applied-revision ACK and
  lease renewal. Unknown authority fields fail closed; reconnect can resnapshot.
- [ ] **T73** `D: T72,T08` Enforce the measured propagation+lease+skew+eject ≤5s
  budget at reads/writes/subscriptions and **media path**. Add LiveKit admission
  verifier/watchdog that rejects an unexpired stale bearer after authority
  expiry, including SFU alive/controller dead and control-plane overload.
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
| G02, G03, Q01, Q09 | alt-rank merge rule, protected Owner recovery, entitlement intervals/rejoin, block/report in shared chat | T33,T37–T39 |
| G04, G09 | retention and retry windows, roster source freshness and fail-closed policy | T30–T39 |
| G05, G06, G07, Q11, Q12 | API/node versions, store schema, bootstrap approver/provider, quotas, host support/capacity/RPO/RTO | T06,T10–T12,T76 |
| G11, G12, Q02 | opt-in routing, alias visibility, scope/owner-change reconsent | T14,T58–T59 |
| G13, Q05 | dangerous action classes/challenge, execution start/complete bounds and in-flight UX | T53–T57 |
| G08, G10, Q04, Q06 | signed revisions/files, control-plane capacity, media lease/eject budget, export/loss/defederation terms | T70–T78 |

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

- [x] Read pasted scope, repository instructions, PLAN, target docs, acceptance
  and design audit; fetch and compare specification branch with origin/master.
- [x] Record initial source inventory and plan/architecture contradictions.
- [ ] T00–T94 implementation and acceptance remain open. Update each check as
  work lands; do not infer completion from this planning pass.

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

## Risks and follow-ups

- `ARCHITECTURE_REQUIREMENTS.md` still contains legacy federation fallback
  `5–10 min` auth cache and in-memory notification retry. The game-docs branch
  supersedes that text for this sprint. Remove or mark obsolete references in
  the implementation docs; implement and measure the new ≤5s authority/media
  budget and durable event/command semantics.
- Bot audit was source-only at `77ec7240a`, not a current-code or live proof.
  T02 rechecks all gaps; T50 closes those that remain.
- The current Federation service is a scaffold and `federation_db` is not
  provisioned. `FED10` cannot be passed by issuing a shorter LiveKit JWT alone;
  enforce active media-path fencing and measure it.
- Independent provider proof, Q11 bootstrap and operational Q12 values are
  design-and-build tasks owned by this sprint. A fake provider proves the
  contract; a real selected provider must pass the acceptance path before
  production admission.
- This plan contains no calendar estimate: no team size, compatible host target
  or measured capacity exists yet. Those are T08/T93 outputs.
