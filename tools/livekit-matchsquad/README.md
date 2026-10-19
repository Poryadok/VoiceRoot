# Pinned MatchSquad SFU boundary probes

Task: `BE-A3-MATCHSQUAD-SFU-FENCE`. Consumer: the retained MatchFound vertical
and its stale-token/teardown requirement. This directory contains diagnostic
probes, not a patched provider, supported deployment or release capability.

Pinned upstream: `https://github.com/livekit/livekit`, tag `v1.8.4`, exact commit
`154b4d26b769c68a03c096124094b97bf61a996f`. Its module is
`github.com/livekit/livekit-server`, with `github.com/livekit/protocol v1.34.0`.
Upstream source is Apache-2.0; inspect its LICENSE and preserve upstream notices
when preparing any separately authorized patch.

## Source-contract disposition

The accepted task requires a global per-room admission/close arbitration which
survives owner replacement, final current-authority checking through actual RTC
mutation, bounded media teardown and an acknowledgement only after all relevant
participants/room/routes are absent. Its explicit unsupported-provider exit is
applicable here. Astra review is retained in task comment `c4` (2026-10-19).

The pinned provider plus a callback and local owner gate is **unsupported** for
that contract. This finding does not establish that every possible fork is
impossible. No runtime attack was executed for the owner-replacement reasoning.

Exact pinned-source observations:

- `pkg/routing/redisrouter.go:106` reads the owner with HGET; assignment at117 is
  HSET and cleanup at121 is unconditional HDEL. The room allocator replaces an
  unavailable owner without evidence that the old RTC process stopped.
- `pkg/service/redisstore.go:250` implements an expiring SETNX lock. The allocator
  uses it during room creation; it does not span admission and close.
- `pkg/service/roommanager.go:258` starts a session and creates a room; reconnect
  resumes an existing participant, and actual Join occurs at482. `rtc.Room`'s
  lock and closed channel cover that process's local room object only.
- `RoomManager.DeleteRoom` at776 closes the room on that process. The close
  callback at594 logs route/store deletion errors instead of propagating them.
  Late old-owner cleanup can unconditionally remove the replacement route.

Counterexample: owner A pauses after its final allow/lease check and before
local Join. Routing replaces A with B. Voice commits closing; B closes its own
state and acknowledges local absence. A resumes into its still-open local RTC
object. Another check, software timer or renewable lease retains this final
scheduling window. The permanent Voice closed row denies future checks but
cannot retract an allow already in A's instruction stream.

No local mutex, lease renewal, current-route absence or caller cancellation is
implemented here as a substitute. The consumer remains fail-closed. Source work
which allocates a new durable admission/owner protocol must first name its actual
RTC/process/media-sink enforcement boundary and bounded-completion assumptions.
Durable non-expiring intents can retain unknown old-owner obligations and keep
close pending; alone they do not prove finite media cleanup. Task c4 defines the
minimum boundary and the future acceptance oracles. It does not allocate a new
Voice RPC, listener, schema or deployment.

## Expiry probe

In protocol v1.34.0, API token verification validates registered JWT claims, then
returns only ClaimGrants. The provider's ParticipantInit/StartSession transports
that grants object, which has no original signed expiry. The stock refresh path
constructs a new token with a ten-minute lifetime. Retained, verified private
expiry provenance and a real Voice current-authority check are required; an
attribute or metadata field supplied by a caller cannot replace them.

Copy `probes/missing_expiry_test.go` into `pkg/service/voice_missing_expiry_test.go`
in a disposable checkout of the exact pinned commit. Run the following inside a
native Linux build container with no published ports:

Copy `probes/run.sh` to the checkout root and run `sh run.sh`. It selects the
actual build's production Go files and only this probe. Stock `service_test` has
an unconditional TestMain which expects a Docker socket and creates containers
with host ports; that unrelated integration harness is not used or changed.
This command is explicitly a component probe, not the upstream service suite.

The independent `control/go.mod` module keeps these upstream-only probes out of
ordinary module checks. Its deterministic model checks run with `go test -race
./...` and `go vet ./...` from `control` inside a Linux container.

The MatchSquad missing-provenance case is expected to fail on stock upstream.
Its spoofed attribute/metadata subcase must also fail closed in a future accepted
implementation. The ordinary refresh control must retain stock behavior. Bearers
and fixture keys must not be printed. These component probes do not exercise
actual media, owner replacement, real Voice authority persistence or the full
private provider fixture, and cannot qualify a provider by themselves.

## Continuation

Task c6/c7 selects the next architecture: a separate one-room MatchSquad worker
and an independent trusted supervisor with an exclusive mandatory forwarding
attachment. Replacement is permitted only after the previous sandbox has been
physically fenced. The conditional fault profile assumes a responsive trusted
host/kernel/runtime/controller; a whole-host fault requires independent VM/power
enforcement. No timer or worker-owned lease provides that boundary.

`control` now defines the reference-only protected authority/supervisor close
boundary, complete owner/birth census and exact runtime/host-boot bindings. It
archives each actual supervisor receipt before aggregate confirmation, rejects
missing scope/evidence and retains lost-reply ambiguity. Its tests use explicit
model-only readers. There is no installed native authority journal or qualified
runtime supervisor implementation yet. Shared Voice/proto allocation and the
first actual private forwarding/process fence proof remain explicit next steps.

## Private enforcement capability probe

`fixture/fence_probe.py OUTPUT.json` runs a disposable process/network boundary
probe using a trusted host-side Docker CLI supervisor. Every peer runs in Linux;
the orchestrator opens no host listener or published port. It uses immutable
runtime UIDs returned by its own creates, two separate internal networks and a
fixed image digest. It never discovers/deletes resources by labels. The worker
and forwarding peers have no Docker socket, privileged mode or added Linux
capabilities. The trusted, short-lived host-PID observer reads only exact known
owned PIDs and birth times; it neither enumerates nor controls other processes.

The worker's parent is paused with SIGSTOP while a child in another process group
continues a real bounded UDP stream. The independent forwarding sandbox closes,
its attachments are removed, then the worker sandbox terminates. The probe
checks actual Docker running/PID/attachment state, unchanged host boot and absent
old parent/child PID births, direct-path denial, positive TCP forwarding, and no
new client delivery after the final owned fence marker. Container/network cleanup
is guarded by recorded exact UIDs. An incomplete observation is a failure.
Returned network and container UIDs are persisted before fallible inspection.
The mocked custody regressions run with `python -m unittest discover -s fixture
-p test_fence_probe.py -v` from this directory; they create no Docker resources.

This is raw evidence for a **fixture UDP/TCP process/network boundary**, not a
LiveKit media, ICE/TURN, owner-routing or participant-epoch qualification. It
does not emit a trusted FenceReceipt, implement native authority/supervisor
journals or establish crash/restart recovery. Linux cgroup v1 was observed on the
current host; no cgroup-v2 kill/events support is assumed. Bounds are recorded
observations under a responsive trusted host/control profile, not a promised
worst-case product SLA. Whole-host pause/partition and already delivered client
buffers remain outside this component observation.

The first successful run recorded zero post-fence frames, an old descendant
continuing while its parent was stopped, and no cleanup failures. The initial
run failed before fencing because a direct UDP send correctly had no route;
the probe now treats that send error as denied and retains failure logs. Both
outcomes belong in the task evidence; neither upgrades the full provider gate.

1. Preserve exact source/probe outcomes in the owning task.
2. Keep the existing MatchFound consumer fail-closed at the provider boundary.
3. Obtain a separate source-contract allocation naming a real stale-owner sink
   fence, or select a provider with that demonstrated contract.
4. Only then implement the allocated provider/Voice interfaces and run task c4's
   old-owner pause/partition, partial mutation, lost reply, failed delete,
   reconnect/refresh and ordinary-room regression matrix. The requested finite
   media revocation and full product acceptance remain open.
