# Known staging NATS baseline and cold backup

This authorized staging-only operation preserves NATS identities and non-NATS
state. The earlier historical 566-consumer/eight-message criterion is superseded,
not passed. The populated ACL custody proof remains separate.

Before changing staging, an owned isolated broker uses the pinned NATS2.12.12
image and selected existing bootstrap/Social/Realtime identities. It creates
the reviewed application baseline and publishes three synthetic fixture records
only into its own storage. Each record binds event_id, subject, stream sequence,
exact payload SHA256 and canonical header SHA256. Fixed Realtime durable
rt_realtime1_social ACKs record1, leaves delivered record2 unacknowledged, then
disconnects before record3 is published. No business service receives fixtures.

Stop broker before cold full-store backup. Restore into owned empty isolated
storage, compare canonical stream/consumer configuration and scalar delivery,
ACK and pending state BEFORE subscribing, then drain exactly records2/3 with
byte/identity checks and verify final ACK state across broker restart.
Elapsed-time redelivery counters/timestamps are excluded from stable census
equality. Record2 must report at least its second delivery before ACK; final
consumer ACK and delivered floors are4, covering all3 unique records. The
logical restore drain checks records2/3; archive per-file hashes preserve the
already ACKed record1 bytes. A timeout or mismatch cannot be PASS.

After this proof, the human-operated reviewed controller fences staging, creates
the final clean PVC preserving existing credentials and Service/DNS, and runs
the selected canonical application bootstrap. The closed zero-message staging
15-stream/42-durable baseline gets its own cold archive and isolated restore
verification before services restart. Snapshot scope and post-restart growth
are separately reported. Node-private archive plus separately verified off-node
copy excludes all credentials/configs/signing seeds. Retain the five old PVCs.

The frozen deployed contract is15 streams/42 durables, captured from four
existing ConfigMaps on2026-10-04. Current source intent has43 and different
chat_events subjects. This operation uses exact captured deployed scripts;
it does not update service releases, expand ACL grants, or provision source43.

The reviewed delivery binds a public bundle SHA256 and byte count before
capturing code into a root-owned private directory. The operator runs the
instantiated launcher with `--prepare`. Gateway,18 existing NATS leaves and
the hub stop after the isolated fixture succeeds. Expect staging downtime
through the off-node copy checkpoint and resume; non-NATS stores persist and
may retain records whose earlier NATS notifications are no longer queued.

Prepare prints the exact operation directory and `copy-checkpoint.json`.
It stops with `AWAITING_OFF_NODE_COPY`: the new active claim is selected, all
target replicas are0, and two archives have independently verified restores.
Copy only the reported archives/manifests into the assigned checkout's
`.local/known-baseline-<operation>/`, verify both SHA256 values, and retain them
until explicit owner disposal. Old retained PVCs are forensic rollback sources,
not this scenario's verified backups. The actual staging archive captures its
closed bootstrap baseline with0 messages; the fixture archive has3 records.

Run the same hash-pinned launcher with `--resume <operation-directory>
<fixture-sha256> <staging-sha256>` after the off-node receipt. Resume revalidates
code, credentials metadata, deployed ConfigMaps, maintenance ownership, exact
PVC/PV/path, zero replicas/no mounts and native store inventory before restart.
The four-hour custody TTL can be extended with `--refresh <operation-directory>`
only while all those facts remain unchanged. `--status <operation-directory>`
prints bounded sanitized diagnostics. A global nonblocking root-owned lock
prevents concurrent operations; process death releases the lock.

Failures do not automatically unfence or replay data. The durable private
journal retains original templates, observed resource identities and progress.
`VERIFIED` fence status requires observed completion; failed ownership/refence
is `UNKNOWN`. An interrupted prepare or failed restart requires a separately
reviewed recovery using that journal; refresh/resume accept only the completed
copy checkpoint. Do not run legacy root rotation on a marker carrying dataPVC.
This scenario is not historical preservation, ACL signoff or service E2E.

The scoped GitHub workflow runs Linux Go tests, Python safety guards and the
same production fixture/zero-baseline cold archive/restore functions with
disposable test-only JWTs and pinned NATS/NATS-box images. It never accesses
staging credentials or Kubernetes. Operator delivery requires expert acceptance
and exact-head scoped CI success; source templates alone are not activation.
### Interrupted fence verification: operation 0049430b0dbb

The human operation stopped at `FENCE_STAGE` with `command_output_limit` after
scaling the hub, Gateway and 18 leaves to zero. The old joint Pod/ReplicaSet
JSON was 3,187,996 bytes, exceeding the unchanged 2 MiB subprocess cap.
The fixed typed projection reads only kinds, UIDs, owner UIDs and PVC claim
references (50,731 bytes in the observed namespace); historical templates,
environment values and annotations are excluded. Row/shape limits fail closed.

`--continue-fence /var/lib/voice-nats-preservation/known-baseline-0049430b0dbb`
is a narrowly guarded human-root continuation for that exact blocked operation.
It accepts only the recorded phase/error, the two exact reviewed old code hashes
with all other module/kernel hashes unchanged, the protected original inputs,
the verified three-record fixture archive, and the original replica ledger.
It rechecks mounted Secret/ConfigMap identities, source PVC/PV, maintenance
UID/resourceVersion/token, templates, zero replicas, controllers and Pod mounts
before allocating any final claim. The capture must be at most four hours old.
The process lock still excludes duplicate callers.

Continuation preserves the operation, fixture and maintenance token; it does
not reseed the fixture or restart applications. It performs the existing final
zero-message 15-stream/42-durable baseline backup/restore and stops at the same
off-node copy checkpoint. The previous capture hashes remain in the private
journal and the new reviewed capture binds subsequent resume/refresh commands.
Failures remain stopped with `VERIFIED` only after observed refencing, otherwise
`UNKNOWN`; there is no automatic unfence or generic blocked-operation retry.
Use only the newly reviewed hash-pinned launcher. This does not make historical
preservation or the separate ACL proof pass.

The observed source PV uses `spec.local.path`, not `spec.hostPath.path`.
The shared selector accepts exactly one of these forms, bound to the exact
claim/PV identities, canonical local-path directory and single `pmdebook`
hostname affinity. Final storage verification also preserves the selected kind.
An early failure now prints `KNOWN_NATS_ERROR` as a fixed guard label or
exception type; it never prints exception messages, command output or private
values. A failed four-hour eligibility guard still requires reviewed recovery,
and does not silently renew the captured operation.
