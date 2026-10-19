# ExecPlan: owner-consumed release policy and Notification guard

## Purpose

An alpha Notification process consumes the same immutable, source-bound policy
for device admission, final provider dispatch and its private effective snapshot.
Existing mobile tokens and retries cannot bypass the dispatch guard. Free in-app
delivery, web push, settings, unregister and ordinary message behavior remain.

## Context

PL-A4-02 c4-c9 records Astra277's accepted authority/qualification contract
(artifact SHA256 ca9fd7bccffd40b549deffadbba2a0d3c156e42c107359deacc0d62d8e38eebb).
PLAN A4/G1, DEPLOYMENT, ARCHITECTURE_REQUIREMENTS and Notification feature/service
docs govern behavior. Base: 68927e5912ebc218050fcc9daf4b76cf4a9e3c86.
All event and direct notification paths converge on PushDispatcher.Send. Current
default provider dispatch falls through unknown metadata to FCM; RegisterDevice
prefers enum over conflicting legacy platform. Alpha must fail closed instead.

## Scope

Shared Go schema/immutable parser and build/digest binding; a real Notification
bootstrap consumer, RegisterDevice and final dispatcher; private process-local
snapshot from that same object. No File/Messaging/BE080/proto writes. Other owner
guards, Java/Dart consumers, UI/REST/WS qualification and rollout remain open.
No new WS message.send, provider activation or deployment is introduced.

## Milestones

- [x] Freeze policy schema and meaningful source/digest/malformed RED cases.
- [x] Consume parsed policy in Notification bootstrap before external work.
- [x] Prove valid-principal mobile admission denial and final-provider suppression
  for old tokens/retries; preserve real web dispatch and unregister controls.
- [x] Export effective snapshot and reject dev/noop misrepresentation.
- [ ] Run Linux component checks, review, portable source and 121 delivery.

## Detailed Steps

1. Add tests and minimal compiling stubs, run intended RED in owned Linux.
2. Implement strict bounded JSON parsing, exact independent expected digest/source,
   explicit development mode and immutable copied projection. Six capability IDs
   and finite rich intent table are shared schema; no alpha enabling fallback.
3. Wire one policy instance into actual Notification RPC and dispatcher objects;
   authenticate before mobile admission, validate platform/provider conflicts.
   Alpha suppression returns processed/no-provider, avoiding infinite NAK retries.
4. Bootstrap rejects missing/conflicting policy and debug/offline recording in
   alpha before DB/network/listeners. Snapshot is an exclusive private file, not
   an unauthenticated HTTP endpoint; its source is compiled metadata, not env SHA.
5. Test actual bootstrap consumer plus real owner entrypoints with positive
   provider controls, no effects and old tokens; execute real PG where required.

## Validation

Owned Linux Go1.26 standard-library parser tests, Notification focused/race/vet
and affected component checks. PostgreSQL integration uses an owned private
network and no host ports. A valid snapshot is not image/orchestrator attestation;
full 18 groups require exact observed replica/build/config/readback controls.

## Progress

- [x] Atomic parent ownership retained; fresh WT43 lease3ec8b8bdd0bda630e2528368e9891389.
- [x] Current source confirmed; CodeGraph direct call bounded without result.
- [x] Shared parser four meaningful RED cases then Linux race PASS.
- [x] Four owner regressions RED against original implementation with compiling
  new fields, then Linux race PASS. Initial concurrent compile attempt was
  inconclusive (missing helper); it is not the RED evidence.
- [x] Four bootstrap/composition/private-snapshot tests RED then GREEN.
- [x] Real isolated PG16 no-effects/web/unregister and old-token retry PASS.
  A repeated-run harness collision was corrected with a unique owned schema;
  no product assertion was weakened.
- [x] Notification whole short suite and vet PASS. First short harness lacked
  Compose/config files; exact copied inputs corrected the harness failure.
- [x] Pinned golangci-lint2.6.1 built Go1.26.5: Notification and new shared
  package zero issues. A QF1001 style diagnostic was corrected without changing
  validation behavior; shared race suite remained green.
- [ ] Image/build binding proof, source review and 121 transfer.

## Decisions

- Missing release config blocks bootstrap. Local development is explicit and
  cannot emit an alpha-qualified snapshot; no production default profile exists.
- Mobile suppression applies at final dispatcher even for preexisting tokens,
  not only token registration. Unknown/conflicting metadata cannot fall into FCM.
- An alpha policy does not disable web push or ordinary in-app notification.

## Risks And Follow-Ups

Old pods/config cannot qualify. Build jobs must pass independently resolved source
metadata and rollout must mount a matching policy/snapshot target before alpha.
Other owners remain required; partial Notification source is never full alpha
qualification. BE080 preview pipeline and all acceptance/resource gates remain.
