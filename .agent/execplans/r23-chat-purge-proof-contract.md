# ExecPlan: R23 Chat purge prerequisite proof contract

## Purpose

Close the documentation and RED-evidence gap that currently prevents a safe
implementation of terminal R23 Chat purge. The repository must name how Chat
receives and validates durable proof that Messaging has completed its purge and
File has accepted release of the exact Chat-owned reference producer manifest.
Until that seam is accepted, Chat `PurgeSpace` must remain fail-closed and must
not delete chats or manufacture a completion receipt.

## Context

- `docs/PLAN.md`, “Accepted R23 deletion/retirement contract”.
- `docs/features/spaces.md`, “P3 convergent lifecycle contract”.
- `docs/microservices/space-service.md`, “P3 Space lifecycle coordinator”.
- `docs/microservices/chat-service.md`, “P3 Space lifecycle participant”.
- `docs/microservices/messaging-service.md`, “P3 Space lifecycle participant”.
- `docs/microservices/file-service.md`, “P3 reference authority, lifecycle fence and GC”.
- `docs/DATA_MODEL.md`, `docs/DATA_STORES.md`, and
  `docs/ARCHITECTURE_REQUIREMENTS.md` define evidence ownership, deterministic
  bytes and fail-closed propagation.
- `protos/voice/chat/v1/chat.proto` currently gives `PurgeSpaceRequest` only one
  field, the generic `voice.common.v1.SpacePurgeRequest purge`.
- Messaging can issue a participant-3 `SpacePurgeReceipt`. File can issue a
  `ReleaseSpaceDeletionProducerReferencesReceipt`, including producer, purge
  generation, source schedule generation, expected-reference hash and request
  hash. No accepted wire binds either receipt to Chat's purge request.

## Scope

### In

1. Document the missing bound-receipt seam and the decisions required before
   terminal Chat deletion can be implemented.
2. Name the only valid proof issuers: Messaging for its completion receipt and
   File for acceptance of the exact `CHAT` producer release.
3. Freeze fail-closed executable evidence for the current one-field wire and
   generated unimplemented handler: no current request can authorize deletion or
   a Chat completion receipt.
4. Freeze future acceptance cases in a RED manifest: exact proof binding,
   restart/resume replay, changed/missing proof rejection and ordering.

### Out

- Choosing a new protobuf shape without an accepted documentation decision.
- Chat participant storage/fence/manifest implementation.
- Messaging/File production, Space orchestration, Gateway/Flutter, Realtime or
  Compose tests.

## Required decision

The canonical contract must select and document all of the following together:

- **Transport:** proof bytes are either fields covered by the deterministic
  `voice.chat.v1.PurgeSpaceRequest` hash or fetched through authenticated,
  request-bound receipt lookup RPCs. An unsigned header or coordinator assertion
  is not sufficient.
- **Issuance authority:** Messaging alone issues participant-3 completion after
  its saved Chat work set is deleted and its `MESSAGING` File release is durable;
  File alone issues acceptance for the exact `CHAT` producer release.
- **Caller for Chat-owned release:** choose whether Chat invokes File and stores
  the returned receipt before local deletion, or Space obtains and binds the File
  receipt before invoking Chat. The retry owner must be explicit.
- **Binding:** protocol, Space ID, deletion operation, purge generation, source
  schedule generation, root/chat manifest hashes, participant/producer IDs,
  reference aggregate hash and exact deterministic request/receipt bytes must be
  cross-checked. Hash equality alone is insufficient.
- **Resume acceptance:** after timeout, response loss or restart, exact accepted
  proof bytes replay the same result. Missing, reordered or changed proof cannot
  advance deletion. Persisted partial progress may resume only from its exact
  stored operation and must never recapture the work set.

## Milestones

- [x] Read the accepted R23 docs, current wire and existing participant patterns.
- [x] Confirmed the current wire has no canonical Messaging/File proof binding.
- [ ] Add the docs/TODO gap and CI-green fail-closed contract test.
- [ ] Obtain independent review of wording, evidence and non-invention boundary.
- [ ] Run scoped checks, push, open the PR, wait for exact-head CI and merge only
      after required review.

## Validation

- [ ] `go test ./internal/grpcsvc -run 'TestR23Chat(PurgeDependencyProofContractGap|DeferredREDManifest)$' -count=1`
- [ ] `go test ./... -short -count=1`
- [ ] `go vet ./...`
- [ ] `git diff --check`

## Decisions

- This PR records the missing decision; it does not select a new wire shape.
- Current `Unimplemented` purge is not treated as successful implementation, but
  it is safe evidence that the insufficient request cannot delete data today.
- Future production GREEN requires a separate docs/proto/implementation cycle
  after the bound proof seam is accepted.

## Risks And Follow-Ups

- A broad Chat participant implementation before this decision could delete the
  authoritative chat rows before Messaging or File durably completes.
- The future change must add success, response-loss, restart, changed-proof and
  irreversible-generation tests before enabling destructive behavior.
