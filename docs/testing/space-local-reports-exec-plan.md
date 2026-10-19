# ExecPlan: separate Space-local reports

## Purpose

A member can explicitly complain about a readable Space message to that Space's
moderators. A currently authorized local moderator can inspect only permitted
local complaints, assign a reviewer and record a decision. Platform complaints,
reporter identity and platform sanctions remain outside this local surface.

## Context

- Product: [reports.md](../features/reports.md#локальные-жалобы-space).
- Owning API/data/authority: [Moderation](../microservices/moderation-service.md#space-local-report-jurisdiction-accepted-target),
  [Role](../microservices/role-service.md#moderation-local-report-decision-contract-accepted-target),
  [Gateway](../microservices/api-gateway.md), [data stores](../DATA_STORES.md).
- TT `voice/BE-066` c3 records the explicit user choice for isolated local
  reports. c4/c5 contain the earlier bounded security analysis and recommended
  product package. Later user delegation directs autonomous ordinary choices;
  this plan selects that narrow A-E package, not fictitious separate human
  approval of its details.
- Acquired source: develop `ba14ae774b2ea7858d7165e5aade9816aec34e90`.
  Existing Moderation RPCs use global `Report` and raw legacy metadata. They are
  not suitable local authority. New API/table/handlers must be isolated.
- Chat owns membership/history intervals; Messaging owns exact message
  visibility, deletion, timestamp and canonical Chat binding. Space owns live
  Space/node ownership. Role owns `MODERATION_MANAGE_REPORTS`. No service may
  read another service's database.

## Scope

Local message reports in main feed/threads of group/channel Space chats;
explicit local-vs-platform submission; private reporter; current content ACL;
typed same-target references; versioned assignment/terminal review; positive
delete/move minimization and lifecycle cleanup. No local profile/voice/Story/DM
targets, copied snapshots, global staff bypass, dual submission, integrated
sanction or local appeal system. Existing global intake semantics are unchanged.

## Milestones

- [x] Record the bounded product choice in owning feature/service/data docs.
- [x] Resolve exact protected target-owner protocol from current implementations:
  Astra327 BE-066 c10, attachment `ab60c723ae5e48ba7359686a21ac44b1`, SHA256
  `6421823c604bd6fb82629e4cf68df57475b96bb2840e7c643dac18f8daf6f2ce`.
- [ ] Add local DTO/RPC/generated contracts, independent persistence and guarded
  rollback. One owner reserves the next actual Moderation migration number.
- [ ] Execute meaningful negative RED, then implement protected ingress,
  authoritative target/Role adapters, per-row current ACL and workflow CAS.
- [ ] Wire Gateway and client local-vs-platform path and cleanup workers.
- [ ] Prove isolated Linux/PG/TLS negative matrix, independent exact-head review,
  accepted publication transfer to121 and assembled hosted CI.
- [ ] Authorized exact-version environment checks and appointed acceptance.

## Detailed Steps

1. Inspect `Chat.CheckMessageReadEntitlement`, viewer-scoped
   `Messaging.GetMessage`, Chat/Space protected ownership surfaces and their
   actual per-caller routing. Never adapt the no-viewer GetMessage as a viewer
   read or forward raw identity metadata. Determine whether one new protected
   owner RPC is required and bind all viewer/Space/Chat/message fields.
2. Make owner result semantics explicit: current allow, known forbidden/missing,
   positive deletion/move and dependency uncertainty are distinct. A current
   reader denial hides data but does not prove deletion. Specify finite list
   authorization before pagination without a caller-controlled tenant filter.
3. Add new local API and table independently of global `reports`. Preserve
   reporter provenance privately and define account/Space cleanup before
   activation. Prove forward/guarded DOWN with real retained local rows.
4. Author negative tests first for spoofed caller, foreign jurisdiction, revoked
   membership, unreadable message and deny/unavailable owner/Role. Implement
   narrow signed ingress and Role caller without relaxing other listeners.
5. Implement atomic workflow transitions and exact terminal replay. Competing
   versions cannot overwrite a decision. Prove authorization failures have zero
   local effects and never call sanction/threshold/event hooks.
6. Add client choice, submission receipt, local queue/detail/assign/resolve UX
   with loading/denied/unavailable/conflict states. Never render private reporter
   or claim an enforcement action occurred solely from Resolve.
7. Add positive delete/move and P3/account cleanup before enabling the route.
   Unknown owner responses preserve evidence and hide content. Rollback of
   cleanup must leave all row/work state unchanged.
8. Freeze immutable source/test artifacts for independent review. Sole publisher
   Рудеус-121 must explicitly ACCEPT delivery/assembled CI custody. Local proof
   is not deployment, hosted CI, privacy deadline proof or acceptance.

## Validation

- Jurisdiction matrix: local Owner/moderator A cannot list/get/assign/resolve
  platform reports or local B, even when IDs/targets are known. A platform Space
  complaint is not exposed to that Space's owner. Global staff has no implicit
  local bypass. UUID version is not authority.
- Identity matrix: raw profile/internal metadata, bad issuer/audience/RPC/body,
  replay, expired token and unavailable JWKS/epoch/Role fail before data/effects.
  Another valid service cannot use the new Moderation Role capability.
- Content matrix: message main-feed/thread visibility, group/channel Space
  ownership, membership history, ghost/FOR_ME/block restrictions, sender change,
  leave/ban/revoke, unknown/deleted/moved target. Reader denial differs from
  positive deletion. Lists never leak unauthorized summaries/cursors/counts.
- Workflow matrix: authorized create/assign/reassign/clear/resolve; unauthorized
  assignee; current revoke before each new request; competing expected versions;
  exact terminal retry and divergent terminal conflict; no-op/rollback has no
  version/event/sanction change. Assignee is not an exclusive decision lock.
- Privacy/lifecycle: no reporter fields in serialized local outputs/events,
  typed same-target evidence only, no copied message body, bound description/note,
  positive delete/move scrub both open and closed content, no cleanup on timeout,
  freeze deny, account/Space cleanup and restore cannot expose old content.
- Real Linux PostgreSQL migration, concurrency and rollback cases; affected Go
  package race/vet/lint; proto format/lint/codegen/contract compatibility;
  Gateway API and Flutter critical-path tests; required exact-head hosted CI.
  Shared DEV/STG checks require actual TT resource reconciliation and claim;
  appointed independent acceptance remains separate.

## Progress

- [x] Read parent decisions, current queue and prior exact-head review custody;
  atomically claim BE-066 as Рудеус-320 before writes.
- [x] Acquire own Treehouse WT44, lease `6d5887b144f73da883ce032170ba953c`,
  branch `codex/be066-space-local-reports` at actual develop base above.
- [x] Direct CodeGraph first; both bounded calls stalled and were terminated.
  No graph freshness proof; immutable/current source reads govern.
- [x] Record bounded protected owner-chain question in TT BE-066 c6; queen
  acknowledged its specialist queue. Consumer remains the real implementation
  owner320; no owner API or runtime capability is invented while unresolved.
- [x] Consume327 c10 and canonize owning Messaging/Chat/Space/Social/Role target
  contracts and the finite page algorithm. These are explicitly new producers.
- [x] Reacquire actual WT44 under NEW lease
  `c6c9648f3aafc3102737c090b70e0f16`, holder `rudeus-320-be066-resume`, branch
  `codex/be066-owner-chain` from current develop
  `c850c2e4d56b2130cf739dbcddde8faf6d72a663`; normal merge preserves own fcfdd docs.
- [x] Reserve Chat26 in TT c11 after fresh scoped search; add native separate
  generation and exact positive retirement receipts in existing source TX.
- [x] Real disposable Linux PostgreSQL canonical schema25 regression failed
  `REPORT_ATTACHMENT_MISSING`; schema1..26 passes core+overflow SQL assertions
  including ABA, delete/recreate, type/creator not move and rollback. Initial
  freeze fixture missed required source-manifest fields and was corrected; that
  setup failure is not hidden as product RED. No runtime handler/feature PASS.
- [x] Canonical Go assertions with backfill/guarded DOWN and actual two-writer
  barrier plus three existing25 regressions: Linux Go1.26.5/real PostgreSQL16,
  `-race`, 4top PASS/0FAIL/0SKIP, package event25.648s. Exact source archive
  `fef19c13d9d1de6dd774d8bc02a11366b2f4433baba53af877888947840e3d2f`.
  Private fixture-only DSN overlay is published separately; test assertions and
  product SQL are unchanged. It does not certify canonical Testcontainers
  startup. Earlier private DinD image-copy preparation was bounded-cancelled
  before tests (partial copy, no engine-outage inference). First DSN overlay
  missed shared PostgresImage constant: build failed before tests, then corrected;
  raw setup-failure logs remain distinct from the actual passing runtime.
- [x] Close the named integration gap in c13: same26 DB now exercises actual
  GetSpaceChatScopeAuthority after move/type and actual
  PrepareSpaceDeletionManifest/freeze; report generation and positive receipt
  remain unchanged on freeze. Final archive
  `d9a67b4c4bf889c41b20c51a68039fc73a439c84b29f65179e28dc2f3398e6d1`,
  real Linux PostgreSQL `-race` 4top PASS/0FAIL/0SKIP36.857s, scoped vet0 and
  runner exit0. The three older tests retain their declared schema25 fixtures;
  the new producer test supplies the explicit schema26 reader integration.
- [x] Independent original contract SOURCE CLEAR144 in BE-066 c19; exact768
  native producer/327 delta SOURCE CLEAR364 in c20 and existing attachment review
  c2. Both are scoped; neither certifies the following new transport source.
- [x] Acquire NEW own WT31 lease `68f528c4680a7ba0fc23d0fd1f86c857`, holder
  `rudeus-320-be066-protected-chat`, from current develop
  `e77469b8b0da38c5cc3bde986c02f4e1aca54fb1`; normal merge reviewed768.
  Add protected Messaging-only exact saved binding observation and current scope
  attachment generation. Unknown/foreign/missing history never proves deletion.
- [x] Exact-source archive
  `41165b011b7f4598a733c5104dffc9827685546d313f2c1f990e27100d3df893`:
  real private PostgreSQL16 + Linux Go1.26.5 `-race`, eight top-level tests
  PASS/0FAIL/0SKIP across store/handler/protected TLS packages; main compiles
  with no matching tests. Scoped vet0/runner0. Same published fixture-only DSN
  overlay qualification applies, not canonical Testcontainers startup. Tests
  cover unknown/foreign binding, MVCC during an uncommitted move, A-B-A,
  deletion, freeze-not-retirement, exact hash/caller and TLS/JTI replay.
  Final config includes all three dedicated callers and confines observation to
  Messaging. Full proto lint/format and wire compatibility against a descriptor
  built from exact develop e774 pass; earlier scoped Git/image path invocations
  selected no files and are not credited. Go mirrors and Dart are regenerated.
  Graphify update was bounded-cancelled after verified own process chain; no
  graph freshness PASS.
- [ ] Independent review of this new transport scope; implement remaining
  protected chain and local scenario without waiting for that review.
  The current reader now requires schema26; the two prior25 reader tests have
  explicit26 fixtures, while backfill assertions still begin at25. No feature PASS.
- [x] Independent executable Social edge: exact `social-messaging-report`
  capability, dedicated mTLS production factory/configuration, bounded unique
  directional pairs, protocol/operation/viewer echoes and ordinary versioned
  request rejection. Existing native account mapping/block reads remain owned by
  Social; the production factory test substitutes only profile-owner mapping.
  Archive `4405d762f3ca8da6d23ba8d44cf628dd498fc9fc1bcf2ae522513f8653439eef`:
  full shared principal package 28 top-level PASS and selected Social packages
  22 PASS, no FAIL/SKIP, Linux `-race`, vet0/runner0. Real PostgreSQL16, real
  private Redis7 for existing shared receiver/JTI/TTL integration; the new
  factory test explicitly uses Miniredis. Fixture-only PG/Redis overlays replace
  container provisioning, preserving assertions; no Testcontainers startup proof.
  The first unadapted hosted Redis test panicked on its missing private Docker
  socket before product execution; raw setup failure is retained separately.
  Full protobuf lint/format/breaking against exact e774 descriptor pass; two Go
  mirrors and Dart regenerated. Own Graphify worker31852 was bounded-cancelled
  after validated process ancestry; handle32266 terminal1, no freshness PASS.
- [x] Resolve the subsequent named Space union-history producer uncertainty:
  Astra381 BE-066 c23 / `DECISION-BE066-EFFECTIVE-EPISODE-v1`, exact durable
  contract SHA256 `fc98f277c7f926228b56c6bf4cf26f0e16d6535632c0befa2b56ab90a277e8c2`.
  It separates effective episode identity from ONE-source admission binding and
  settles the OLD coverage horizon before lease overwrite; legacy cutoff remains
  UNKNOWN. No additional product or general Astra gate remains.
- [ ] Next coherent source: canonicalize381 in owning Space/Messaging docs,
  consume actual native33 baseline `fdd2035fd3e817d280e1f657033a47d949025bca`
  in a real owned lease and coordinate exact source SQL hooks with owner186.
  Owner186 confirms no whole-native integration/activation acceptance: scoped
  source CLEARs are separate, Bot/Ban/GIS account capture and global logical
  finalizers remain open. Do not edit migration33 in place or infer activated
  writers from this immutable pin. Reconcile reservations before a new migration.

## Decisions

BE-066 c10 selects content-free Messaging batch viewer decisions and an independent
positive lifecycle observer. Exact caller/data/visibility contracts are now in
owning Messaging/Chat/Space/Role docs. Social's existing typed pair check needs a
new protected Messaging registration (max100 distinct sender pairs in this
consumer), with exact complete result validation; metadata forwarding is not an
accepted adapter. No new RPC or protected producer is assumed present.

Finite list contract: default50/max100 public items, immutable descending
(created_at,id), private upper watermark and opaque authenticated or server-held
cursor bound to jurisdiction/Space/actor-session/filter/order/last EMITTED tuple/
expiry. Fetch at most1000 private ID/binding/version candidates plus overflow
sentinel under exact Space/filter/watermark before payload access. Overflow is
generic UNAVAILABLE with no page/count/new token. For <=1000 candidates complete
<=10 owner batches of100, current Role per distinct canonical ChatRef (<=1000)
and per-message ACL under one deadline and <=16 concurrent downstream calls.
Any authority error fails the whole page. Only after the complete authorized set
is known apply public LIMIT+1 and version/fence-checked payload projection.
Changed row/fence is UNAVAILABLE. Next cursor exists only for an additional
authorized row and uses last emitted tuple, never hidden scan position; no total,
skipped count, hidden node or empty continuation pages. Every page reauthorizes;
new rows above watermark wait for refresh. This explicit bounded availability
does not guarantee success for arbitrarily large queues or constant-time secrecy.

Positive retirement must distinguish actual Space exit/deletion from Chat's
ordinary type/creator incarnation changes. Chat26 supplies native separate
generation plus immutable exact old-binding receipt. Messaging message
incarnation/tombstone and protected lifecycle read/ack remain separate new
producer work. Moderation serializes create/assign/resolve/cleanup on the same
target fence and row order, records retired-binding fence and scrubs open/closed
description, evidence and user-authored decision notes atomically. Current reads
deny delayed cleanup; per-report reconciliation catches delayed events and
pre-event insertion races. UNKNOWN/absence/viewer deny never scrubs. No receipt
is a new indefinite account-erasure exception.

Select earlier c5 A-E defaults under explicit autonomous delegation. This keeps
v1 within the requested separate local jurisdiction without adding privileged
content disclosure, reporter identity or cross-platform enforcement. Sources and
choices are canonicalized above before behavior code.

Request-time owner/Role authorization is explicit. Each request reads current
owners and performs a local versioned decision; no distributed immediate-revoke
linearizability is claimed. A later in-flight revocation is not solved by JWT
TTL. Stronger cross-service permits would need their own justified contract.

## Risks And Follow-Ups

The current source has no complete protected local-target resolver chain.
Existing viewer reads and Chat entitlement methods must not be broadened by
unsigned metadata or synthetic Space IDs. Exact owner protocol is a bounded
cross-service/security question before code. Reporter/account lifecycle and
list visibility require implementation as part of the same enabled vertical;
an isolated permission check or disabled foundation does not complete BE-066.
No source runtime, migration execution, deployment or acceptance is claimed by
this contract stage.
