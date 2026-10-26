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
  tests and exact-source evidence. The named activation consumer is non-E2E GIF
  selection; live provider activation remains disabled.
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

- Native byte-backed READY proof passed without a fake converted representation
  (7.329 s, no skip): 127-byte generated GIF fixture went through the actual
  source PUT, existing durable GIF worker and native FFmpeg 6.1.1 into a 1613-byte
  MP4 and 68-byte WebP. Protected File lookup attested the exact source/delivered
  digests, size, video category and revision 3 from real PG/object bytes. Actor,
  destination and source-byte adapter are explicit engineering fixtures; this
  is not an actual Messaging insertion or external-provider proof.
- Windows lacks ffmpeg on PATH. The engineering-only codec helper was built
  from this checkout for Linux and ran in immutable image
  `sha256:771212dd0203520df0a33fd375e088113f944e43eb536e367b19ba8bfd6e79d9`
  with no network, read-only root, one CPU/512 MiB and bounded temporary space.
  Helper SHA256 was
  `357981d49d90afead3658f0d8bed7abcf6a7420991ffc9b2d8feec56934ce45c`.
  No image application/service or production resolver was started.

- Independent Anton-140 marked `89ce2f2dd9948555806612b0b6838fcfe74f16b5`
  NOT CLEAR: ordinary GIF original cleanup bypassed the source write pin and
  could declare deletion complete before a late PUT. A real owned HTTP PUT plus
  PostgreSQL MarkDeleted/Cleanup test reproduced both acknowledged and lost-ACK
  cases (RED). The shared collection predicate now filters candidates and
  rechecks after the File lock with a fresh snapshot. Source writes cannot be
  deleted/completed by age; actual ACK permits cleanup after deletion, while a
  lost ACK remains explicit reconciliation. Focused race GREEN (10.011 s).
  The ordinary blob/reference/legacy paths retain their existing lifecycle
  authority and the database pin; this is no blanket end-to-end clearance.

- The protected Chat Import handler now creates the real File/receipt before
  fetch, stores actual length/digest and a unique write declaration before PUT,
  rechecks current actor/destination before PUT, and queues the existing GIF
  worker only after an acknowledged write. Replays keep the original File and
  perform no second fetch/PUT; destination loss during fetch terminalizes the
  prepared, never-started write. Missing-API RED became real-PG handler GREEN
  (37.658 s, then 72.900 s including compensation).
- The new source PUT uses one SDK attempt and follows no HTTP redirect. Real
  HTTP success/503/307 tests passed race (4.475 s), proving no hidden retries or
  redirect writes. Every write error stays UNSETTLED, and a database guard keeps
  ordinary reference/expiry GC from collecting STARTED/UNSETTLED source writes.
  Timeout/session loss is never treated as runtime/storage fencing evidence.
- Real PG source-write/handler matrix passed race (114.764 s); File short and
  scoped vet passed. These tests inject the source-byte provider/writer where
  stated and do not prove native codec READY, actual cross-service Messaging
  insertion or production listener configuration. Source/actor reconciliation,
  terminal mapping retirement, named recovery/cleanup consumer and final
  catalog reference-acquisition barrier are still required before activation.
- Existing FileReferenceKey/AcquireFileReferences do not carry
  `retention_account_id`; committed live-reference preservation is implemented,
  but this source slice does not invent or certify missing per-reference
  retention-account policy. The gap is reported to root and remains open.

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

### Protocol 2 ordering and shared physical custody checkpoint

The isolated protocol-only branch is frozen at
`19ad40c1b77dee9716d1af040f67447c101a448e` (parent exact `d3d0e5`), with no
import-executor ancestry. The isolated tracked-write core is frozen at
`1484a90b1924abb0a731ed5458957235b304e33c`, parent `c46b13b4`; PDF owner126
consumed it by normal merge. Migration11 is owner126's schema-only
`8e8de42bcb4c9b8f757d814a884bda29b337bc3f`, consumed without PDF source edits.
The core retries only idempotent database settlement, never PUT. Real
terminated-connection RED57P01 became race GREEN8.888s. Source settlement now
records the physical outcome before caller/actor publication checks (source
`73817aa183333d86954cb000548b7db310e20c3d`); cancellation RED STARTED became
GREEN in the focused write/purge/direct-cleanup race matrix21.126s.

Migration12 and the File store now assign one positive selection revision in
the pending-admission transaction. The stable account lock and complete retained
HMAC keyring locate a single selector counter across rotation. Operations/assets
carry immutable admission proof; retained v1 rows are historical and cannot
become new v2 admissions. Retry returns its original revision and current head.
File Lookup/Validate require an exact live current head in addition to existing
ready/scan/representation/actor predicates. Status reports both revisions;
only the separate exact minimized terminal-asset proof permits zero revisions.
Terminal File transitions/source retirement clear current identity but preserve
the opaque aggregate highwater and terminal fence, never restoring older READY
assets. Compact head state has no raw actor/provider/URL fields.

Actual PostgreSQL tests cover A1/B2, retry A1/head2, terminal B then fresh3,
READY A denied by pending B and after B failure, simultaneous admissions2/3,
key rotation without a second counter, missing retained keys, and legacy replay
failure rather than fresh NotFound. Focused ordering race14.448s, complete
catalog matrix84.266s, File short and scoped vet passed. Native byte proof was
rerun under v2: actual127B GIF ->1613B MP4 +68B WebP, revision3, using the pinned
FFmpeg helper/image and actual File worker/object bytes. Actor/destination/source
authorities remain injected in that engineering test; this is not cross-service
Messaging insertion/acquisition proof.

Activation remains incomplete: File runtime composition/reconciliation,
catalog reference-acquisition currentness and immutable retention-account
contract, derived GIF tracked writes, unknown remote-write recovery and purge
minimization remain required. The current unresolved-write File FK/tombstone
does not justify retaining raw uploader/profile identifiers indefinitely.
Encrypted-DM identity disclosure and installed-sticker source authority remain
outside this GIF/non-E2E implementation.

The next reference checkpoint reuses the authoritative current/READY reader in
the actual File acquisition transaction. A real PG regression first reproduced
catalog acquisition during processing (expected error, actual success), then
passed with the guard. New acquisition after supersession is denied; exact
completed-operation replay preserves one original reference/receipt. Two
concurrent current-source references pass without blob lock upgrade, and an
ordinary processing File still acquires under the original pre-catalog schema.
Focused reference race10.627s passed. The native byte proof now also commits one
actual File message reference after READY (6.361s, no skip); it still injects
actor/destination/source authority and does not invoke Messaging insertion.

The derived-output checkpoint adapts the existing GIF worker to the one shared
typed write ledger. The native assertion first failed (expected two settled
output declarations, actual zero), then passed with real MP4/WebP bytes and
both single-attempt ACKs. READY now requires the exact settled catalog source
and both output identities plus live blob/current worker ownership. Ordinary
pre-ledger GIF worker checks still pass (7.598s); catalog rows cannot fall back
when typed custody is absent. Ordinary GIF confirmation of an internal catalog
source was independently RED (unexpected success), then rejected in the store
without changing the shared ConfirmUpload/PDF handler.

Actual HTTP delayed PUT plus PostgreSQL tests pass under race (13.151s): an
expired processing lease and File deletion cannot clear STARTED; actual ACK
settles before failed publication and allows cleanup, lost ACK remains
UNSETTLED, only one network attempt occurs and no thumbnail starts after
authority loss. Direct completion with only declared outputs fails even if
the caller flips its job flag. Native codec + current v2 READY + real File
reference passed race7.458s (127B ->1613B MP4 +68B WebP, revision3), with the
same pinned codec image/helper. The stdout capture wrapper had a Windows
console-encoding error after writing the complete PASS log; the saved UTF-8 log
contains the actual successful test result and is retained as evidence.
File short, focused vet and diff checks passed. Unknown remote-write liveness,
unreferenced raw-actor tombstone minimization, production runtime composition,
actual cross-service Messaging insertion and A7 retention remain open.

The last-reference variant found an additional real RED: Release during the
derived PUT left the blob LIVE behind its physical pin, then the old worker
published READY after ACK. The worker now distinguishes logical retirement,
preserves unknown pins and runs a bounded retired-write consumer before new
processing. Actual release/late-ACK and restarted-consumer cases restore GC work
without another PUT; both canonical and uncommitted output keys are cleaned.
An actual Acquire paused at receipt insertion wins over reconciliation when it
commits before the blob lock. The first combined lock/subquery check was RED
because its reference subquery retained an older READ COMMITTED snapshot; a
separate reference statement after the blob lock fixes that race. These real
PG cases passed race11.304s; the broader derived/recovery/legacy-worker matrix
passed race26.469s. File short and scoped vet passed.

Independent140 review closed the old direct-cleanup P1 at exact6a273e72 and
cleared isolated shared core1484 (TT c8). Narrow reference identity/currentness
seam review139 found no additional issue (TT c9), but reference head e63d5d55
remains NOT CLEAR on140's fresh-expiry P2 (TT c7): its clock was captured before
authority lock waits. That causal correction is the next separate checkpoint;
no current admission-complete or activation claim is made. Messaging2 consumer
upgrade is owned by114 and frozen at d8a95d2083a510e4797e17e236ed48c94d42a52a;
its new independent review is pending and does not inherit old v1 clearance.

Astra101 approved the next additive13 privacy correction: derive and retain
immutable blob binding in the same tracked ledger, preserve durable retirement
of all attempt keys through File erasure, and settle late ACK without reviving
File. No released-reference/capability/history rows may be cascaded to erase an
uploader. Until implemented and independently reviewed, unreferenced measured
File/raw-uploader retention behind an unknown write pin remains OPEN.

The final native capture after the retired-write consumer passed race22.744s,
with exit0 persisted alongside the UTF-8 log. This supersedes the earlier
console-only wrapper failure; no native test skip or codec stub was used.
