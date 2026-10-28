# DEV media dump producer

`story_media_backup_capture.Producer(expected, guard, drain, journal)` is a
streaming producer for `story_media_backup.export`. It executes only fixed
`file_db`, `story_db`, `messaging_db` and password-free roles dumps in the
`postgres` container of `voice-postgres-0` in `voice-dev`. It supplies bounded
kubectl and PostgreSQL deadlines, sends stdout directly to the encryption
process, and keeps diagnostic stderr private and out of receipts. It does not
create an operation, claim, drain, backup key, off-node copy or restore target.

The consumer must supply immutable `expected` operation and snapshot IDs,
namespace/StatefulSet/Pod/PVC/PV identities, container ID/restart count,
mount and PostgreSQL data-directory paths and server major. Read these from
the current environment under the operation's actual authority; historical
values are not authority. The caller provides real `guard()` and `drain()`
checks, plus a durable `journal` implementing `begin(member)`,
`uncertain(member)` and `complete(member, receipt)`. No default authority or
journal exists. `begin` must guard, durably record the unique pending attempt,
fence the step and guard again; a previously pending attempt must not replay.
`uncertain` must retain the lock and persist uncertainty for reconciliation;
`complete` must bind the exact immutable operation/member/identity values.

The producer checks observed Kubernetes identities and claim/volume links,
container status, data directory, PostgreSQL major and mount namespace before
and after the dump, and rechecks guard and drain after observations. These
observations use the trusted Kubernetes/engine boundary; they are not atomic
physical storage or container-runtime attestation. The caller must maintain
the actual fence and drain throughout the capture window, including external
requests and all relevant writers. Timeout, nonzero exit, failed postcheck or
failed journal completion after begin are uncertain, and must not be replayed
until the operator reconciles the remote process and restores authority.

`complete` records a **dump-terminal-only** receipt: operation/snapshot/member,
Pod/PVC/PV UID, container ID/restart count and exit 0. It is not a successful
encrypted backup or restore. `export` can wrap a producer exception in
`BackupError`; the operator must read the durable journal before deciding on
recovery, never infer replay safety from an exception type. Only after all
encrypted streams and their authenticated manifest complete may the consumer
bind them to that snapshot in a separate durable phase. Off-node custody,
isolated restore, migrations, binary checks, MinIO and NATS remain separate
required evidence; this adapter alone never grants rollout admission.

`story_media_backup_capture_test.py` exercises synthetic authority, drift and
uncertain outcomes. `story_media_backup_capture_integration_test.py` exercises
the probe and `pg_dump` against a disposable Linux PostgreSQL container with
synthetic Kubernetes metadata. It does not call Kubernetes or establish DEV
authority; use the same isolated Docker runner described in
`story_media_backup.md`.
