# Non-E2E catalog asset protocol v1

This is the concrete implementation contract for `BE085-FILE-MESSAGING-ASSET-IMPORT`
and the protected Chat consumer of `BE-085`. It implements PLAN A6 and the owning
File, Chat and Messaging contracts. It does not activate a provider or resolve
the encrypted-DM catalog-identity privacy contradiction. Implementation and
verification progress are in [the ExecPlan](be085-catalog-asset-import-plan.md).

## Source identity and trust

Chat is the sole GIF resolver. It obtains the source URL from its configured
provider adapter or authoritative cache, never from a client URL or file ID.
It persists an immutable source generation: UUID `source_id`, `provider`,
`provider_id` and SHA-256 of the exact resolved HTTPS URL. URL rotation creates
a new source generation. The UUID is an identity, not an access capability.
The same URL may also receive a fresh generation after a profile's File reaches
terminal `failed`, `deleted` or `expired`. There is no permanent unique constraint
on `(provider, provider_id, URL hash)`. One profile's terminal binding does not
invalidate another profile's still-ready binding; validation reads the exact
current per-profile selection, never a global active-source pointer.
The imported File binding is `(actor_profile_id, source_id) -> file_id`, also
bound to the actor account. It must not be treated as one global file per source.
Provider metadata MIME/size hints do not override File's byte validation/limits.

The client need not prove that an asset appeared in its particular search page;
that is not a product requirement. Chat must independently resolve the requested
provider identity. Missing/unconfigured sources fail closed, with no seeded fake
product results. Search DTOs remain unchanged and may omit `file_id` until an
actual import has a committed File row.

## Protected API matrix

New services use separate `voice/chat/v1/catalog_assets.proto` and
`voice/file/v1/catalog_assets.proto`, each in its owning existing package and
named `CatalogAssetService`. Existing service/proto methods remain unchanged.

| RPC | Credential / principal | Request fields in field-number order |
| --- | --- | --- |
| Chat `ResolveGifAsset` | delegated `service:gateway`, issuer gateway, audience chat; active account/profile come from verified credential | `protocol_version=1`, `operation_id=2`, `provider=3`, `provider_id=4`, `destination_chat=5` |
| Chat `ValidateGifAsset` | pure `service:messaging`, issuer messaging, audience chat | `protocol_version=1`, `actor_account_id=2`, `actor_profile_id=3`, `provider=4`, `provider_id=5`, `file_id=6`, `destination_chat=7` |
| File `ImportCatalogAsset` | pure `service:chat`, issuer chat, audience file | `protocol_version=1`, `operation_id=2`, `source_id=3`, `provider=4`, `provider_id=5`, `source_url=6`, `actor_account_id=7`, `actor_profile_id=8`, `destination_chat=9` |
| File `ValidateCatalogAsset` | pure `service:messaging`, issuer messaging, audience file | `protocol_version=1`, `source_id=2`, `provider=3`, `provider_id=4`, `file_id=5`, `actor_account_id=6`, `actor_profile_id=7` |
| File `LookupCatalogAsset` | pure `service:messaging`, issuer messaging, audience file | `protocol_version=1`, `actor_account_id=2`, `actor_profile_id=3`, `file_id=4` |
| File `GetCatalogAssetStatus` | pure `service:chat`, issuer chat, audience file | `protocol_version=1`, `actor_account_id=2`, `actor_profile_id=3`, oneof `operation=4` or `asset=5` |

All version fields are uint32 fixed to 1; IDs are canonical nonzero UUID strings;
ChatRef uses the existing canonical Chat reference. Raw identity metadata,
duplicate credentials, unknown fields and absent/mismatched request binding are
rejected. Each call uses a fresh short-lived credential/JTI bound to full RPC,
request hash and request ID, even when retrying a durable operation. The durable
operation UUID is separate from transport request/JTI. Listener registration,
issuer trust, mTLS and replay checks must exist before any handler is reachable.
Protected RPCs reject public/ordinary-listener calls; there is no forwarded-header
compatibility fallback. Use existing Phase-0 key rotation/cache requirements.

Chat Resolve response fields are `source_id=1, provider=2, provider_id=3,
file_id=4, status=5`. File Import response fields are `source_id=1,
actor_account_id=2, actor_profile_id=3, provider=4, provider_id=5, file_id=6,
status=7`. Both use File's closed status vocabulary: `pending_upload`,
`processing`, `ready`, `failed`, `deleted`, `expired`. A committed import normally
first returns `processing`; there is no fake READY result or fabricated file ID.
Source URL and storage keys never occur in either public response.
File Import also echoes `operation_id=8, destination_chat=9`.
Import request and Import/Get Status responses additionally carry opaque UUID
`chat_account_generation=10, chat_profile_generation=11`. Chat captures these
LIVE local actor generations before File calls, persists them with the binding
under ordered actor locks, and rejects recovered operations from an older
generation. They enter the internal import hash, not the public-selection hash.
They are opaque captured data; File independently enforces its own local actor
fences and cannot treat Chat's generations as File authority. Save never opens
or restores a closed actor gate. A verified restore needs a new generation and
operation and never revives an old receipt.

Get Status's operation selector contains `operation_id=1, provider=2,
provider_id=3, destination_chat=4`; its asset selector contains `source_id=1,
provider=2, provider_id=3, file_id=4`. Its response uses File Import's identity,
status and operation/destination fields; the latter two are present only for
an operation selector. Operation lookup happens before resolving any source URL
when Chat has no recorded result. File stores a separate public-selection hash
(version, operation, provider identity, destination and actor) in addition to
the full internal import hash. This permits recovery of the original source/File
after File commit plus Chat response loss even if the CDN URL has rotated.
Changed retained public selection conflicts. Only unknown operation `NotFound`
permits a fresh import; compact terminal `FailedPrecondition` never does. Asset
lookup permits polling an active mapping beyond the completed operation-detail
window without retaining raw URLs or a second full receipt in Chat.

Gateway owns `POST /api/v1/gifs/resolve`, transporting the Chat Resolve request
through the protected delegated-principal client. It derives the active actor
from authentication and never accepts actor identity or a source URL from JSON.
The request operation UUID is stable across client retries.

Chat Validate response fields are `source_id=1, actor_account_id=2,
actor_profile_id=3, provider=4, provider_id=5, file_id=6`. It is read-only:
no resolving new sources, importing, downloading or elevating metadata access.
It verifies the exact current stored source/profile/File tuple and independent
current destination membership and non-E2E state before returning a binding.

File Validate returns the same binding fields 1..6, then `object_revision=7`
(uint64), `intent=8` (closed string `gif`), `scan_result=9` (`clean` or `skipped`),
`file_type=10` (`video`), `source_sha256=11` (32 bytes),
`delivered_sha256=12` (32 bytes), `delivered_size_bytes=13` (int64 positive),
`delivered_mime_type=14` (`video/mp4` or `video/webm`). It succeeds only for a
non-E2E, unexpired, undeleted READY File whose persisted source binding, upload
intent, scan and delivered representation satisfy these predicates. It returns
no source URL, R2 key, uploader metadata, or unrelated File record. Public
`GetBulkMetadata` and its ACL do not change.

File Lookup is the authoritative discriminator for every file-backed attachment,
not only payloads containing optional provider attribution. Its response is a
oneof: `catalog` with the same validated catalog snapshot, or `ordinary_file=true`
only for an existing noncatalog File. Known catalog with a wrong actor denies;
unknown File or unavailable lookup errors. There is no ordinary fallback after
an error. A known catalog File requires GIF content/attachment kind. Messaging
checks any supplied attribution against the canonical binding; omission does
not waive that binding or choose a different authorization path.

## Durable import and retry lifecycle

The caller owns a stable operation UUID for the selected source and destination.
Chat and File persist the complete deterministic request fingerprint with it.
File's fingerprint includes all request fields, including account/profile,
destination and exact URL. Same operation with changed data is `AlreadyExists`.
Import retries recheck current target membership, authoritative chat type/E2E
and File-owned entitlement policy before reading/continuing a retained import.
No accepted source mapping or previous import substitutes for destination access.

File atomically creates an internal pending File, source binding and import
operation receipt before fetching. It does not create a separate pre-File
reservation or persist the raw URL/full request. A catalog-only pending marker
permits unknown `size_bytes=0` solely in this internal initial pending stage;
the fetch budget is stored separately. Ordinary uploads retain their positive
size constraint. Public metadata/list surfaces hide this unknown-size internal
row. Actual positive length and digest must be persisted and the pending marker
cleared before object write or processing. This avoids presenting a byte budget
as measured size to metadata, quota, events or workers.

File keeps a stable File UUID under `(actor_profile_id, source_id)` and an
immutable account/provider/source-URL-hash binding. Competing imports serialize
by operation and then profile/source. A bounded session advisory lock may protect
the synchronous fetch without holding a SQL transaction over network I/O; loss
of the session releases the lock. Durable state, not process memory, controls
recovery. Different operations for the same binding reuse the same reservation;
different source data under that binding conflicts.
Chat's current selection is keyed by account/profile/provider/provider ID. An
old retained Resolve operation returns its immutable recorded source/File tuple;
it never replaces a newer current selection with stale data. Validation checks
the current per-profile selection, so an old result is not authority for a new
send after that binding has been superseded.

After fetching bounded valid GIF bytes, File persists their actual digest and
length before object write. A retry after an uncertain write may refetch only
if its digest/length match the persisted bytes; it cannot replace the source
under the same identity. Storage uses the reserved File UUID. File then commits
the processing handoff for the actual File using the existing GIF processing
path (PR898), with persisted `upload_intent=gif`, original identity and File-owned
size policy. An internal pending File is never sendable. Recovery takes the
same exclusive session fence after the old executor is conclusively gone;
timeout/age alone is insufficient. If no write began, or storage conclusively
lacks the expected committed object, it terminalizes the operation/File as
FAILED and removes recovery-sensitive data. If an object exists it verifies
the persisted digest and length before continuing. Uncertain object state
remains reconciliation work. No endless refetch obligation or new orphan TTL
is introduced. Late writes and completion require the current operation
generation and nonterminal File. Unreferenced staged bytes use existing durable
GC intent; no fake message reference is acquired. Processing failure follows the
existing processor's fenced retry rules. Failed/expired/deleted files never
become sendable merely by replaying an import operation. Physical source
replacement needs a new source generation and operation.

Every acquired durable message reference retains its own immutable verified
`retention_account_id` and existing reference deadline rules from
[file-storage.md](../features/file-storage.md). Binary dedup and import cache
identity do not move those owners/deadlines or override a live reference.
Sharing the same File ID across independently authorized chats of that profile
is permitted only through the typed catalog admission below. Binary dedup may
reuse physical bytes; it grants no profile or chat authority.

## Remote-byte policy

Only protected Chat-resolved HTTPS URLs enter File. File uses deployment-owned
exact provider hostname allowlists, no wildcard/subdomain inference, proxy or
client-supplied transport. Userinfo, fragments, IP hosts, trailing-dot hosts and
ports other than 443 are rejected. Each redirect (maximum three) is revalidated.
Every redirect removes Referer, including same-host redirects, so signed source
query parameters are never forwarded to another request.
Every DNS answer must be a public global address; mixed safe/unsafe sets reject.
Each connection resolves again and pins the validated IP while TLS verifies the
original hostname. Private, loopback, link-local, multicast, documentation,
benchmark, shared-address and IPv4/IPv6 transition targets reject. No second
hostname resolution occurs in the socket dialer.

Fetch time is at most 20 seconds; headers and TLS each have a five-second bound.
The response has a bounded header size, HTTP 200, uncompressed GIF MIME and GIF
magic bytes; body length is capped by File policy and the existing processor's
limit. Errors never expose signed source query parameters. Byte validation does
not replace actual scan/transcode/representation validation before READY.

## Messaging admission and history

Messaging derives account/profile/destination from its authenticated send.
It classifies File IDs through protected File lookup, never optional client
attribution. For a catalog GIF it calls protected Chat validation, then File validation
with the returned exact source binding, checking all echoed identity fields.
It still applies current destination membership, role/communication policy,
chat type/E2E and existing send receipt rules. Missing dependencies fail closed.
Only this typed catalog path may replace the ordinary origin-chat equality
predicate. A GIF-shaped client payload, provider attribution or cached file ID
alone does not select an authorized path. Ordinary attachments keep their ACL.

AcquireFileReferences happens only after source and destination checks; it is a
lifecycle action, never source authorization. Readback requires a committed
message reference plus current history entitlement. Sticker sends retain exact
installed pack/sticker/File checks; uninstall denies new sends and preserves
committed history. The new GIF source service does not authorize sticker ingest.
Existing exact accepted send receipts remain exact retries, not fresh sends.

Encrypted destinations are rejected before resolving/disclosing source identity.
This protocol does not add server-visible catalog identity to an E2E envelope or
reuse a plaintext shared File as per-DM ciphertext. That separate canon decision
and its tests remain open.

## Receipt minimization and terminal replay

Catalog import is explicitly a File lifecycle operation. The existing File
lifecycle completed-receipt window is extended to this named operation: full
completed receipt details retain P30D from original completion; replay does not
refresh it. Account/profile privacy purge may shorten raw-detail retention and
never waits for P30D. Chat uses File's terminal authority and does not retain its
own eternal full operation receipt. Source URLs are ephemeral; active retries
resolve again through Chat and must match the immutable URL hash, otherwise
they need a new source generation and operation.

On terminalization retain only a purpose-scoped replay fence, terminal outcome
and opaque File lifecycle generation/fence. Purge provider/URL metadata, raw actor
IDs, full response and request hash when no longer needed. A purpose-HMAC of
account/profile/operation may index this fence using the existing lifecycle
deletion-key discipline; it is pseudonymous sensitive replay data, never
anonymization or analytics. After compaction an old operation returns terminal/
not-available even for changed body: exact-success/AlreadyExists semantics apply
only while full details remain. The old operation cannot bootstrap a missing
mapping. If the existing key discipline cannot support this, use a server-issued
existing-only File operation handle whose absence denies; never add an eternal
raw actor/operation ledger. Account/profile purge erases its mappings and raw
receipt identity even if other profiles' exact references keep binary bytes.
