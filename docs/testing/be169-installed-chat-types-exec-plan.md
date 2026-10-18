# BE-169: installed Bot Chat types

## Purpose

Return Chat-owned group/channel types in `ListInstalledBots.allowed_chats`
instead of labeling every whitelist entry as a channel.

## Context

- [Bot service](../microservices/bot-service.md), install/uninstall and
  `ListInstalledBots`: whitelist entries are group/channel Chat IDs.
- [Bots](../features/bots.md): installations select text chats.
- [Data model](../DATA_MODEL.md): Chat owns chat identity and type; other
  services use `ChatRef`, without reading Chat's database.
- [Testing](../TESTING.md): focused RED/GREEN and affected Go module checks.
- Starting source: `ab6aa8e224f31f2ec55e93b923a43d9b7867ec76` (`develop`).
- Existing code stored only whitelist IDs and fabricated `CHANNEL` on reads.

## Scope and decisions

Hydrate each distinct whitelist Chat ID through existing `Chat.GetChat`,
forwarding the original caller context without creating an internal identity.
Accept a type only when the returned Chat ID and Space exactly match and the
type is group/channel. Preserve the existing whitelist ID with absent optional
type on missing client, denied/unavailable owner lookup, incomplete response,
foreign identity/Space or unsupported type. This routine degradation choice
avoids inventing types or exposing owner error details.

Cache only within one list request, so a later request observes changes.
Close installation/whitelist PostgreSQL rowsets before owner RPCs. Existing
list authorization and whitelist mutation are outside this slice; no new
permission, principal, endpoint, schema or deployment is introduced.

## Milestones and detailed steps

- [x] Add a real PostgreSQL/public-handler regression covering mixed Chat
  types, absent client, denied/unavailable lookup, foreign identity/Space,
  unsupported DM, original metadata forwarding and fresh subsequent reads.
- [x] Observe meaningful RED against the original handler: group expected,
  channel returned; missing-client references also had a fabricated type.
- [x] Implement scoped Chat hydration and rowset cleanup; assert shared Chat
  IDs across installations are looked up once per request.
- [x] Observe GREEN for the actual PostgreSQL test, without short/skip.
- [x] Run Bot short suite and vet in Linux with a native module/build cache.
- [x] Complete static lint.
- [ ] Complete source review and immutable source handoff.
- [ ] Integrate into current develop with applicable exact-version CI.

## Validation and execution evidence

The isolated `golang:1.26-bookworm` runner extracts an immutable base archive
onto its Linux filesystem. A read-only local Go ZIP proxy supplies dependencies
to its own writable Linux cache. Testcontainers uses the existing Docker socket
and dynamic PostgreSQL ports; no Windows test executable is run.

- `go test -json -count=1 -run TestListInstalledBots_ActualChatTypes ./internal/grpcsvc`:
  RED before production changes, then GREEN with PostgreSQL (no skipped test).
- `go test -short -json -count=1 ./...`: PASS, 147 passing test records;
  183 integration/opt-in records explicitly skipped by the short suite.
- `go vet ./...`: PASS in Linux.
- `golangci-lint run ./...`: PASS, zero issues; host static analyzer is built with
  Go 1.26.2 and does not execute test binaries.
- `git diff --check`: PASS.

Portable execution logs and source restoration instructions are recorded in
TT BE-169 before resource release. Source review, CI and deployment/acceptance
are separate gates; the local checks do not imply those outcomes.

## Risks and follow-ups

Hydration uses one existing owner RPC per distinct whitelist Chat, with the
caller's context/deadline and permission checks. It does not add a new batch
API or cache owner data across requests. When Chat cannot authorize hydration,
consumers must treat the optional missing type as unknown. This change does
not fix the separately existing `GetChatWhitelist` channel assumption.

BE-046 authoritative report-threshold work has a separate accepted specialist
contract and no source changes in this branch.
