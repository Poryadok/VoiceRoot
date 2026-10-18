# ExecPlan: fixed NATS participant and Auth subject authority v5

## Purpose

Permit the reviewed participant and Auth observation bootstrap sources only after
the existing voice/user streams receive their exact additive subject expansion.
Retain old messages, consumer state and historical migration authority.

## Context

Base: develop `8984552dd177cec85af7cd328579886a50acb619`. TT
`CI-NATS-V5-AUTHORITY`, accepted Astra decision SWARM-DEVELOP-20261018 c15.
Canon: voice-service participant events, auth-service observation events,
DATA_STORES, TESTING and the preservation README. Existing root pins describe
v4; changing their hashes alone would omit both required stream updates.

## Scope

Fixed whole four-template v5 source tuple, v5 planner/executor/actor/recovery,
census proof v4 and active receipt readback. Four existing v4 consumers and auth
enrollment remain unchanged. No installed helper, stand, credential, historical
contract, cold-cut or journal mutation. One author owns tests and implementation;
an independent preservation/security reviewer must approve before intake121.

## Milestones

- [ ] Canonical root RED with production pins and historical contract.
- [ ] Fixed v5 tuple and exact 10/10, 12/10, 10/14, 12/14 planner cases.
- [ ] Version-aware actor, execution, recovery and active receipts.
- [ ] Existing regressions plus negative compatibility/source controls.
- [ ] Isolated pinned native NATS preservation and interruption proof.
- [ ] Independent review, portable bundle/restore and accepted handoff121.

## Detailed Steps

1. Add canonical tests using actual template bytes, existing immutable contract
   and root/compiler/custody paths. Observe existing pin failure.
2. Freeze named v4 schema/ID/declarations and complete old source tuple. Add
   separately fixed four-file v5 tuple; select whole tuple, never mix per-file acceptance.
3. Parse unique exact voice/user declarations across all relevant templates;
   permit only complete old or target subject sets, preserve other config fields.
4. Add only exact two UPDATE APIs to v5 throughout execution and recovery.
   Old plans/receipts must reject those operations and v5 proof/source shapes.
5. Run focused Python regressions. Extend isolated Linux fixture with existing
   authenticated identity, pending voice/user consumers and byte-preserving
   crash recovery after each update; use no Windows executable/listener.
6. Freeze source, review, preserve portable proof and transfer only after121 ACK.

## Validation

Canonical pins -> root -> compile -> compiler -> custody -> serialized active
readback -> second no-op. Test both independent mixed stream combinations,
malformed/duplicate/overlapping declarations, mutation/TOCTOU, historical v3/v4,
consumer/state drift and version smuggling. Native proof compares complete
semantic census plus old /msgs/index.db and consumer o.dat records.

## Progress

- [x] Atomic task claim and own WT20 lease; accepted decision read.
- [ ] RED/GREEN, implementation and proof in progress.

## Decisions

The v5 transition is new migration authority, not a hash refresh. Preserve
literal historical receipt readers and unchanged auth-v4 enrollment. Root plan
digest binds its selected schema, migration identity and complete source tuple.
The original decision incorrectly omitted notification's already accepted
participant subjects. CI-NATS-V5-AUTHORITY c4 supersedes that premise: preserve
its current exact bytes as the fourth v5 source/script and require four guarded
CAS/readbacks. The observed three-hash compiler refusal remains a negative test.

## Risks And Follow-Ups

High preservation/security risk. Native isolated proof is separate from hosted
CI, installed upgrade, DEV/STAGING and appointed final acceptance. Those remain
with their actual owners and authorization gates.
