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
- [ ] Persist immutable Story offer/consent with exact replay and guarded
  current-offer checks; prove real PostgreSQL races and rollback behavior.
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
- [ ] Immutable first-wire consumer pin, exact-head independent review.

## Decisions

Keep private RPCs in the existing Story protobuf package to avoid introducing
a new cross-service module. Registry service is implemented by MM, offer
validation service by Story. Receipt bytes alone convey no authentication;
only the exact protected Story call can establish provenance at registration.
Protocol codecs never mint identity or current offer/intent authority.

## Risks And Follow-Ups

High risk: signed identity, privacy, immutable replay and two-database ambiguous
acknowledgement. Mandatory independent review. Pair support does not implement
N-member consent; unsupported groups deny without dropping peers. Story restore
must not reuse an offer incarnation/revision. Exact deployment/credential and
account erasure composition remain separate acceptance concerns.
