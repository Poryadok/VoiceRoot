# ExecPlan: protected LFP consent producer

## Purpose

Produce an immutable, authenticated Story responder consent for the existing
pair JOIN/INVITE flow. Matchmaking consumer145 must receive an exact wire pin
before implementing its distinct registry and atomic acceptance.

## Context

Behavior: `docs/features/matchmaking.md` Social Discovery,
`docs/microservices/story-service.md`, and accepted BE-011 c23
BE011-LFP-CONSENT-SOURCE-v1 (artifact SHA256
`c84d4f974e1304a967324c51e3c4aa1ff10f3607d41b138c6bca96616f7dd9c0`).
Baseline `0a0024900b5bb3ba2f6fee391615f0f4fa0864e3` Story only reads a raw
profile header and publishes a best-effort profile-only event. It cannot issue
qualified identity evidence. Story owns offer and responder consent; MM owns
registration/search-intent capture, author decision and accepted party source.

## Scope

Story/Gateway protected responder boundary, minimal immutable offer and consent
journal, exact private registration/readback and offer validation. Keep public
events profile-only and default activation gates unchanged. No MM registry,
acceptance/migrations, native roster, runtime rollout or provider authority.

## Milestones

- [x] Pin additive private protobuf, strict canonical receipt codec and golden
  vectors for immediate consumer145.
- [x] Implement component journal with immutable offer/consent, exact replay,
  guarded current-offer checks and real PostgreSQL race/rollback checks.
- [ ] Wire verified delegated responder plus narrow service authentication,
  durable registration/readback and exact fresh offer validation.
- [ ] Independent security/concurrency review and accepted121 delivery.

## Detailed Steps

1. Confirm actual allocation and shared Gateway route ownership before edits.
2. Test canonical consent validation: identity/version/expiry/type/digests,
   unknown/duplicate/reordered/overlong fields, copied buffers and stable hashes.
3. Add private Story-package RPCs; deterministic protobuf uses explicit domains,
   UUID bytes16, hash bytes32, positive generations, no maps. Registration ack
   carries MM's original capture identity; retries never select a fresh intent.
4. Reserve a free Story migration only after current catalog inspection. Lock
   exact Story/operation, recheck offer; never make remote calls under SQL locks.
5. Test forged/raw/duplicate/wrong-method identity, lost registration ack and
   changed-body replay; complete production wiring after component proofs.
6. Freeze source/proof and review exact head, then transfer delivery to121 only
   after explicit acceptance.

## Validation

Owned Linux Go1.26 tests/race/vet, actual PostgreSQL integration for journal,
Buf lint/breaking/generated parity for proto; protected TLS/JWKS/replay/caller
matrix for transport. No Windows generated executables or host listeners. No
DEV/STAGING qualification without actual resource claim/window and observation.
CI and product acceptance remain distinct from source/component proof.

## Progress

- [x] Atomically claimed function; consumer145 acknowledged disjoint scope.
- [x] Own Treehouse33 lease `dac427080ca9c0cfd401822c14023d92`, branch
  `codex/be011-lfp-consent-291`, exact baseline above.
- [x] Meaningful Linux codec RED2fail -> consent GREEN3top/26sub; registration
  RED1fail -> combined GREEN5top/32sub0skip under race (1.013s), vet0.
- [x] Independent Python literal-field goldens plus generated protobuf parity
  for JOIN, INVITE and self-hash-excluded registration (2tests/race1.021s), vet0.
- [x] Buf lint/breaking master and canonical Go/Dart generation succeeded.
  Initial Buf response-name error and host WSL bash absence were setup failures,
  corrected using canonical response wrappers and installed Git Bash.
- [x] Immutable first-wire pin `5b33b6e3e8a95428e053fe7a6e53f772ca7a8884`,
  independently SOURCE CLEAR by305; source-only publication by121. No full-task
  delivery or runtime acceptance transferred.
- [x] Journal RED4 -> GREEN4; registration RED1; cohort journal GREEN7;
  malformed metadata-import RED1top/2sub reproduced a real WIP defect before
  strict relational/canonical validation fixed it.
- [x] Shared admission ABI314 `468e807efd807705c191a713c493046de126e6a6`
  consumed as additive commit `ab9cb4ba6`. Own bound factory holds its permit
  before SQL locks until commit/rollback. Component authority is synthetic.
- [x] Journal Linux race9top/2sub0skip (9.999s), vet0, including a deferred
  PostgreSQL constraint trigger that blocks actual COMMIT while close/drain
  remains incomplete. No installed controller or real restore proof inferred.
- [ ] Freeze journal pin, independent exact-head review and producer transport.
- [x] Review147 found P1: pgx COMMIT error/ErrTxClosed did not prove native
  transaction completion. Real PostgreSQL canceled-caller/deferred-COMMIT RED
  reproduced false successful drain. Completion guard now releases only after
  confirmed COMMIT or confirmed pre-commit ROLLBACK. Unknown BEGIN, COMMIT or
  ROLLBACK retains restrictive admission; no timeout/reopen/connection-close
  assumption releases it. Actual old execution/database-session retirement is
  required to replace that runtime. Added canceled admission-head rollback and
  confirmed ordinary rejection rollback oracles; independent recheck remains.
- [x] P1 successor `c58d3d71b3c0d42338016a12127f19fa8a1ea09e`
  independently SOURCE CLEAR147, parent c11. Exact archive race12top/2sub0skip
  11.879s and vet0; first exact run interrupted by own runner sleep expiry,
  preserved separately without PASS credit.
- [x] Implement component private Offer validation and Registry producer with
  exact Read-before-Register recovery, mandatory actual peer authority and
  pre-network same-cohort durable receipt loading. Fresh Story visibility is a
  mandatory bound dependency; no production adapter exists. Linux race14 Store
  tests/2sub0skip10.957s and transport2top/13sub0skip1.261s, vet0.
- [x] Add actor-neutral private action protobuf: stable publication/response IDs,
  exact offer view, no public account/epoch/receipt bytes. Buf lint, Go/Dart
  generation succeeded; delegated action handlers remain to implement.
- [x] Resolve c12 actor/account-type sideproof with Astra301 before implementing
  real eligibility. Canon permits contextual guest flows and separate guest
  privacy; regular-only is not an assumed replacement. No current-ID inference.
- [x] Accepted301 c14 freezes12-tag actor codec, mandatory qualified consent/ACK/
  source domains and NEW ActorBound wrapper methods. Independent literal Python
  goldens, strict codec negatives and generated parity cover both human types;
  v1 bytes/domains/goldens remain unchanged. Codec construction is data only.
- [x] Implement additive migration000006 (reserved c15 after actual catalog and
  fresh search), atomic immutable publication/responder actor sidecars and
  qualified ACK. Original guest/type/epoch survives exact replay; legacy rows
  cannot be enriched. Immutable audience/criteria context uses the existing
  offer state hash, preventing mutable audience A->B->A substitution. Actual
  Linux PostgreSQL race22top/4sub0skip22.761s and vet0 cover concurrency,
  sidecar/ACK rollback, cohort rejection and credential expiry after row lock.
  Synthetic actor/admission fixtures are component evidence only.
- [x] Correct independent review350 DOWN first-writer race. Actual PostgreSQL
  RED reproduced evidence removal for both000006 and analogous000005. Each
  DOWN now obtains admission-head and target relation ACCESS EXCLUSIVE locks
  before checking retained rows, and checks/drops inside one DO transaction.
  Non-READ-COMMITTED transactions refuse rollback rather than use an old
  snapshot. Focused Linux race2top/4sub0skip6.100s and vet0 cover both first
  writers, genuinely empty rollback and snapshot isolation rejection. Exact
  successor review remains required; this is not runtime restore proof.
- [x] Implement ActorBound Validate component and original-viewer privacy
  adapter. Exact signed service request + actual authenticated peer authority,
  original actor/current immutable audience read, fresh floor/privacy and final
  SQL recheck are mandatory. Add required canonical_criteria_json field6 to the
  response; exact PostgreSQL bytes must match unchanged v1 criteriaSHA, with
  client hash/shape checks before MM capture. No public-listing fallback.
  Actual Linux mTLS + PostgreSQL tests cover both guest and regular viewers;
  supplied admission/peer/floor fixtures do not prove installed authority.
- [x] Add protected HumanVerifier data capture with explicit authenticated type
  and original operation/body projection. Genuine RED reproduced a live epoch
  callback outliving credential expiry; expiry now rechecks after dependencies.
- [x] Implement ActorBound RegistryProducer and delegated Story action handlers.
  Producer reloads exact sidecar/cohort before EACH outbound request, reads
  original MM capture first and permits Register only on authenticated definite
  absence. Full base/Q/Origin/boundACK checks precede atomic local persistence.
  Action handlers verify actual Gateway human actions; immutable replay lookup
  precedes fresh candidate privacy and returns original guest/type/epoch/MM
  witness without recapture. Publish/Observe expose only profile-scoped views;
  Respond returns an empty success only after original ACK is durable.
  Actual Linux PostgreSQL + mTLS guest AND regular JOIN/INVITE, lostACK,
  conversion replay, privacy denial, local ACK rollback and cohort retirement
  tests pass: transport11top/57sub0skip17.475s, added Store replay1top2.759s,
  race and vet0. Admission/MM capture-policy fixtures are synthetic.
- [ ] Complete Gateway routes and actual protected root/controller/retention
  composition. Original bare-v1 components cannot qualify the actor-bound path.

## Decisions

Keep private RPCs in the existing Story protobuf package to avoid introducing
a new cross-service module. Registry service is implemented by MM, offer
validation service by Story. Receipt bytes alone convey no authentication;
only the exact protected Story call can establish provenance at registration.
Protocol codecs never mint identity or current offer/intent authority.

Accepted restore-cohort decision is BE011-LFP-CONSENT-PRODUCER c7,
`BE011-STORY-LFP-RESTORE-COHORT-v1`, artifact SHA256
`592a5bb6bb9e6dda50520f42f358d3f5727a3bf38d80c44f3a3f5274b59d775b`.
Database revision/UUIDs cannot detect restore. Immutable origin accompanies
offer/consent and MM registration separately from unchanged v1 wire bytes.
`BE011-STORY-LFP-RESTORE-CONTROLLER` owns the external authority/composition
continuation; consumers are291/145. Its actual deployment-owned controller,
credential/route retirement and replica drain remain absent prerequisites.
Missing authority never means clean installation or an open qualified path.

## Risks And Follow-Ups

High risk: signed identity, privacy, immutable replay and two-database ambiguous
acknowledgement. Mandatory independent review. Pair support does not implement
N-member consent; unsupported groups deny without dropping peers. Story restore
must not reuse an offer incarnation/revision. Exact deployment/credential and
account erasure composition remain separate acceptance concerns.
Migration000005 has no initial admission row and preserves immutable evidence
without cascading Story deletion. These are personal data, with no retention
exemption: actual account-erasure/retention composition is still required before
activation. Down migration refuses retained journal/head data. Synthetic local
cohort rebind and malformed-import fixtures do not prove authorized restoration.
