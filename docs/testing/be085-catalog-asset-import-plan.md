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
  Source inspection found the current Messaging implementation checks UUID
  shape and File metadata only; installed-pack/exact-tuple authority remains
  OPEN. The GIF slice does not claim that negative matrix is implemented.
- Encrypted-DM opaque references versus server-visible catalog identity remains
  an unresolved product/privacy contract; no plaintext source branch is enabled.

## Milestones

- [x] Establish truthful task custody, clean lease/base and write boundaries.
- [x] Freeze concrete interface with Chat/Gateway consumer and review it.
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
- Anton-97 reviewed the exact pushed correction `985f774fea80c936cc449862c1b6e156c2079249`
  and returned narrow byte-fetcher CLEAR. Full import/admission is not covered.
- The [concrete protocol](be085-catalog-asset-contract.md) includes the mutually
  agreed Chat/File fields and protected caller matrix. New File schema and
  generated clients are additive; existing File proto/processing remains owned
  by PR898. Astra101 supplied bounded abandoned-import cleanup and receipt
  minimization: actual internal pending File, no raw URL persistence, P30D
  completed details, purpose-HMAC compact terminal fence. Storage activation
  still requires its actual executor fencing and account/profile integration.
- The File source admission predicate now checks exact source/account/profile/
  provider/File identity, READY, non-E2E, logical expiry, scan, persisted GIF
  intent, video category, revision and actual source/delivered digests/limits.
  Its focused negative matrix is GREEN after missing-API RED; combined native
  race checks with the principal suite passed (3.012 s and 2.196 s). It does not
  substitute for an actual database lookup or current destination authority.
- Normal Git merges assembled exact Chat protocol `32260ce` and reviewed GIF
  processor `708e54d2264f8f05aea3b529a26fd9b653dec154` into the author's branch;
  source ownership remains with their authors. This is dependency integration,
  not an assertion that PR898 is merged or that catalog import is implemented.
- The Chat-only GetCatalogAssetStatus operation/asset selectors close response
  loss plus CDN rotation without persisting URLs. Its route failed ordinary
  listener isolation before the allowlist update (RED), then the combined native
  principal/fetch race suites passed (2.786 s and 3.156 s).
- Messaging's typed consumer now classifies by File ID, checks canonical Chat
  source/current destination, repeats File attestation and preserves ordinary
  origin-chat metadata validation. E2E does not call source authority. Catalog
  forwarding remains unavailable in this initial consumer; ordinary forwarding
  is unchanged. Runtime configuration and actual File handlers are not wired.
- Messaging component RED included missing coordinator, an actual parsed-path
  metadata dependency failure, forged ordinary attribution acceptance and a
  typed-nil dependency panic. Corrected focused race tests passed (4.055 s);
  Messaging and File `go test -short ./...` and scoped vet passed. Database tests
  are skipped by short suites; these are not full import/send delivery proof.
- File's User actor client now verifies exact pair/version/enum/cardinality
  snapshots, signs with the pure File principal and preserves UNKNOWN. Its tests
  verify actual JWT signatures, full request binding and fresh replay IDs.
- Migration 10 and zero-size metadata exclusion passed real isolated PostgreSQL
  race tests (13.025 s): an old publisher actually blocked on the closing actor
  row and was rejected after closure committed; restored generations do not
  revive old bindings, ordinary zero-size uploads remain invalid and DOWN cannot
  erase permanent fences. Empty DOWN remains reversible. No R2 drain/GC claim.
- Purpose-framed HMAC keys retain old versions and separate actor/operation/
  source/asset purposes; mutation/rotation tests passed in the combined File
  catalog race suite (3.212 s).
- User source dependency was narrowed with forward commit `98a4371c4b` to the
  new catalogactor proto/generated clients. Importing the whole child initially
  failed User tests on absent PR907-only helpers; server code was removed from
  this consumer branch and the restored User short suite passed. User authority
  server source/review remains owned by Anton-104/PR910, not this proof.

## Risks and follow-ups

- The User compilation dependency is now coherent: normal merge of exact PR907
  `a479a15dbdfcfb6871ef4b3e218d7bc9c6bc4d44` at `b5fa2c7034`, followed by LF-only
  correction `a284ae299270d73b348027bfccc18bddb2277fac`, preserves both User
  authority contracts. Actual User short passed (grpcsvc 8.265 s). This
  supersedes the earlier compilation-failure checkpoint, not its history.
- File now atomically creates the real internal unknown-size pending File,
  immutable source binding and operation receipt before fetch. Local gates are
  captured by exact actor and never reopened by a read. Same-body retry recovers
  identity, changed URL/body conflicts, and terminal generation fences deny
  resurrection. No raw URL is persisted. Missing API and hash-prefix REDs became
  real PostgreSQL race GREEN (8.971 s, then 6.381 s after binding hardening).
- File-owned status recovers a committed operation before Chat resolves a URL;
  protected operation status independently reauthorizes its original destination.
  The actual File terminal transition records receipt P30D once, and replay/poll
  cannot refresh it. Compaction removes raw receipt details, retains an opaque
  terminal fence and leaves independent active asset status intact. Real PG
  receipt/handler tests passed race (20.592 s); the wider catalog matrix passed
  race (64.866 s). These are storage/status primitives, not byte-write/import or
  full Messaging-send proof. Object-write declaration/settlement, recovery,
  Import handler and listener activation remain unfinished.

- Normal merge `a631b630b672ada8ece1ff93543ca31ef73551ec` consumes Chat's
  `5c05cf34358eb9beda472a6ce8502c8a80b4ce11` destination-authority protocol.
  File's signed pure-service client checks exact actor/destination echo, actual
  chat type and captured Chat generations; request-hash/signature/replay tests
  passed under race (3.239 s). No ordinary user-header fabrication is used.
- Protected File Lookup/Validate handlers now use actual locked source/gate/
  blob/representation rows and fresh User actor snapshots. Actual PostgreSQL
  matrix plus private-principal checks passed under race (19.895 s), including
  UNKNOWN never authorizing purge, soft deletion closing the local gate, and a
  later live snapshot never reopening it. No listener/startup activation yet.
- The Chat dependency merge reintroduced User catalog server tests requiring
  PR907-only helpers/stubs. User short currently fails compilation on
  `deletionTransportIdentity` and `GetProfileDeletionStates`; the issue is sent
  to Chat/User/integration owners. File checks do not qualify that server.
- Graphify refresh handle 64238 completed exit 0 (119479 nodes/248439 edges);
  visualization size and pre-existing C++ header warnings do not assert fresh
  semantic labels. The next source checkpoint requires another AST refresh.

- File profile purge now commits a CLOSED opaque generation before erasing
  private source/operation mappings. It writes purpose-scoped terminal fences,
  preserves committed message references and ready history, and rejects an
  incomplete historical keyring. Missing-method RED became four real isolated
  PostgreSQL race tests GREEN (12.648 s). Unsettled writes remain unchanged and
  do not authorize blob GC. Internal unknown-size rows are removed only with no
  write declarations/references; their surviving blob ledger enters existing
  GC. No runtime authority reconciliation or account-purge wiring is claimed.
- READY admission additionally requires an authoritative LIVE blob with no
  deletion in progress. Catalog package race tests passed (3.316 s), and scoped
  File vet passed. This predicate still requires an actual protected DB reader.

PR898 source and generated File proto ownership must be reconciled before File
API edits. Full delivery depends on the protected Chat producer, not its preview
search API. Encrypted-DM privacy semantics remain explicitly open.
