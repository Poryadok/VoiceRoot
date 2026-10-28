# Dedicated DEV story-media replay store

This is an operator-rendered fixture, not an active deployment or a production
qualification. Freeze the rendered manifest and record its digest with the pinned
Redis image digest and the consumer's exact File build before admission. Resolve
every `__...__` placeholder; use a unique DNS-safe cohort and names. Record the
namespace, Deployment, Service, PVC and PV identities, their UIDs, storage class,
capacity, and the relevant resource versions. The operator owns the PVC, Secret,
File configuration, claims and admission. This template creates none of them.

## Boundary and configuration

The connection is plaintext Redis TCP, including AUTH. DEV relies on the documented
trusted namespace boundary (see `docs/DEV_SERVER.md`); verify the actual CNI and
cross-namespace isolation. Namespace peers can reach the service under the existing
additive policies. This fixture does not provide peer isolation or TLS, and must
not be presented as a production security boundary. The password is a fixture
choice, not a requirement established by the File contract.

Provide a cohort-specific immutable Secret with key `password`, containing 64
lowercase hexadecimal characters generated with a cryptographically secure random
source. Never print or commit its value. Configure File's replay Redis address to
the rendered Service on port 6379 and its password from the same Secret; privately
verify the bindings. Confirm that the pinned image is available for the target
platform and that the rendered object, Secret and PVC bindings match the recorded
package. Redis readiness establishes availability only; it proves neither volume
lineage nor freshness.

## Storage and recovery

Before first initialization, fence and drain all File verifiers for the cohort.
Check the actual PVC and PV UIDs and exclusive storage attachment. An operator
with the storage claim must initialize a verified empty volume by atomically writing
the exact bytes `<PVC-UID>|<PV-UID>` without a newline to
`/data/.voice-replay-binding`, then fsync the file and its parent directory and
record the identities and operation. Do not automatically initialize an unknown
volume. Startup rejects a missing or different marker, but the marker can be copied
or forged: it does not prove UID identity, continuous lineage or fresh data.

Before admitting File, establish that the actual storage provides the durability
and attachment fencing required by Redis AOF fsync. One replica and `Recreate` do
not establish fencing during node failure. Verify actual UIDs, complete AOF lineage,
and no other Redis writer. Keep admission closed when these facts are uncertain.
The configuration requests synchronous AOF and rejects truncated AOF; readiness
does not replace inspection of failure and recovery evidence.

Each container start creates a new random `/run/replay/nonce` and waits without
starting Redis. The operator must verify storage identity, fencing and history for
that exact Pod and container incarnation on every start, including automatic
restarts. After performing the appropriate recovery procedure below, the operator
opens the gate by writing the observed nonce to `/run/replay/admit` in that same
container; confirm the Pod UID, container ID, restart count and nonce have not
changed during verification. The process consumes the gate before starting Redis;
a gate from a previous container start cannot authorize a new one. Restrict exec
access to the operator. If identity or history changes during verification, start
the verification again; never automate gate opening merely on readiness or marker.
The gate itself is not an attestation of storage history. If File is already live,
keep it fenced until the store has been verified and opened.

Back up the complete `/data` tree, including the AOF manifest and every referenced
base and incremental file, while Redis is stopped and storage is fenced. Retain
the binding record separately and verify backups with an isolated restore. Never
repair the original AOF in place or infer freshness from a successful restore.
For a stale, replaced, unknown or unprovably continuous volume, stop and drain
**all** previous File verifiers, record the latest possible successful replay SET,
and keep them stopped for at least 35 seconds after that point. This is valid only
when verifier/issuer clocks and offsets are qualified, without backward adjustments
or uncertainty that could extend credential acceptance; otherwise remain closed.
Prove the old writers cannot resume before admitting the replacement. The marker and
the manifest do not automatically enforce this recovery procedure.

The isolated `story_media_replay_qualify.py` exercises the exact startup script and
Redis configuration with disposable Docker volumes: missing/mismatched binding,
the per-start admission gate, authentication, process kill/restart, no-eviction, offline restore, stale backup,
and truncated AOF. It does not prove Kubernetes storage durability, power-loss
behavior, network isolation, File integration, or the operator's recovery history.
