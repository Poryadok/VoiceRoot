# ExecPlan: fixed A3 roster NATS authority

## Purpose

Qualify the exact A3 roster subject and Matchmaking pull consumer without losing
retained messages, changing prior consumers, or silently repinning v5 authority.
The consumers are BE-012 producer owner147, BE-011 owner145 and integrator121.

## Context

TT BE012-A3-NATS-AUTHORITY owns this slice. Base develop is
2445e81de07681cfd5f1491e4c793aaac747f571. The reviewed v5 authority is
9368d0efa3e5ba8bce88faaefcd35dca4b1c0c06. The exact four canonical template
blobs come from owner147 checkpoint 2b920a0fd37b502384e02f091b928e3969ebbd8d;
that commit also contains unrelated producer work and must not be merged whole.
Owner147 explicitly permits fixed qualification fixtures and a narrow template /
ACL composition. Product sources are PLAN A3, Voice roster and Matchmaking
contracts, DATA_STORES, TESTING, and the preservation README.

## Scope

Add separately versioned whole-four-source authority, exact voice 13/user14
sets and matchmaking_voice_roster_v1 pull/all/explicit declaration. Preserve
literal v3/v4/v5 source, plan, API, receipt and Authv4 enrollment semantics.
Qualify bootstrap, publisher and MM permits/denials using actual credentials in
an owned Linux fixture. No installed signer, credential rotation, shared stand,
deployment, provider/native adapter activation or acceptance authority.

## Milestones

- [x] Atomically claim task and acquire own WT33 lease from develop.
- [x] Freeze exact source/script identity and canonical root RED.
- [x] Versioned compiler/actor/root/receipt and four ConfigMap custody GREEN.
- [ ] Exact all-policy census/recovery and backlog delivery/ACK proof.
- [ ] Canonical preservation suite and actual isolated pinned native fixture.
- [ ] Independent security/preservation review, portable restore and real121 handoff.

## Detailed Steps

1. Materialize the four immutable target blobs as qualification fixtures. Keep
   current production source and historical test fixtures authoritative by version.
2. Add exact v6 declarations and source/script descriptors; test mixed/partial
   tuples, extra subjects, wildcard overlap, missing notification and wrong API.
3. Extend root auth normalization with separately versioned live proof while
   preserving existing enrolled bytes. Missing installed grants remain a gate.
4. Extend execution, issued-intent recovery, census proof, custody and active
   readback only for v6. Prove interruption after UPDATE and CREATE, then no-op.
5. Execute pinned Linux NATS/Box with populated matching backlog, existing
   pending consumers and native record/state byte comparison. Verify exact MM
   INFO/NEXT/ACK rights and wrong subject/durable/publish denials.
6. Freeze exact source/evidence, request independent review and offer121 delivery.

## Validation

Canonical four files -> root -> compile_plan -> compatible_bootstrap -> four
guarded CAS/readbacks -> serialized active receipt -> repeat no-op. Existing v5
proof cannot stand in for new deliver-all backlog qualification. Targeted Python
tests precede implementation; canonical suite follows coherent integration.
Native programs run only inside owned Linux Docker without host ports.

## Progress

Source investigation found v5 verify_census requires new consumers to have no
pending and delivered.stream_seq equal to old last_seq. Astra187 resolved the
separate all-policy authority in TT c2/c3. The exact CLOSED predicate is delivered
consumer0/stream(S-1), AckFloor0/0, independently counted pending M and empty
native pending/redelivery maps. An actual pinned NATS first_seq3/last_seq5 edge
confirmed delivered0/2, AckFloor0/0 and two matching retained messages across
stop/restart. That edge is not full migration acceptance.

The versioned implementation now includes immutable Authv4 projection from a
five-row Authv5 proof, original-archive count descriptor, issued/closed/native
Created evidence, partial recovery, four-CM custody and serialized v6 receipt.
Linux scoped tests passed 141 checks, followed by four root/auth/custody tests
including the complete source-to-active roundtrip. Canonical Linux regression
then passed 947 preservation tests, 44 baseline tests and both shell assertions
on immutable source archive 324f64d7f032284850e34805b17619e1ee91c363cde3735f900c829a9bd4ca17.
Reviewer258 recorded bounded SOURCE CLEAR in the review function c1 and parent
c8 for runtime archive c29f1167 plus inspected fixture-only v7 delta. Final
commit applicability and native/runtime acceptance remain separate gates.

Native empty10 and empty12 cases passed actual lost-UPDATE/CREATE response
recovery, the before-CREATE-intent interruption, and fresh-cut no-op. Each
preserved all 43 captured old consumers and three Social records. A later
empty13 setup hit a Docker client wait timeout before compile/mutation; this
did not invalidate completed cases. Only unrun scopes continue on a separate
owned fixture. Stronger delivery metadata/unique-event assertions and original
roster consumer unacked/acked cases run separately. Full native matrix remains
open. The first native attempts identified fixture
setup errors (Go API signature, missing reviewed predecessor preparation/input),
all before retained-store mutation. Their actual logs remain private evidence.

Actual pinned empty filtered INFO has total0/offset0/limit0, whereas a positive
subject match has total1/offset0/limit100000. Require exactly that conditional
wire shape. The raw zero response and original native cut are retained; no broad
normalization or inferred absence is accepted. Graphify AST update was attempted
but remained unresponsive; its owned handle was interrupted and all its process
IDs were confirmed absent. It is not a runtime or source acceptance substitute.

## Decisions

Use a new authority rather than updating v5 pins. Keep all four immutable
template identities together. Do not merge owner147's wider source checkpoint.
Authv4 enrolled proof stays immutable; fifth-consumer evidence needs explicit
successor semantics under the new Authv5 schema, with exact four-row projection
equality. The canonical producer/issuer files are not installed or changed by
this slice; exact owner147 bytes are qualification fixtures. Parent BE-195
review reservation was withdrawn on discovery
of an earlier atomic219 claim and is unrelated to this implementation.

## Risks And Follow-Ups

Security and data preservation risk requires independent review and real native
proof. Existing v5 native evidence can be reused only for unchanged modules;
the new pull/all backlog and ACL edges need new execution. Missing installed
rights and later rollout remain receiving executor gates under TT resources.
Owner147 retains actual product Matchmaking/PostgreSQL C4/C5 inbox commit and
lost-ACK evidence; broker-only delivery does not satisfy that dependency. The
main roster relay must remain dormant until qualified v6 authority, installed
rights and producer/consumer activation gates are accepted (222 c28/c29).
