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
- [ ] **T05** `D: T03,T04` Freeze resource/state model and ownership: account,
  selected profile/alias, character, app/env/installation, binding, party,
  match/fleet, corporation→Space, grant reasons, operations and tombstones.
  Update DATA_MODEL/DATA_STORES/CONTRACT_MATRIX as schemas land.
- [ ] **T06** `D: T04,T05` Review OpenAPI/proto/JSON for public Game API and S2S:
  version negotiation, errors, canonical IDs, idempotency, pagination,
  capability fallback, signed envelope/revision, required security fields.
  Add contract tests and generated-code compatibility checks.
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
  This vertical is the durable receiver foundation consumed by T56; T56 remains
  responsible for integrating actual GIS authority and game-specific state.
  - [x] T07a local proof: `TestT07aPermitAuthoritySerializesIssueAndRevoke`,
    `TestT07aCallbackPersistsPermitAndImmutableReceiptThenReplays`,
    `TestT07aAuthorizationDenialAndAdmissionFailuresHaveNoEffect`, and
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
    T03/T04/T06 parents remain open.
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
- [ ] **T08** `D: T03` Define the measurement record and harness for revocation,
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
- [ ] **T11** `D: T10` Complete the developer registry lifecycle. The bounded
  implementation covers owner-derived application creation, separate operator
  sandbox approval, app/environment-scoped policy and installation
  configuration, provider, redirect/origin and callback configuration, and
  credential issue/retry, rotation and revoke. Before enabling `game.events.write`,
  each event-enabled installation must also be bound to a live Bot ID owned by
  the application owner. Empty-database API bootstrap
  and cross-scope denials pass without direct SQL or a portal. Its T10 database
  isolation prerequisite is now proven. T11 remains open: the staged operator
  route can create only a pending production environment and owner policy; it
  cannot activate production. Live provider/user proof and out-of-band
  production secret provisioning are still required. The seeded-fixture test
  denies production credentials and makes no claim of production onboarding. See the
  GIS service contract and
  [Q11 T11 evidence](game-integrations-acceptance.md#q11-bootstrap-evidence-api-only-clean-start-passed-real-google-gate-open).
  The sandbox installation authority slice is implemented: registration takes
  a Bot ID, derives owner only from the application row, and persists the
  binding only after a protected, replay-resistant GIS→Bot owner/live proof.
  BOT11 acceptance covers the exact request/response authentication, owner and
  lifecycle denials, durable binding/idempotency, and clean-bootstrap flow.
  Proof failure may write a sanitized denial audit, but never an installation
  or successful idempotency result. This bounded slice does not close T11's
  production-admission gate.
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
- [ ] **T14** `D: T13` Add browser/device authorization, PKCE, explicit selected
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
  remain pending.
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
- [ ] **T15** `D: T13,T14` Implement the frozen Auth-owned ES256/P-256 device
  key lifecycle, RFC 8785/JWS v1 message bytes, Auth RS256 status assertions,
  Messaging EdDSA tombstones, revision chain, File manifest and exact-receipt
  ordering from the API contract. Decision gate (enrollment/recovery/rotation/
  revoke, receiver verification, fail-closed clock/outage behavior and ≤4.25s
  node revoke ceiling) is closed; runtime, conformance vectors and measured
  revoke propagation remain open. Auth authority issuance also consumes the
  T16 Auth/GIS execution-permit producer and must fail closed until it is available; never
  synthesize `binding_id` from another identifier. Messaging must additionally
  consume the T30/T31 exact app/environment/binding/chat resource mapping;
  Auth binding authority and Chat membership alone cannot prove that link, so
  game-authored writes remain fail-closed until that producer ships. Each
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
  Direct target-chat authorization remains T30/T31 mapping+ChatGuard. Real
  provider, live deployment and production admission are not claimed.
  - [x] **T16-DEV** `D: T14-DEV,T15-DEV` Complete the hosted Linux Compose
    cross-service acceptance described in [game-integrations acceptance](game-integrations-acceptance.md#t16-dev-cross-service-permit-acceptance).
    The production Auth/GIS services, GIS PostgreSQL handlers/store, and
    Messaging gRPC path passed on commit
    `e1fbe949e440f0ddfc2d1e5788b7af2f689901da` in [run 36609945598](https://github.com/Poryadok/VoiceRoot/actions/runs/36609945598).
    This hosted proof alone does not close T30/T31 producer work.
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
- [ ] **T31** `D: T30` Implement the normative durable orchestration contract in
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
    cleanup passed. Other SE02 lifecycle requirements remain tracked above.
- [ ] **T32** `D: T31` Implement stable party/match lifecycle, host transfer,
  explicit close, late join and roster freshness under the frozen contracts: a
  complete roster lease is 60s from GIS DB commit; retries do not renew; host
  transfer requires a complete next-revision CAS and existing successor; exact
  expiry denies admission/reconnect/governed reads+writes and fences Voice/media
  within 5s. A failed match cannot delete shared party chat. See API/GIS T32
  contract sections and SE03/SE04/SE07 acceptance. Runtime remains open.
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
- [ ] **T51** `D: T50,T11,T16,T30,T31` Accept signed game event into durable
  inbox/outbox with stable event ID/payload hash, own installation/recipient/
  character validation, expiry, 409 mismatch and idempotent Messaging
  publication. GIS installation-to-Bot binding, active binding authority and
  app-linked chat mapping must be available before runtime activation.
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
- [ ] **T56** `D: T55,T07a` Controlled game backend verifies actor, binding,
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
| G02, G03, Q01, Q09 | alt-rank merge rule, protected Owner recovery, Q01 frozen by T32 as immutable `created_at` inside membership interval `[joined_at, revoked_at)`; rejoin opens a new interval with no gap access; block/report in shared chat | T32–T33,T37–T39; SE04 |
| G04, G09 | G04 frozen by T32: match access ends exclusively at close+30d; terminal receipts retry for 30d; non-content external-key tombstone persists. G09 frozen by T32: complete accepted roster lease is 60s from GIS DB commit; exact retry inert, stale lower rev no-op, same-rev changed body conflicts, incomplete/failed fetch is never empty, expiry fails closed with ≤5s media fence. | T30–T39; SE03/SE07 |
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
  acceptance and plan docs. This is contract-only progress; T51 runtime remains
  open behind T11 installation-to-Bot binding, T16 authority and T30/T31 chat
  mapping. See the [Game Event v1 contract](../architecture/game-integration-api.md#t51-game-event-v1-ingress-and-publication-contract).
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
