# Staging NATS JetStream PVC migration

`infra.yaml` now describes `voice-nats-jsdata` as a PVC. Applying that manifest
replaces the current pod-local `/data` volume, so `scripts/staging/apply-infra.sh`
refuses to reach the apply/rollout step until the pre-cutover evidence bundle
passes `guard-nats-pvc-migration.sh --prepare`. Do not set readiness values from
assumptions. No staging class, capacity, complete consumer inventory, or
consumer disposition evidence is present in this repository.

The currently known inventory is 7 streams, 566 consumers, 8 retained messages,
and an application cap of 512 consumers. A snapshot restores consumer
configuration and state as well as stream data. The operator must classify all
566 consumers and mark each `RESTORE` or explicitly reviewed `DROP`; the
selected restore count must be no more than 512. A DROP needs a review reference.
Nothing in this procedure silently filters or deletes a consumer.

## Read-only storage preflight

Run this against the staging context and select the class based on cluster-owner
evidence:

```sh
VOICE_K8S_NAMESPACE=voice-staging \
VOICE_NATS_STORAGE_CLASS='<confirmed-class>' \
bash scripts/staging/preflight-nats-pvc.sh
```

This lists the selected class/provisioner, checks PVC create authorization, and
reports CSIStorageCapacity when the API exposes it. It creates no PVC and does
not prove that a 10Gi claim can bind. The production 10Gi claim is a reference,
not staging sizing evidence. Record the actual approved class and requested
capacity in the migration evidence after the storage owner confirms it.

## Snapshot and decision bundle

1. Arrange a maintenance fence: stop or pause all source writers and consumers,
   verify no further writes/acks, and record the fence time. Do not start the
   backup until this is confirmed; keep the fence through acceptance or rollback.
2. Record the seven exact stream names in `streams.txt`, one per line, and each
   stream's last sequence in a tab-separated file `name<TAB>last_sequence`.
   Compare this list with live `nats stream ls` / `nats stream info` output.
3. Create an empty, access-controlled, off-cluster evidence directory. Use a
   preconfigured NATS CLI context with access to the source (do not put creds or
   tokens in this directory), run:

   ```sh
   NATS_MIGRATION_EVIDENCE='<protected-directory>' \
   NATS_SOURCE_CONTEXT='<protected-nats-cli-context>' \
   NATS_SOURCE_QUIESCED=true \
   NATS_STREAMS_FILE='<path>/streams.txt' \
   NATS_LAST_SEQUENCE_FILE='<path>/last-sequences.tsv' \
   bash scripts/staging/migrate-nats-pvc.sh backup
   ```

   This creates one `nats stream backup` archive per stream and writes an
   archive SHA-256 manifest. NATS stream backups carry messages, stream
   configuration, consumer configuration, and consumer state. Copy the bundle
   to durable protected storage before any pod rollout; verify the copy hashes.
4. Complete `consumer-decisions.tsv` with exactly 566 tab-separated rows:
   `stream<TAB>consumer<TAB>classification<TAB>RESTORE|DROP<TAB>review-ref`.
   Classify each source consumer from live server data and its owning service.
   A `DROP` is a destructive decision and requires the recorded review reference.
5. The backup command creates `evidence.env` with observed source counts and
   the source context name. Complete it with the reviewed target context, storage
   class/size and readiness fields below (plain `key=value`, no secrets):

   ```text
   SOURCE_STREAM_COUNT=7
   SOURCE_CONSUMER_COUNT=566
   RETAINED_MESSAGE_COUNT=8
   APP_CONSUMER_CAP=512
   SOURCE_CONTEXT=<source-context-name>
   TARGET_CONTEXT=<new-target-context-name>
   STORAGE_CLASS=<confirmed-class>
   STORAGE_SIZE=<confirmed-capacity>
   STORAGE_CLASS_CONFIRMED=true
   CAPACITY_CONFIRMED=true
   QUIESCE_CONFIRMED=true
   BACKUP_VERIFIED=true
   SOURCE_BACKUP_RETAINED=true
   ```

   The guard validates archive presence/hashes, exact stream/consumer counts,
   explicit decisions, and the 512 restore cap. It fails until every consumer
   has a disposition and storage evidence is recorded.

## Fenced cutover and restore

After the bundle passes `guard-nats-pvc-migration.sh --prepare`, apply staging
infra with the reviewed storage values and the evidence path:

```sh
VOICE_NATS_STORAGE_CLASS='<confirmed-class>' \
VOICE_NATS_STORAGE_SIZE='<confirmed-capacity>' \
VOICE_NATS_MIGRATION_EVIDENCE='<protected-directory>' \
bash scripts/staging/apply-infra.sh
```

This creates the PVC and rolls the NATS Deployment onto it. Keep all application
publishers/consumers fenced. Point the NATS CLI at the new service using its
normal protected context, then restore:

```sh
NATS_MIGRATION_EVIDENCE='<protected-directory>' \
NATS_TARGET_CONTEXT='<protected-new-staging-context>' \
NATS_TARGET_QUIESCED=true \
bash scripts/staging/migrate-nats-pvc.sh restore
```

Restore recreates snapshot consumers, then issues `nats consumer rm` only for
rows explicitly marked `DROP`. Before releasing the fence, fill these additional
reviewed values in `evidence.env`:

```text
RESTORE_VALIDATED=true
SEQUENCE_HASHES_VERIFIED=true
REPLAY_IDEMPOTENCY_APPROVED=true
CUTOVER_FENCE_APPROVED=true
```

The operator must record stream count, consumer count, each stream's final
sequence and an agreed message-hash comparison, and replay/idempotency results.
This repository has no canonical stream payload hashing/replay checker; the
service owners must supply and review that evidence rather than infer it from
archive checksums. Validate with:

```sh
NATS_MIGRATION_EVIDENCE='<protected-directory>' \
bash scripts/staging/migrate-nats-pvc.sh acceptance
```

Only after acceptance may the application fence be released. Retain the complete
source bundle until then.

## Rollback

Before releasing the application fence, rollback uses the retained source
snapshot. The Deployment's prior ReplicaSet uses the old `emptyDir` pod
template; `rollout undo` starts a fresh emptyDir, so restore the snapshot after
the old pod is ready. Do not delete the PVC or source bundle.

```sh
NATS_MIGRATION_EVIDENCE='<protected-directory>' \
NATS_TARGET_CONTEXT='<protected-original-staging-context>' \
NATS_TARGET_QUIESCED=true \
NATS_ROLLBACK_APPROVED=true \
bash scripts/staging/migrate-nats-pvc.sh rollback
```

The rollback command requires an explicit operator approval flag, undoes the
Deployment rollout, waits for readiness, and restores the same stream snapshots
and reviewed consumer dispositions. Keep the fence until the restored source
passes the same sequence/count/hash checks. If any writes escaped the fence,
stop: the snapshot no longer represents the complete source of truth and a new
reconciliation plan is required.

## Current blocker

Cutover is intentionally blocked now: storage-class/capacity evidence and the
566-row classified consumer decision file do not exist. The minimum prerequisite
is the read-only storage preflight result plus a source-consistent, off-cluster
NATS backup and complete reviewed consumer inventory. Do not apply the PVC
manifest or restart NATS until those artifacts pass the gate. This migration
does not alter central NATS auth/JWT/ACL bootstrap behavior owned by #473.
