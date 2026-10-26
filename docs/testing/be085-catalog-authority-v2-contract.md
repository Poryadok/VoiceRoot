# Non-E2E catalog authority protocol 2

This is the mutually agreed Chat/File/Messaging authority update following the
independent current-selection finding. It supersedes protocol 1's currentness
and physical-write assumptions in the [import contract](be085-catalog-asset-contract.md).
It is a schema/implementation contract, not evidence that the importer, ordering
store, native worker, Messaging references or deployment are complete. No
provider is activated; the encrypted-DM identity-disclosure decision stays open.

## Wire boundary

All four private File `CatalogAssetService` requests require
`protocol_version=2`. Private Chat `ValidateGifAsset` also requires version 2.
Version 1 cannot supply admission authority and has no fallback. The public
Gateway/Chat Resolve selection envelope remains version 1, but its owning
server always uses File authority version 2 and requires the ordered proof.
This is an envelope translation, never a legacy authorization path.
The independent destination-only `AuthorizeCatalogDestination` remains version
1 because it conveys no catalog/source/current-selection authority.

File Import and Get Status responses add positive uint64
`selection_revision=13` and `current_selection_revision=14`; Status retains its
existing cleanup-only `terminal_fence_id=12`. File Validate adds those names at
fields 15 and 16; Lookup embeds that same Validate snapshot. Chat Validate
echoes the same revisions at fields 7 and 8 alongside its exact source tuple.
Successful active admission requires both revisions positive and equal, a live
head matching the exact source/File, and every existing actor/readiness/scan/
representation/destination check. Object revision remains the separate byte/
metadata identity and never substitutes for selection order.

The minimized asset-only terminal cleanup proof keeps its separate capability:
exact authenticated actor/provider/source/File echo, closed failed/deleted/
expired status, canonical nonzero `terminal_fence_id`, both Chat generations
empty and both revisions zero. One missing generation/revision or a nonterminal
proof is invalid. Import, operation status, Lookup, Validate and READY admission
never use this exception. Chat retirement also compares its captured slot
incarnation, stored revision and exact File tuple; stale A cannot delete B.

## One File-owned order

In the initial pending-File admission transaction, after existing-operation/
terminal replay lookup and actor-generation checks, File locks one selector
head scoped by exact account/profile, captured Chat and File actor generations,
and provider/provider ID. Each new admitted operation receives the next positive
monotonic revision. File persists it on the operation and asset and advances
the current source/File head in the same transaction as the real pending File.
Order is the first successful File admission commit, never response arrival,
client timestamps, UUID order or a guessed ordering of pre-existing rows.

Retries/status retain the original operation's tuple/revision and never advance
the head. Admission B supersedes A even while B is pending or later fails;
pre-admission failure does not. Old bytes and committed message references are
untouched. File Lookup/Validate and Status currentness checks use the same head;
physical File status READY alone is insufficient. An old operation cannot
become current because a newer cache row or full receipt disappeared.

The durable aggregate watermark is keyed by a purpose-HMAC of that exact actor
generation/selector scope, with opaque generations, counter and current/terminal
fence. Compact state contains no raw URL/provider/account/profile ledger. After
terminal B, retain its highwater/fence, clear detailed current identity when no
longer needed and never fall back to older READY A. A fresh live-scope operation
receives a revision greater than B. Retire the aggregate only when the whole
actor-generation namespace is conclusively retired and retained actor/operation
fences reject every old attempt. Key rotation must locate the same aggregate
across retained key versions and cannot create a second counter.

Existing version-1 rows have no trusted order. An additive migration preserves
their bytes/committed history but grants them no version-2 current authority.
Never infer revision from timestamps, UUIDs or current SQL row order. Known old
operations fail closed rather than masquerading as unknown/fresh operations.

## Disposable Chat projection

Chat keeps one per-selector cache-slot incarnation, captured with LIVE local
actor generations before the remote File call. Save is UPDATE-only and requires
the same slot incarnation, actor captures and monotonic ordered proof; it cannot
UPSERT a missing retired slot. Validation/cache hydration consult fresh File
currentness rather than treating a cached File ID as authority.

Terminal retirement deletes/rotates the exact slot. A late A response cannot
recreate it or replace B. A fresh request creates a new random incarnation and
must obtain a new File current attestation; File rejects A below its retained
head. Chat has no permanent selection floor or second per-operation receipt.
These are local stale-response barriers, not a remote-snapshot lease or a
zero-lag distributed deletion permit.

## Physical writes and retention

Catalog source and derived PDF/GIF writes share one File-owned tracked-write
protocol and physical collection predicate. A durable unique attempt key,
owner/purpose, opaque generation, measured length and digest precede PUT.
STARTED is committed before the only network attempt. Only an actual successful
single-attempt ACK can settle, using a fresh connection independently of current
File/actor publication authority; publication still requires a live exact
generation. If deletion won, settle first and queue GC, never publish READY.

Any error, lost ACK or crash leaves unresolved nonexpiring physical pinning.
Cancellation plus local goroutine drain does not prove a submitted remote PUT
cannot later commit. HEAD, age, session loss or an administrator assertion is
not that proof. SDK retries and HTTP redirects are disabled for these writes.
Every collection route, including ordinary blob/reference cleanup and direct
GIF cleanup, must use the shared predicate. Unknown remote liveness remains
explicit reconciliation work; no fabricated autonomous-cleanup claim is made.

Import/cache identity supplies no retention entitlement. Existing File reference
retention requires its immutable verified retention account; the current
FileReferenceKey/AcquireFileReferences schema lacks `retention_account_id` and
that integration gap remains open. No new TTL or fake message reference fills
the gap. Ordinary attachment ACLs and committed history entitlement stay intact.
