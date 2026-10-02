# Federation authority HTTPS v1

This executable foundation replaces the old scaffold semantics for the game
sprint. Legacy S2S protobuf remains unimplemented. No fallback token grants
authority. Dedicated mTLS HTTPS listens on :9443; health/metrics :8080 expose
no authority routes. This contract is approved in the Federation ExecPlan.

## Identity and endpoints

TLS 1.3 verifies client certificates against the configured CA. Operators must
also match FEDERATION_OPERATOR_CERT_SHA256 (comma separated lowercase hex
SHA-256 DER pins). Node pins must differ from operator pins. Node requests also
require an active registry entry and a Bearer credential. Forwarded certificate
headers are ignored. Certificate lifetime is checked on each request, including
reused TLS connections. Operator ownership attestation is an out-of-band check;
the service never fetches supplied endpoints. Approval does not establish that
an operator cannot read node disks.

Every ID is a canonical nonzero UUID. Environment is fixed in process config.
POST rejects unknown fields, trailing JSON and bodies above 1 MiB. Errors are
JSON `{error: invalid_request|forbidden|conflict|unavailable}`, with respective
HTTP 400/403/409/503. Requests have a three-second database deadline.

| Endpoint | Caller | Body / result |
|---|---|---|
| POST /v1/nodes | Operator | `{node_id,operator_id,endpoint,certificate_sha256}` creates pending node |
| POST /v1/nodes/{node}/approve | Operator | `{ownership_verified:true}` activates; returns `{credential,expires_at}` |
| POST /v1/nodes/{node}/rotate | Operator | `{certificate_sha256}` replaces credential/cert and increments epoch |
| POST /v1/nodes/{node}/suspend | Operator | `{}` closes access and increments epoch |
| POST /v1/nodes/{node}/defederate | Operator | `{}` permanently closes access and increments epoch |
| POST /v1/nodes/{node}/spaces/{space} | Operator | `{}` establishes immutable placement generation 1 |
| POST /v1/nodes/{node}/spaces/{space}/resources/{resource} | Operator | `{resource_type,routing_generation,lifecycle_state,capabilities,room_name?}` appends an immutable canonical-resource route generation; `voice_room` requires explicit `room_name` |
| POST /v1/nodes/{node}/spaces/{space}/snapshot | Operator publisher | Complete Snapshot below; sequential revision CAS |
| GET /v1/nodes/{node}/spaces/{space}/snapshot | Node certificate + Bearer | Signed snapshot Envelope |
| GET /v1/nodes/{node}/spaces/{space}/snapshot/pages/{index} | Node certificate + Bearer | Signed exact-scope snapshot page |
| GET /v1/nodes/{node}/spaces/{space}/revisions?after_revision=N | Node certificate + Bearer | Bounded signed revision stream or resnapshot requirement |
| POST /v1/nodes/{node}/spaces/{space}/lease | Node certificate + Bearer | `{revision,hash,nonce}` returns signed lease Envelope |

Successful mutations without a credential/envelope return `{status:"ok"}`.
Credentials are 32 cryptographically random bytes encoded base64url; only their
SHA-256 hash is stored. Lifetime is 24 hours, secret returned once. Rotation
revokes previous credential immediately, without overlap in this foundation.
Suspension has no implicit resume endpoint; defederation is permanent. No
operator certificate can be used on node routes. No NATS credential is issued.

Hosted resource routes are operator-managed on the mTLS authority listener.
The initial `routing_generation` is 1; retries with the exact same canonical
resource ID, type, Space, home node, lifecycle state, capabilities and
generation are inert. A change requires exactly the next generation. Resource
ID, type, owning Space and home node remain fixed for v1; there is no online
cross-node migration. A generation may change lifecycle state or capabilities
while retaining that identity. The Space must already have a master placement
on that exact home node, locked with the node for registration,
and the home node must be active in the same Federation environment when each
generation is registered. Every generation is append-only in PostgreSQL,
including terminal lifecycle states. Capabilities are limited to `messaging`,
`file`, `search` and `voice`; the caller cannot supply NATS subjects,
credentials, environment IDs or a master-owned Chat identity. This registry
records routing authority only; node data stores own hosted content, while
master services keep Space and Chat metadata. Client route discovery and node
admission grants are the downstream T74 slice.

`voice_room` routes additionally require the canonical owner's explicit RTC
`room_name` and `voice` capability. The name is valid UTF-8, 1–256 bytes, with
no surrounding whitespace or control characters. Other resource types cannot
set it. The durable binding is immutable per canonical resource and unique per
node/RTC room across Spaces. Higher routing generations cannot substitute its
room. A separate canonical room UUID and an RTC room string remain distinct;
neither is inferred from the other or from a client request.

Schema migration 5 adds these bindings and an exact placement foreign key for
new route generations. Historical room-less or misrouted rows remain intact;
the foreign key is initially `NOT VALID` to preserve that evidence. An exact
retry of a foreign historical route is denied. A room-less historical voice
route requires a new owner-supplied generation with its explicit canonical room
before media grant issuance. No migration guesses a room, rewrites a saved
route, or makes historical mismatches authoritative. `/ready` requires version 5.

## HTTP request correlation and Q11 denial audit

Every HTTP request on the authority listener receives one effective request ID.
The server preserves `X-Request-ID` only when exactly one value is a canonical,
nonzero UUID. A missing, invalid, noncanonical, or repeated value is replaced
with a server-generated UUID. The effective value is returned in the
`X-Request-ID` response header on success and error responses. This ID is for
correlation only and never establishes identity or authority.

The Q11 clean-start gate requires an audit row for its enumerated
Federation-authority HTTP denials: an operator certificate on a node-only route;
a node certificate on an operator-only route; a trusted but unregistered node
certificate/pin; a cross-node or cross-Space node request; a stale or revoked
node bearer; and duplicate node approval. Each row records a generated audit-row
UUID, actor class, lowercase SHA-256 DER certificate fingerprint, canonical
node/Space IDs when available, fixed action, `denied` or `conflict` result,
HTTP status, safe reason code, effective request UUID, and server timestamp.
Rows contain no request body, bearer, credential, claims, provider subject,
certificate bytes, or private-key material.

The fixed action values for these cases are `node.snapshot.read` (operator on a
node route, unregistered node certificate, cross-node/Space request, or stale
bearer), and `node.approve` (node certificate on an operator route or duplicate
approval). Safe reason codes are respectively `certificate_role_mismatch`,
`node_certificate_mismatch`, `node_scope_mismatch`, `credential_revoked`, and
`approval_conflict`; they are stable classifications and never include
database, certificate, bearer, or request-body details. The actor class is
`operator` for the configured operator pin and `node` for any other
verified-chain certificate, including an unregistered node certificate.

The denial row is appended after the handler/domain transaction returns, in a
separate database transaction, so a rejected operation cannot erase its audit
event. The schema rejects `UPDATE`, `DELETE`, and `TRUNCATE` against audit rows.
Audit persistence failure returns only the generic `503 unavailable` response
with the effective request ID; a denial is never reported as completed without
its audit row. Invalid TLS handshakes that fail before an HTTP request are
outside this audit contract because no request ID or authority handler exists.
Malformed routes/bodies and failures outside the enumerated Q11 denial cases do
not gain a new audit guarantee from this slice.

## Complete snapshots, pagination and signatures

The authority accepts one complete source snapshot, bounded to 1 MiB, then
exposes a deterministic paged wire representation. Snapshot fields (all
required): `version:1`, `complete:true`, `page_count`, positive `revision`,
`valid_until` Unix milliseconds, and `permissions`. `page_count` must equal
`max(1, ceil(len(permissions)/256))` and may not exceed 4096; pages contain at
most 256 permission entries. An empty permission list is one empty page and
denies all. Permissions
is the complete effective allowlist of objects with `account_id`, `profile_id`,
`resource_id`, positive `session_epoch`, and `actions` containing `read`,
`write`, `subscribe` or `media`. Missing entry means deny; missing/null
permissions is invalid. Publisher reconciles Space lifecycle, membership, Role
overrides, bans, binding/profile/session state before producing this projection.
Federation cannot write owning stores. There is no production owning-service
publisher yet; operator fixtures exercise the contract only. Unknown
schema/fields fail closed. `valid_until` is within the next two seconds. Source
expiry prevents indefinite renewal of stale authority. Same revision/same
canonical bytes is a no-op; changed bytes, skipped or lower revisions conflict.

`GET /v1/nodes/{node}/spaces/{space}/snapshot` returns one signed manifest with
`version`, `complete`, `page_count`, `revision`, `valid_until`, and `total_hash`.
`GET .../snapshot/pages/{zero_based_index}` returns one signed page with
`revision`, `page_index`, `page_count`, `page_hash`, `total_hash`, and its
permission entries. Pages are slices of the canonical source permission array;
their order is significant. `page_hash` is SHA-256 of canonical Go
encoding/json bytes of that page's permission array. `total_hash` is SHA-256 of
the canonical complete Snapshot bytes. The manifest and every page bind the
same node, Space, generation, epoch, revision, validity and total hash in their
signed claims. A page outside the manifest range, duplicate index with changed
bytes, changed manifest, bad page hash, or final total-hash mismatch fails
closed.

`GET /v1/nodes/{node}/spaces/{space}/revisions?after_revision=N` returns one
signed revision-stream envelope. Its events are in strict sequence after N,
with at most 100 events per response; each event binds the new revision, total
hash and source validity. The authority retains the latest 101 events per
placement. A detected gap, conflicting hash for one revision, history older
than retention, or response overflow sets `resnapshot_required` and omits
events. A node never infers an empty permission set from a missing event or
page. The current snapshot endpoint always offers the latest complete revision,
so reconnect can discard an incomplete stage and resnapshot.

The node stages all pages outside its active policy, verifies every envelope
and page hash, assembles the complete Snapshot, verifies `total_hash`, and
atomically activates only when every page is present. The current active
revision remains visible until that transaction succeeds. A higher revision
must be exactly current+1 when consuming the ordered stream; a gap triggers
full resnapshot. Same revision/hash is an inert replay; same revision with a
different hash conflicts; lower revisions are stale. The node sends the
existing `/lease` acknowledgment only after activation, naming the exact
revision/hash and a fresh single-use nonce UUID. A failed or partial stage
cannot renew the lease.

The node enforcement cache accepts a signed lease only for its exact active
revision/hash and treats authority as valid until the earlier of snapshot and
lease expiry. Every operation check supplies account, profile, resource,
session epoch and action; uncertainty above 250ms denies, and otherwise the
uncertainty is subtracted from the deadline. The development media watchdog
rechecks active participants on a bounded 250ms interval and ejects identities
whose `media` permission or lease is no longer valid. The separate production
node controller verifies manifest/pages/digest before ACK and atomically
publishes complete signed bundles. Its independent Space workers and pre-ACK
process-local floors reject partial/foreign/rollback policy. The maintained
node LiveKit build checks its private media admission claim and has an independent
in-process 100ms watchdog. Real two-Space RTP acceptance includes actual controller
SIGKILL while the signer advances and SFU stays healthy: files and ACKs stop
refreshing, active media expires within five seconds, and unexpired stale bearer
reconnects fail. This mechanism evidence does not replace owner projection,
production media-grant issuance, fresh-online boot, node bundle or qualified 2×
load gates.

Envelope JSON: `{key_id,payload,signature}`. `payload` is unpadded base64url of
exact UTF-8 JSON Claims bytes; `signature` is unpadded base64url Ed25519
signature of those bytes. Verify the bytes before decoding; never reserialize
to verify. Claims bind `version:1`, `kind`, `issuer`, `audience:"voice-node"`,
`environment`, `node_id`, `space_id`, `generation`, `epoch`, `revision`,
`issued_at`, `expires_at`, and `hash`; manifest/page/event claims carry their
typed payload. Times are Unix milliseconds. Consumers pin `key_id` to a trusted
public key and verify exact issuer, audience, environment, node/Space,
generation, epoch, revision/hash and expiry before atomic activation. `key_id`
is a lookup hint, never an authority source.

Lease lifetime is min(two seconds, source expiry, credential expiry). ACK must
name the exact current revision/hash and single-use nonce UUID. Nonce is retained
through credential expiry; replay cannot renew a lease. There is no expiry
grace. Transactions lock node then placement, serializing lease issuance with
revocation and publication. The publisher attempts renewal every 500ms; retries
never slide `valid_until` or extend a lease.

#### G08/Q06 revoke budget

The source publishes a committed newer revoke revision within 2.0 seconds;
without that update, the last signed snapshot/lease expires within 2.0 seconds
of the owning-authority mutation because its signed validity is capped at two
seconds. These are alternate deny paths, not additive delays. Combined
wall-clock uncertainty is at most 250ms. Nodes subtract that uncertainty from
the signed wall-clock deadline before converting it to a monotonic deadline, so
clock skew can only deny early; uncertainty above 250ms fails closed
immediately. After the deny trigger, the node/SFU enforcement path fences
active media within 2.75 seconds. The conservative allocation is
2.0s propagation/validity + 0.25s clock guard + 2.75s enforcement = 5.0s
maximum revoke-to-eject. Revoke, freeze, and lease-expiry work uses a separately
bounded priority lane with reserved worker and database capacity; message
delivery, roster snapshots, and resync cannot starve it. This is an acceptance
target, not a measured runtime claim. FED02/FED03/Q06 must measure each interval
under 2x the qualified event/snapshot load and show that unrelated Spaces remain
available.

For each measurement, start at the durable commit of the owning-authority
mutation and retain its event ID. Record commit→new-revision publication/deny
trigger and deny-trigger→final media delivery stop as separate durations. Record
command drain from its own start/stop events against the stricter 4.25-second
bound; drain completion is not a substitute for media delivery stop. Use
monotonic durations per host, retain event IDs, and include clock uncertainty
for cross-host comparisons. Exercise stale bearer reconnect after authority
expiry while the control plane is unavailable. Until qualified media runtime
evidence exists, actual media eject remains `OPEN` / `NOT RUN`.

## Storage, runtime and activation

`federation_db` owns nodes, placements, current snapshots, lease nonces and schema
versions. Migration is embedded and runs transactionally under an advisory lock.
Runtime database login must be restricted to that database. TLS/CA, signing
seed, key ID, issuer, environment and operator pins are mandatory. /ready checks
database and schema. Secrets and snapshots are never logged.

Named activation consumer is future Voice Node policy projection and media
verifier T72–T74. Live stream, owning-service publisher, player grants, resource
routing, node stores, lifecycle receipts and bundle are still pending. No media
capability is advertised. Actual media enforcement must prove the full <=5s
G08/Q06 budget above; an ordinary LiveKit JWT is insufficient.
