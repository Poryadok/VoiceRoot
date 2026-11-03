# ExecPlan: Auth-purpose finite User profile authority

## Purpose

Give Auth protected evidence for its own finite profile-reference inventory and
the current account binding of an ownership-transfer target. This is a BE-019
coordinator prerequisite; BE-020 still owns User physical cleanup. No deployment
activation or complete account-erasure claim is included.

## Context

Sources: `docs/microservices/auth-service.md` terminal account-erasure receipts;
`docs/microservices/user-service.md` protected capabilities and qualified identity
source; `protos/voice/identityerasure/v1/identity_erasure.proto`; User
`identityassociation/{account_inventory,lookup,batch,transaction}.go`.
The live profile list omits retired profiles. Captured account inventory explicitly
does not certify complete historical coverage. Stored selectors cannot enumerate
lost raw profile IDs. Reuse canonical local facts and cryptographic plumbing,
never Social, Role or SDK authorization grants.

## Scope

One new Auth-only service with `MatchAuthProfileReferences` and
`ResolveAuthOwnershipTarget`; canonical proto and targeted User generated code;
shared contract validation; qualified User reader, protected adapter/listener;
PostgreSQL, transport and negative tests. No Auth Java, cross-database writes,
remote calls under database locks, profile capture/backfill, TT changes or deploy.

## Contract and decisions

- Version 1; canonical nonzero UUIDs; sorted unique finite batches of 1..256
  profile IDs. Requests bind request ID and caller inventory ID/revision,
  account, opaque owner/cycle; owner/cycle are echoed Auth context, not User facts.
- Matching returns exactly one MATCH, DIFFERENT_ACCOUNT or INCOMPLETE per input.
  DIFFERENT_ACCOUNT requires positive complete canonical account-alias evidence.
  Missing/uncaptured history or retained-key aliases never implies non-match.
  `all_requested_resolved` covers only the supplied batch, not an account roster.
- Current target returns CURRENT_PROFILE plus private account binding and positive
  revision/flags, TERMINAL_PROFILE only from canonical terminal evidence, or
  INCOMPLETE. CURRENT_PROFILE is an observation, never an admission grant.
- Source cut binds User source/authority revision, history/head/digest,
  keyset revision and retained floor; counters report unresolved historical gaps.
  A cut/source pin alone does not prove independently current source custody.
- The qualified source stream is held through the actual IdentityTx and commit.
  Complete retained keys, exact physical source and guarded catalog are checked.
  Missing policy/source fails closed. Prepare selectors before identity mutex;
  perform authoritative reads in later READ COMMITTED statements and publish only
  after commit. No fact capture or mutation is authorized by this read protocol.
- New `auth-profile-authority` mTLS capability, exact service:auth/issuer auth/
  audience user, fixed methods, deterministic request hash and shared Redis replay.
  Existing SDK, Social and Role allowlists stay unchanged. No default activation.
- Auth will perform local actor/target/lifetime admission after remote observation.
  This addresses parent-account P30D; it does not linearize independent User
  profile retirement. Response/request digests are ephemeral bindings, not fields
  for permanent terminal fences. Receipts never gate the original P30D purge.

## Milestones and detailed steps

1. Record contract; add executable matcher regression with real PostgreSQL.
2. Implement finite read under qualified source and exact identity transaction;
   cover retained/retired/foreign/unknown/missing-key and concurrent mutex cases.
3. Define proto and validators; generate only this User Go protocol; add narrow
   protected handler and listener with source/config/replay denial.
4. Run affected package checks, protobuf lint/format and compile; submit source,
   tests and evidence for independent privacy/concurrency review.

## Validation

- `go test ./internal/identityassociation -run AuthProfile -count=1 -v`
- `go test ./internal/sourcecustody -run AuthProfile -count=1 -v`
- `go test ./internal/grpcsvc -run AuthProfile -count=1 -v`
- `go test ./... -run AuthProfile -count=1` in User and affected shared packages.
- `buf lint` and `buf format -d --exit-code` for changed proto; targeted generated
  code reproducibility, User compile, actual TLS/replay denial where relevant.

## Progress

- [x] Design approved; isolated branch at d6d2b9679412, same author tests/code.
- [x] Real PostgreSQL RED and initial matcher GREEN (five tests, 9.333s).
- [x] Shared contract/capability checks: 151 tests pass; User compilation passes.
- [x] Protected adapter/transport checks: six tests pass, including real mTLS,
  signed Auth JWKS, exact request binding and shared-miniredis replica replay.
- [x] New protobuf lint/format pass; targeted Go generation uses installed protoc.
- [x] Qualified-source held-permit read checkpoint: serial PostgreSQL run
  passed in 14.38s with unchanged strict bounds. It observes the held control
  permit at the pgx commit boundary, cancels without publishing a read result,
  rejects a closed source and proves no history-head mutation.
- [x] Independent privacy/concurrency review: no blocking findings. The final
  consumer validator now requires erasure-issued == covered, matching the
  producer; focused unequal-counter mutation failed before this hardening and
  passes after it, with positive equal-nonzero coverage.
- [x] Shared and affected User package linters: zero issues. User short run had
  596 passing tests, 313 deliberate short-mode skips and one existing 100 ms
  health-startup deadline failure; the exact startup rerun plus new dedicated
  partial-configuration tests passed (11 tests), without changing its timeout.
- [ ] Final AST refresh and commit/push.

### Resume checkpoint (2026-11-03)

Original matcher behavioral RED and GREEN are preserved in the dispatcher-owned
`tmp/astrie-be019/user-match-{red,green}.log`. Capability isolation first failed
with the new purpose missing, then passed after its narrow registration. The
new request validator first failed compilation with the missing function; this
is API-shape evidence, not a behavioral RED claim.

The resumed implementation adds a held qualified-stream read, shared wire
validation, dedicated adapter and opt-in process listener. Runtime configuration
requires the existing guarded qualified User source runtime; it does not reuse
Space's caller grant. No capture/backfill is invoked by either Auth method.

Fresh matcher runs initially failed strict two-second bounds under observed 100% host CPU,
including the unchanged native fixture activation path. A later serialized run
is recorded separately in `user-match-resume-green.log`; it failed and its
filename does not assert success. A subsequent exact positive matcher test passed
in 15.24s. No timeout or assertion was loosened. Qualified-source fixture startup
also failed twice before the tested reader; after the shared heavy PostgreSQL
window ended, the same test passed (`user-match-custody-serial.log`, session
59526 exit 0). Initial five-test matcher GREEN remains valid for the unchanged
behavior; the deferred rollback lint fix only explicitly discards its cleanup
error. No production source qualification is claimed by these synthetic OPEN
fixtures.

Fresh CodeGraph inspection was initially rejected by another live writer
lock (PID 34636); no process or lock was modified. Completed exact inspections
and Graphify queries were used; later direct CodeGraph inspections succeeded.
Required AST refresh is still pending. Five observed live Graphify updates were
reconciled by the dispatcher to foreign checkouts; no foreign process or output
was modified, and no duplicate update was launched. No deployment, tracker
transition or final acceptance is claimed.

## Risks and follow-ups

Historical evidence may legitimately remain INCOMPLETE; this change cannot recover
lost prehistory. Independent User profile retirement needs its own admission/fence
protocol for linearization. Source qualification is mandatory, not created by this
listener. Auth client/coordinator, terminal cleanup, fleet acceptance and deployment
are separate integration work. Larger inventories need Auth-side complete local
inventory closure and recheck across these finite responses.
