# ExecPlan: protected non-E2E catalog asset import

## Purpose

Make a selected GIF catalog asset usable as a READY File in an independently
authorized destination, without turning client URLs, cached file IDs or lifecycle
references into authorization. This is the File/Messaging child
`BE085-FILE-MESSAGING-ASSET-IMPORT`, related to `BE-085`.

## Context

Product canon: PLAN A6, text-chat media, file-service Stickers and GIF assets,
messaging-service Stickers and GIF, Chat sticker catalog, and Phase-0 principals
in ARCHITECTURE_REQUIREMENTS. Astra101 resolved the non-E2E security constraints;
the concrete API/lifecycle design is delegated implementation work.

Base is `4773721665dedd3780ac9d10d0a18a7b11a8b5e3`; Anton-95 owns Treehouse75,
branch `codex/anton95-catalog-asset-import`. Anton-93 owns Chat APIs, generated
Chat clients and Gateway catalog endpoints; Anton-100 owns Flutter. PR898 owns
GIF processing, which is a dependency rather than proof of protected import.

## Scope

- In: immutable source identity, protected File import and source-scoped lookup,
  safe remote byte retrieval, typed Messaging GIF admission, meaningful negative
  tests and exact-source evidence. The first activated consumer is GIF search.
- Out: live provider activation, arbitrary URL upload, ordinary attachment ACL
  changes, encrypted-DM catalog identity, new sticker ingestion and pack editing.
- Existing sticker new-send validation still requires the current installed pack
  and exact sticker/file tuple. Uninstall does not remove committed history.
- Encrypted-DM opaque references versus server-visible catalog identity remains
  an unresolved product/privacy contract; no plaintext source branch is enabled.

## Milestones

- [x] Establish truthful task custody, clean lease/base and write boundaries.
- [ ] Freeze concrete interface with Chat/Gateway consumer and review it.
- [x] RED/GREEN safe fetch: every DNS answer/connection/redirect, bounded bytes
  and lifetime; no proxy fallback or unsafe target.
- [ ] Durable request/source binding and import into the existing File processing
  path; prove exact retry, changed request conflict and crash recovery.
- [ ] Protected source-scoped lookup and typed Messaging admission; preserve
  ordinary origin-chat checks and current destination membership/policy.
- [ ] Assemble with actual Chat producer; positive two independently authorized
  destinations and rejection matrix, focused checks and independent review.
- [ ] Freeze exact head/artifacts; hand off delivery and account for resources.

## Detailed Steps

1. Agree the transport and immutable tuple described below with Anton-93.
2. Add focused tests before each production behavior; use repository protected
   principal runtimes and existing File processor rather than duplicate them.
3. Keep all remote retrieval behind the protected Chat-to-File operation. A
   public metadata call and a FileReferenceKey cannot substitute for source auth.
4. Run focused package tests, meaningful database/transport tests, buf and
   generated-client checks when those paths change; graphify update after code.
5. Record actual RED/GREEN, skips/failures and immutable artifacts in TT. Do not
   claim full delivery until actual source and destination authority compose.

## Interface agreement under review

The gateway-facing selection operation uses an authenticated delegated active
profile and a destination ChatRef. Its response exposes only source identity,
import state and File identity. It never exposes a source URL as upload authority.
Chat owns a separate protected catalog listener with Phase-0 request hash,
RPC/audience binding, replay verification and dedicated issuer trust. Ordinary
forwarded profile headers do not authenticate this operation.

Chat alone resolves `(provider, provider_id)` through its configured adapter or
authoritative cache. It persists immutable `(source_id UUID, provider,
provider_id, generation, trusted_source_url_hash)`; a rotated URL requires a new
generation. No fake catalog data is seeded as a product result. A source ID is
neither a destination grant nor a bearer capability.

File `ImportCatalogAsset` is Chat-only, pure service principal, and accepts
`protocol_version=1, operation_id, source_id, provider, provider_id,
trusted_source_url, actor_profile_id, destination_chat`. The source URL is
protected internal data. File independently checks target membership and rejects
an E2E-enabled target before fetching. File's durable import key is
`(actor_profile_id, source_id)`; Chat's resulting binding has the same scope.
The immutable operation fingerprint includes every field, including destination.
A retained identical operation returns the same identity/state; changing any
field conflicts. A retry receipt never implies READY. The concrete worker
lease/recovery and terminal-failure rules must be frozen before storage code.

Messaging does not resolve or import. Chat `ValidateGifAsset` is Messaging-only,
read-only, and verifies the exact current `(actor_profile_id, source_id,
provider, provider_id, file_id)` binding. File `ValidateCatalogAsset` is also
Messaging-only and echoes that exact source/profile tuple with authoritative
non-E2E status, persisted intent, scan state and representation identity. It
returns no source URL, R2 key or unrelated file metadata. Public GetBulkMetadata
retains its existing authorization.

Only the typed catalog path may replace the original `file.chat_id == target`
predicate after both protected source checks succeed. Messaging still performs
current destination membership, permissions, communication policy and E2E checks.
Missing source authority fails closed. It acquires durable message references
only after source and destination authorization; readback uses the committed
message reference plus history entitlement. Ordinary uploads retain their
existing origin-chat restriction.

## Validation

Required positives: trusted selected source imported, actual processor reaches
READY, authorized send, exact retry identity, and the same profile/source used
in two independently authorized chats. Required negatives: forged/mismatched
source or provider tuple, uninstalled sticker, wrong destination, pending,
failed, expired, wrong intent, public forged S2S, unsafe DNS/redirect targets,
and altered retained retry. E2E exclusion is checked before source disclosure.

## Progress and decisions

- Read-only CodeGraph located ordinary attachment origin-chat equality and the
  existing File processing path. No authorization bypass was introduced.
- Graphify query in new lease had no graph file; read-only main graph query
  completed successfully and surfaced only the metadata catalog context.
- Anton-93 reviewed and refined the interface draft; protected Chat producer
  implementation is separately owned and must be assembled before delivery proof.
- Fetch RED: missing Fetcher API; transport suite then GREEN. A separate test
  exposed provider query-secret leakage through net/http errors (RED); errors
  now return only the redacted source category and context cause (GREEN).
- Native Go `go test -race ./internal/catalogasset -count=1` passed (8.306 s);
  `go vet ./internal/catalogasset` and `git diff --check` passed. This verifies
  the byte transport component only, not a protected import or message delivery.
- Graphify update is running under the author's command handle; no freshness
  claim until its terminal result is recorded.
- Independent Anton-97 found query-secret disclosure through Go's default
  redirect Referer. Real TLS tests reproduced it for both the original and a
  different permitted host; every redirect now removes Referer.

## Risks and follow-ups

PR898 source and generated File proto ownership must be reconciled before File
API edits. Full delivery depends on the protected Chat producer, not its preview
search API. Encrypted-DM privacy semantics remain explicitly open.
