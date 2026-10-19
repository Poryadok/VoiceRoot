# BE-011: current MM source composition

## Purpose

Finish the existing native whole-party matchmaking implementation on the current
MASTER candidate. A standalone MM024 rollback correction has no consumer until
the actual schema, native capture, validators and transition owners are composed.
This checkpoint is source preservation, not a completed serving release.

## Context

Canon: `docs/features/matchmaking.md`, `docs/microservices/matchmaking-service.md`,
`docs/DATA_STORES.md`, `docs/TESTING.md`, BE-011 accepted c16/c23/c53/c86-c87,
and BE012 async settlement c30-c31. Developer145 owns MM consumer/root/candidates;
147 owns provider settlement transitions;359 owns the admission producer/wire.
The initial confirmed candidate was78bc80b975bc7df00c8e8902ce58186b0a4e29f7;
this branch fast-forwards its unchanged MM baseline to5113a511c5a13f1abe3a760a614641809ee7a76d.

## Scope and decisions

Restore only128 reviewed MM/migration/partyauthority paths from52b68ea/829.
Do not merge their historical Voice015 graph. Add the accepted protected snapshot
RPC/authority tuples and cancellation reason to CURRENT protos; regenerate Go
and Dart mirrors, preserving current RoleMutation and Voice H4 contracts.
Consume147's exact0ab5143c FIRST-gate correction after the restoration.

The compatibility root now actually checks its local database before JWKS.
Its old readiness fixture used an unreachable hardcoded database. Keep the real
startup requirement and provide an isolated actual PostgreSQL database instead.
No production guard is relaxed to satisfy the test.

## Milestones and progress

- [x] Confirm current source: MM baseline lacks019-025, candidates and guards;
  current359 mandatory peer wire is already present.
- [x] Compose reviewed implementation and regenerate additive contracts.
- [x] Run current Linux short/build checks and real PostgreSQL/Redis checks.
- [x] Incorporate147 FIRST admission before account locks and verify its actual
  held-closer, commit and closed-settlement cases in the composed source.
- [ ] Consume359's immutable private-FD bootstrap delivery API and proof.
- [ ] Implement the actual fixed native root, required factory/validator wiring,
  fresh external observations and workload close/drain barrier together.
- [ ] Prove positive native serving and negative config/close/restore cases,
  complete independent current-union review, then offer finished work to121.

## Validation

Actual owned Linux Go1.26.5/PostgreSQL16.15, no host ports or Docker socket:

- `go test -short -p 2 -count=1 -json ./...`:251 test passes,246 explicit skips.
- PostgreSQL boundary selection:23 top-level passes; seven handler subtests
  initially skipped for missing Redis and then actually passed7/7 with Redis.
- Real root startup/JWKS regressions:4 top-level passes, no skips.
- Composed147 admission plus candidate/origin regressions:6 top-level and3
  subtest passes, no skips, race12.633s; full-module vet exit0.
- Current proto generation, buf lint and breaking against78bc: exit0.

Raw inputs/hashes/logs are retained in ignored
`tmp/swarm/rudeus145-mm-current/` in the main repository. Input source tar plus
fixture and147 delta tars bind the executed source; unrelated5113 Auth/paid
changes are not claimed as tested by these MM commands.

## Risks and executable continuation

BE-011 is unfinished and must not be merged as a completed task. Current main
does not install the native snapshot client/validators or qualified native root.
Transport/profile/SQL equality cannot substitute for external authority.
359 confirmed that the independent LaunchExpectations/client-key delivery loader
was absent in3e3e48c. Its existing producer task now owns the concrete one-shot
private-FD ABI `LoadNativeReadbackClient(ctx, descriptorBytes, timeout)`. Resume145
when that immutable implementation and source proof exist; public pins or Observe
must never supply its independent inputs. No installed authority is inferred.

121 alone publishes source and owns CI/MASTER intake. Preserve this checkpoint
without offering the unfinished unit for merge. No Voice/Space owner writes,
new queue work, agents, PRs or stopped-operation recovery belong to this stage.

## Active native serving continuation

Current worktree32, lease42a22873, branch codex/be011-mm-native-145 starts
from fd43a284, normal-merges e779744 and reviewed35914729. Combined events
mirrors were regenerated to preserve BE080. CodeGraph was called before new
root/shared scopes; frozen index warning remains, one bounded call terminated.

Implemented TDD components: forward registration through observation/dispatch/
receipt, irreversible close/drain including timed-out close; all eight D/I/P
input combinations and each missing profile; actual PG system/database/schema
observation and restricted LOGIN/head/history/column grant checks. These are
not a completed native executable. Wire native_consumer getter/peer addition
is new source requiring its own review, not covered by35914729 CLEAR.

Accepted332c117-c120 adds response operation_id field3. Implement client-stable
retry ID, exact final-path-bound Gateway delegated principal on protected mTLS,
opaque responseprincipal subject, durable account/profile/op semantic receipt
atomic with validated proposal response. Exact recorded retry is read-only and
never re-dispatches under CLOSED; decline cannot use terminal cleanup to requeue
unqualified parties. Direct342/69 nonoverlap and fresh TT reservation confirmed
in BE011c121. No new task, actor, controller or owner lookup is needed.

Next implementation order: consume real359 privateFD in fixed native command;
verify executable/config/local database and same authenticated startup stamp;
construct actual mTLS snapshot/provider, principal/replay, broker/roster and
queue factories without delivery; private workload close/status and pending
provider inventory; OPEN-only typed serving and bounded forward loops. Hold
registration around every live provider path through durable receipt, never
SQL locks over network. Complete actual client-to-Gateway-to-native response
positive and negative fixtures, root/privateFD and close tests before offering
finished work for MASTER. Existing terminal settlement stays independent.
