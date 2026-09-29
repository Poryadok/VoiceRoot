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
| POST /v1/nodes/{node}/spaces/{space}/snapshot | Operator publisher | Complete Snapshot below; sequential revision CAS |
| GET /v1/nodes/{node}/spaces/{space}/snapshot | Node certificate + Bearer | Signed snapshot Envelope |
| POST /v1/nodes/{node}/spaces/{space}/lease | Node certificate + Bearer | `{revision,hash,nonce}` returns signed lease Envelope |

Successful mutations without a credential/envelope return `{status:"ok"}`.
Credentials are 32 cryptographically random bytes encoded base64url; only their
SHA-256 hash is stored. Lifetime is 24 hours, secret returned once. Rotation
revokes previous credential immediately, without overlap in this foundation.
Suspension has no implicit resume endpoint; defederation is permanent. No
operator certificate can be used on node routes. No NATS credential is issued.

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

## Complete snapshot and signature

Snapshot fields (all required): `version:1`, `complete:true`, `page_count:1`,
`revision` positive integer, `valid_until` Unix milliseconds, `permissions`.
Permissions is a complete effective allowlist array of objects with
`account_id`, `profile_id`, `resource_id`, positive `session_epoch`, and `actions`
containing `read`, `write`, `subscribe` or `media`. Missing entry means deny.
Empty array denies all; missing/null array is invalid. Publisher must reconcile
Space lifecycle, membership, Role overrides, bans, binding/profile/session
state before producing this projection. Federation cannot write owning stores.
There is no production owning-service publisher yet: operator fixtures exercise
the contract only. Payload is bounded at 1 MiB, one complete page. Unknown
schema/fields fail closed. `valid_until` is within the next two seconds, so a
future publisher must continually refresh with a new revision. Source expiry
prevents indefinite renewal of stale authority. Same revision/same canonical
bytes is a no-op; changed bytes, skipped or lower revisions conflict.

Envelope JSON: `{key_id,payload,signature}`. `payload` is unpadded base64url of
exact UTF-8 JSON Claims bytes; `signature` is unpadded base64url Ed25519 signature
of those bytes. Verify the bytes before decoding; never reserialize to verify.
Claims: `version:1`, `kind:snapshot|lease`, `issuer`, `audience:"voice-node"`,
`environment`, `node_id`, `space_id`, `generation`, `epoch`, `revision`,
`issued_at`, `expires_at`, `hash`, optional `snapshot` (only snapshot kind).
Times are Unix milliseconds. Hash is lowercase hex SHA-256 of canonical Go
encoding/json Snapshot bytes, persisted without reserialization in PostgreSQL.
Consumers pin key_id to a trusted public key and verify exact issuer, audience,
environment, node/Space, generation, epoch, revision/hash and expiry before
atomic activation. key_id is a lookup hint, never an authority source.

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
