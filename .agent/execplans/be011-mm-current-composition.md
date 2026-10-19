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
- [x] Consume359's immutable private-FD bootstrap delivery API and proof.
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

### Native preparation/control components (current continuation)

Actual Linux checks now cover existing canonical NATS metadata (2.12.15 own
fixture, no probe publish/create), both actual provider mTLS transports and
Snapshot transport without fabricated domain calls, irreversible close/drain,
kernel-peer Unix control framing, and full retained provider census including
live claims/backoff. Census overflow is unavailable, not a truncated empty ACK.
The first census runs failed fixture setup (protocol0); only the corrected
protocol1 stub run is the meaningful RED, followed by real-PG GREEN.

The closed bootstrap now calls the genuine fixed-FD359 loader, bounded ARMED
readback and restricted actual DB guard. Local executable checks use the actual
running bytes/fixed argv; descriptor reading requires a regular root-owned file
on a read-only filesystem. These checks do not establish installed custody.
An actual full bootstrap/executable positive has not yet run.

The provider worker's new in-process ForwardLifetime hook remains147-owned;
145 supplies EnterForward and does not wrap ABANDONED cleanup in fresh admission.
Roster mismatch callback is bound before Start and cannot be rebound afterward.
New root must connect it to nonblocking beginClose before actual delivery.

Critical remaining native boundary: existing queue ReadQueueIntent only uses RR
shape validation. Finish native projection authorization with fresh source and
external admission outside SQL, then FIRST RC local admission/account locks and
post-lock ban/cut verification before positive Redis enqueue. RELEASE stays
independent. This is required implementation under332c53, not a new feature.

No native command/image, complete factory startup, protected response receipt or
client-to-Gateway-to-native positive is claimed complete. No MASTER-ready offer.
Graphify update was attempted in this lease and bounded/cancelled terminal1;
there is no successful current graph-refresh claim.

### Native queue projection boundary

Implemented sealed QueueProjectionCandidate with private member accounts,
original source origin and current admission history. Fresh source validation
and ForwardLifetime precede short RC FIRST admission, account locks, party/
sessions/intent, fresh ban and full cut verification. No network under SQL;
registration spans Redis plus durable acknowledgement. Unknown preserves the
search; definitive source loss or exact account ban resets the affected party.
RELEASE remains independent. Real PG/Redis checks cover positive enqueue,
missing proof, source/readback Unknown, closed head, noninitiator ban, changed
local generation after RPC, held closer before accounts, defensive copies and
closed RELEASE. Four existing projector regressions also pass. Composed147033/
351 worker tests4top11sub pass; actual root adapter cancellation retains its
registration until explicit release, never an attempt deadline alone.

MM026 is reserved in BE011c122 for a durable projection witness and c117 response
receipt. Its queue witness portion is now implemented; response receipt remains
implementation. Native root must not use the old
ApplyPendingRecoveryEffects network-under-row-lock callback for positive
requeue. Implement native acknowledgement of exact whole-party projection
evidence before starting native workers. Full fixed root, protected response
and client-to-Gateway positive remain implementation, not completed features.

### Native recovery acknowledgement

MM026 persists exact projected state and authorized admission stamp, never
backfills old projected_at. Private dispatch lease carries the stamp only after
source proof plus RC authorization; terminal RELEASE carries no fresh stamp.
State hash v2 includes intent revision/action as well as party/all sessions.
Intent revision/action invalidates old projection evidence in SQL. Native ACK
consumes a matching committed whole-party witness or obsolete generation;
it never calls Redis or produces forward authority. RC history uses the existing
nonblocking try-gate; a pinned writable RR view is deliberately unsupported.
RunNativeOnce routes timeout recovery through that ACK, preserving pre-cut
RunOnce separately. Its actual Redis hook oracle sees zero recovery RPC calls.

Real native PG/Redis tests include actual positive projection/ACK replay,
unqualified projected_at rejection, stale generation rejection, closed RELEASE,
RR DOWN refusal before held gate, noncooperating first-writer retention, empty
RC roundtrip, unknown witness retention, and upgrade refusal while ADMITTED.
No shared migration, installation admission or full native root was executed.

### Review309 configuration-loss correction

Exact103 review found nil-binding projector/recovery selection could enter the
legacy forward path despite retained native admission. Actual native PG/Redis
RED reproduced a successful enqueue and a scalar recovery callback. Positive
nil-binding projection now requires CheckPrecutCut; legacy recovery takes FIRST
nil admission in RC through callback/ACK and rejects configured native callers.
Keep genuine pre-cut behavior and closed terminal RELEASE; qualify both native
ADMITTED/CLOSED missing-binding cases and the historical projection regressions.
This is a same-task correction, not completion of the native root or response.

Review309 additionally identified first-begin between the pre-cut RR read and
Redis dispatch. A real two-connection RED proved the installation close gate
could overtake that interval. EnterPrecutQueueProjection now acquires nil
admission in a fresh RC transaction and retains only its advisory gate through
dispatch/classification/ACK; no party/session row locks cross Redis. Native
projection keeps its external explicit-release registration and zero acquired
SQL connections over RPC. The oracle waits on actual PostgreSQL lock state and
requires the first closer to observe the committed projection ACK after release.

### Protected public operations (c117 consumer)

Implement RespondToMatch.operation_id3 and the existing StartSearch signed
operation as additive operation_id5. Current Gateway search was still ordinary
metadata, and the Flutter call had no stable intent key; native handler tests
alone do not constitute a working user journey. Stable IDs are allocated before
first send and retained across ambiguous retries, with fresh delegated tokens.
Implement exact response principal, private candidate/receipt transaction and
both decision branches, then actual client/Gateway/MM positive qualification.
Do not route native calls through ordinary metadata or enable legacy fallback.

The additive Go/Dart wire is now generated from current proto inputs. Exact
responseprincipal accepts authenticated ACCEPT and DECLINE with live epoch,
method/body/operation binding and fresh-JWT semantic retries; reused JWT and
wrong identity/body/method/dependencies deny. Protected transport validates the
opaque subject again after verification, rejecting mutation before the handler.
Native ordinary response listener denies both decisions; explicit pre-cut
ordinary composition remains separate. Search body operation must equal the
authenticated delegation, with actual handler fixtures using genuine signatures.
Actual Linux race checks: search8top25sub, response2top18sub, protected transport
14top14sub, all0skip; full module vet0. Three affected search handler PG tests
also pass. Initial full transport run lacked docker-compose.yml test fixture;
restored exact current tracked file then all package tests passed. This is not
the response SQL receipt/handler, Gateway/client, native root or installed proof.

The response reader now seals a distinct native ResponseCandidate over the
persisted pending match/proposals/complete parties and sessions, private actor
account binding, original source origins/current admission history and exact
operation/request. It does not expose a pending ProvisionCandidate: the public
provision reader still rejects every unaccepted proposal. Live responding-device
epoch may differ from original admitted epochs. Fresh source validation returns
a private exact-candidate proof, Unknown no reset/proof; locked re-read rejects
changed proposal state. Real PG RED then3top racePASS0skip/fullvet0 qualify this
reader and existing provisioning negatives. The following response stage now
implements the atomic receipt/transition and closed terminal-decline path.

For c117 closed terminal DECLINE, read the same immutable receipt/origin lineage
under an explicitly terminal reader and FIRST shared local gate. It accepts
only the exact retained binding in ADMITTED or CLOSED; it does not produce OPEN
readback or permit ACCEPT/provider dispatch. Positive readers and Acquire remain
ADMITTED-only. The sealed response candidate hashes its decision discriminator,
so a terminal observation cannot be reinterpreted as ACCEPT. This follows c117
terminal/history behavior; no new installer or admission authority is created.

The native handler now requires the actual private response principal on both
decisions, validates every captured party before ACCEPT, retains explicit
forward registration through durable response/provider completion, and invokes
the existing validated provider coordinator only for the first all-accepted
transition. The persisted match must be ACTIVE before public resource IDs are
returned. Exact operation retries read the immutable receipt without validation,
new registration or provider dispatch, including CLOSED admission. A changed
body conflicts. DECLINE takes the same authenticated actor and terminal binding,
performs canonical whole-party recovery and commits its receipt atomically.
Unknown source/ban/readback preserves the pending party; a definitive typed
member denial resets only affected whole parties. No raw authctx fallback.

MM026 includes append-only response receipts. Runtime privilege checks allow
SELECT/INSERT and reject effective table/column mutation authority. DOWN holds
the admission gate and table locks before checking retained receipts, including
an uncommitted noncooperating first writer. Final native Linux PG/race tests
exercise activation once, closed retry/decline, receipt-insert rollback,
all-account denial, source/local-cut races, held-closer refusal, expiry receipt,
RR rollback refusal and concurrent receipt retention. Separate actual LOGIN
tests cover column-only mutation grants. Provider/Voice snapshot and lifetime
fixtures are explicit component fixtures, not installed service authority.
Gateway/client/native root integration and final identity-erasure custody remain
unfinished; this source stage is not a complete MASTER delivery offer.

### Gateway and Flutter operation transport

The existing protected Matchmaking profile now carries StartSearch and both
RespondToMatch decisions. Gateway strictly decodes native JSON, requires a
client operation UUID and explicit authenticated human account type, binds the
final URL match identity before hashing, signs a fresh delegated JWT per attempt
and strips forwarded metadata. A configured or partially present protected path
never invokes the ordinary client. The explicitly unconfigured pre-cut
composition retains its old ordinary routes; existing route tests cover it.

Flutter allocates an operation before sending. The match controller retains it
across an ambiguous result, replaces it for a changed decision/context and keeps
its existing stale-response/refresh guards. QueueSearchScreen retains one search
intent across ambiguous retries, replaces it when criteria/actor changes, and
does not promote a late response into a different actor's active search. A known
successful search releases that local pending intent for the next user action.
Client methods accept the action owner's operation and allocate one for a
standalone invocation before GatewayHttpClient can refresh/retry its request.

Native Linux Gateway signed-principal tests and Flutter client/controller/widget
tests cover these boundaries. The initial missing SQLite native asset was a
fixture failure; the canonical prefetch script seeded the hash-pinned Linux
asset, after which causative tests failed for absent operation fields and passed
with the implementation. Full cross-process mTLS/native-root/external admission
qualification is still required; unit transports are not installed authority.

Review309 found the queue screen compared only the current actor tuple, so an
A->B->A or logout->same actor transition could accept an older response and
reuse its unresolved action. The screen now listens to auth context changes,
advances an explicit generation, invalidates its pending operation/local session
and ignores older completions without changing a newer request's busy flag.
Token-only refresh for the same account/profile/type preserves the action.
Actual held-response and ambiguous-retry ABA/logout tests reproduce the old
failure and pass with the implementation. The same final test bytes are also
run against exact c38 predecessor production in an isolated restored fixture;
this paired RED never changes the authored Git tree. The four affected client
test files pass40 tests and scoped analysis without issues on Linux.

### Native authorized terminal and read paths

c53 preserves authorized status/history and exact cancellation, while raw
metadata is insufficient authority at the native receiver. Add a closed
delegated-user method union for GetMatch/GetSearchStatus/GetMatchHistory and
CancelSearch, with a private exact-request subject using the existing live
epoch/replay/JWKS dependencies. Transport request IDs for these existing
read/idempotent-session methods are fresh correlation IDs, not fabricated
StartSearch/Respond business operations. The latter keep their dedicated
private verifiers and client-stable operation contracts.

Native cancellation must terminate the complete captured party, not remove one
member from an indivisible c16 enqueue. Use the existing atomic roster/reset
mechanism under FIRST terminal admission, verify the requesting account/profile
against the immutable private member tuple and retained origin after canonical
locks, and refuse a search that has become pending/active. Terminal retry may
acknowledge an already cancelled exact session; CLOSED admission permits cleanup
without a positive Voice snapshot/ban grant. Keep true pre-cut scalar cancellation
as a separate composition. No read/cleanup route can create source authority.


Implemented terminal/read stage: private exact-request subjects, protected Gateway
GET/DELETE routes, whole-party native cancellation with post-lock validation,
canonical stored cancellation publisher and PrepareNative. GetMatch deadline
expiry now acquires FIRST terminal or never-begun pre-cut admission and retains
it through commit. Causative PG tests proved lost-binding mutation and overtaking
a held closer before the correction; CLOSED expiry and true pre-cut reads remain
working. Final actual Linux domain11PG/NATStop PASS0skip, security29top68sub and
Gateway33top8sub race PASS0skip, roster7PG PASS. Correctly isolated short suite
182top226sub PASS with261top51sub explicit skips; MM/Gateway vet0. Earlier broad
short invocation inherited integration DSNs and reached aggregate180s timeout;
its logs are retained, the interrupted test passed separately, and no full PG
suite PASS is claimed. Full native Run/cmd/image, actual factory/route contract
validation, serving/worker lifecycle and cross-process qualification remain open.
