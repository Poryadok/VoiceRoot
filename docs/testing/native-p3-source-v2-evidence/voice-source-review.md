# Voice current source applicability — candidate, not admission

Candidate S: 9df1fa83e2cb8741c7fdfc1003658131ccc267b7. Historical H: fa2eac69.
Root source review; independent f13 review and fresh runner evidence pending.
The pinned historical matrix has eight PASS clauses. No present-head PASS is
inferred solely from those values or from successful unrelated CI modules.

## Changed production semantics

- `voice/main.go` removes the insecure ordinary Role client and assigns the
  existing protected rolegrant Checker to RolePermissionChecker in its existing
  initialization block. No native factory, roster relay, P3 drain, coordinator,
  provider or NATS activation branch changes.
- New `internal/rolegrant/permissions.go` implements five existing permission
  queries. Each requires canonical nonzero identities, exact existing Role RPC,
  fresh request UUID, whole-body hash, service principal, replacement outgoing
  metadata and existing finite timeout. Nil configuration, nil response, error,
  cancellation and denied response do not allow the action. This is permission
  observation, not roster publication or a retirement proof.
- `ordinary_control.go` captures room/intent and observed membership before
  User/Auth IO; inside existing Store.Change it rejects a replaced intent,
  account/profile, generation, fence operation or media identity before any
  original termination effects. Existing transaction/counter/writer code and
  the downstream termination switch are unchanged. This narrows stale request
  admission; it does not add a writer or alter restart persistence.
- `000012_ordinary_calls.down.sql` adds READ COMMITTED refusal and locks all four
  affected tables before the unchanged retention predicate and unchanged DDL.
  Locks remain through DDL. Inherited snapshots are refused rather than assumed
  fresh. Current synchronized first-writer and snapshot tests are required.
- `space_media_runtime_test.go` adds an actual NATS readiness log condition to
  the existing listening-port condition. It changes fixture startup only.

Generated/proto/module metadata applicability is separately owned by Messaging
and Bot reviewers; compilation alone is insufficient to assert semantic parity.

## Shared owner-source context

`authoritysource/postgres.go` extracts catalog validation for an already-owned
native transaction and batches trigger/function catalog reads. Source schema,
relation persistence/RLS/rules/inheritance, trigger identity/type/body argument,
function namespace/language/volatility/security/return type and body hash remain
mandatory. Declared names retain PostgreSQL name truncation. Per-function ordinal
uniqueness plus exact count reject overload ambiguity and missing entries. Read
transactions retain catalog locks; ValidateTx requires the caller's domain fence.
Current qualified native Role/Space PostgreSQL evidence and overload-negative
evidence must be bound by the Role/Space reviewer, not replaced by this inspection.

`authoritysource/server.go` permits QualifiedSchemaVersion only for Role and
Space owners, preserving the original schema for other owners and all identifier
and unknown-field checks. `integrationtest/authority_migrator.go` adds an explicit
server-image option while retaining the default PostgreSQL16 path and pinned
migration driver. Failure logs are bounded and redact private fixture URL/password.
It does not change a successful migration or synthesize a receipt.

## Clause applicability boundaries

- A1/A2: historical causal RED remains historical; fresh current root/factory
  and protected Role checker results are required because compiled inputs changed.
- A3: stale Leave regression supplements the unchanged transaction/writer core.
  The historical real process restart proof is reusable only after its exact
  driver and semantic inputs are independently matched; a Store reopen is not a
  substitute for process restart.
- A4: no changed source introduces an upgrade from EJECT_ONLY or UNKNOWN. Exact
  unchanged P3/provider semantic inputs and changed dependency compatibility still
  require independent evidence applicability review.
- B1–B3: fresh ordinary012 empty roundtrip, inherited-snapshot and synchronized
  first-writer tests supplement the changed guard/locking behavior. Empty roundtrip
  alone is not the literal unrelated-schema/data sentinel proof. Old013/014/P3
  literal claims remain limited to their unchanged input sets; any additional
  required012 literal evidence must be supplied, not asserted from these tests.
- B4: exact013/014 SQL and no-MM020 dependency comparison must preserve the old
  retained outbox/counter refusal. This document does not grant runtime activation,
  deployment or a blanket exception for unrelated Role/Space/PR paths.

Fresh test logs are collected beside this document. Successor-record author must
retain whole projection/context, actual event/base/result parity and real producer
history/taint checks; these notes are not independently consumable authority.

## Completed fresh checks
Fresh 9df Linux Go1.26.5 factory tests and protected Role permission binding pass without skips (logs under WT43 tmp/pr875-repair/f13-current-voice). Explicit nonempty VOICE_COMPOSITION_TEST_DATABASE_URL rerun of TestProductionSpaceMediaFactoryKeepsNativeRosterDormant passes both space_enabled_false and space_enabled_true and executes the real child-process database reopen/payload check. Log: voice-native-restart.json.log. Source blobs remained unchanged.
Stale Leave and three ordinary012 PostgreSQL16 guard/roundtrip tests pass without skips; result counts and SHA manifests are in this main-checkout evidence directory.
Independent f13 reviewer accepts bounded B1-B4 original literal proof reuse after original driver/receipt matching: B1 fixture excludes012 and only exercises014 DOWN/UP; B2/B3 have their own successful receipts; B4 unchanged013 scope. These checks do not assert global present-head admission. Remaining wholeunit generated owner-behavior mappings belong to the successor author/reviewer.
