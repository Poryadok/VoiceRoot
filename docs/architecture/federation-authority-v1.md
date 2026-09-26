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
schema/fields fail closed. `valid_until` is within the next five seconds, so a
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
through credential expiry; replay cannot renew a lease. There is no heartbeat
renewal and no expiry grace. Transactions lock node then placement, serializing
lease issuance with revocation and publication. Consumers must combine wall
clock deadline with monotonic elapsed time and fail closed on clock uncertainty.
500ms renewal is a proposed technical default, not a measured acceptance result.

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
propagation+lease+skew+eject budget; an ordinary LiveKit JWT is insufficient.
