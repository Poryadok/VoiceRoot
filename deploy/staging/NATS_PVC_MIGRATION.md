# Staging NATS JetStream PVC migration

`infra.yaml` keeps the current `voice-nats` Deployment and its pod-local
`emptyDir` untouched. It adds `voice-nats-pvc-candidate` plus the dedicated
`voice-nats-pvc-candidate` Service and PVC. The existing `voice-nats` Service
continues to select only the source until the separate `cutover` command passes
the accepted-restore gate. Applying infra does not redirect clients or restart
the source. NATS bootstrap jobs are deferred until acceptance.

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

   This first queries the protected source context and writes a sorted
   `source-census.tsv` containing every unique stream/consumer identity and a
   digest of each consumer's config/state. It then creates one `nats stream
   backup` archive per stream and records archive hashes. The census SHA-256 is
   stored in evidence. Copy the bundle to durable protected storage and verify
   all hashes before proceeding.
4. Complete `consumer-decisions.tsv` with exactly 566 rows and six columns:
   `stream<TAB>consumer<TAB>classification<TAB>source-state-sha256<TAB>RESTORE|DROP<TAB>review-ref`.
   The stream, consumer, and state digest must exactly match the generated
   source census. A `DROP` is a destructive decision and needs its review ref.
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

   The guard re-queries the protected source and compares its complete sorted
   census/hash with the archived census; fabricated decision rows cannot pass.
   It also validates archive hashes, exact identity/state decisions, and the
   512 restore cap. It fails until source contexts and every consumer's
   disposition are present.

## Fenced cutover and restore

After the bundle passes `guard-nats-pvc-migration.sh --prepare`, apply staging
infra with the reviewed storage values and evidence path. This only creates the
candidate PVC/Deployment/Service; the old source remains intact:

```sh
VOICE_NATS_STORAGE_CLASS='<confirmed-class>' \
VOICE_NATS_STORAGE_SIZE='<confirmed-capacity>' \
VOICE_NATS_MIGRATION_EVIDENCE='<protected-directory>' \
NATS_SOURCE_CONTEXT='<protected-source-context>' \
bash scripts/staging/apply-infra.sh
```

Keep all application publishers/consumers fenced. Verify the candidate's TLS
identity is valid for its candidate endpoint (the existing `voice-nats` cert is
not evidence for the candidate DNS name). Point the NATS CLI at the candidate
Service using its protected context, then restore:

```sh
NATS_MIGRATION_EVIDENCE='<protected-directory>' \
NATS_SOURCE_CONTEXT='<protected-source-context>' \
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

The restore command checks target stream final sequences against the backed-up
source sequence records. Acceptance also freshly inventories the candidate and
requires its exact consumer identity/state digest set to match only the
explicitly approved `RESTORE` decisions. The operator must provide the
independent retained-message hashes and replay/idempotency evidence; the archive
SHA-256 alone does not prove restored message contents. Candidate TLS identity
confirmation is mandatory. Validate with:

`message-hashes.tsv` must contain the exact eight retained source messages as
`stream<TAB>sequence<TAB>source-sha256<TAB>candidate-sha256<TAB>owner-proof-ref`;
all records must be unique and each source/target digest must match. Record
`MESSAGE_HASHES_VERIFIED=true` and `CANDIDATE_TLS_IDENTITY_CONFIRMED=true` only
after independent service-owner verification. The acceptance guard validates
the 8-row hash set and candidate's live sequence, stream configuration, and
consumer census before it can permit selector cutover.

```sh
NATS_MIGRATION_EVIDENCE='<protected-directory>' \
NATS_SOURCE_CONTEXT='<protected-source-context>' \
NATS_TARGET_CONTEXT='<protected-candidate-context>' \
bash scripts/staging/migrate-nats-pvc.sh acceptance
```

Only after validation set `RESTORE_ACCEPTED=true` in `evidence.env`, then run
`migrate-nats-pvc.sh cutover` with explicit approval and the same source/target
contexts. The command validates all evidence before patching only the
`voice-nats` Service selector to `app: voice-nats-pvc-candidate`. Keep the
application fence through smoke tests and acceptance. Retain source Deployment,
its original data until cutover, and all archives until final acceptance.
Set `CUTOVER_ACCEPTED=true` only after the post-cutover smoke/replay gate passes
and the application fence is released; before then, selector rollback to the
retained source remains available.

## Rollback

Before releasing the application fence, rollback patches the stable
`voice-nats` Service selector back to the still-retained source pod. It does not
roll out or recreate the `emptyDir`, and it does not delete the candidate PVC or
archive. The command checks contexts, storage evidence, and approval before any
Kubernetes mutation. If writes escaped the fence or cutover was already
accepted, it fails closed and requires a separate reconciliation plan.

```sh
NATS_MIGRATION_EVIDENCE='<protected-directory>' \
NATS_SOURCE_CONTEXT='<protected-source-context>' \
NATS_TARGET_CONTEXT='<protected-original-staging-context>' \
VOICE_NATS_STORAGE_CLASS='<confirmed-class>' \
VOICE_NATS_STORAGE_SIZE='<confirmed-capacity>' \
NATS_TARGET_QUIESCED=true \
NATS_ROLLBACK_APPROVED=true \
bash scripts/staging/migrate-nats-pvc.sh rollback
```

The rollback command requires explicit operator approval and exact source
context equality. Keep the fence until the retained source passes the same
sequence/count/hash checks. If any writes escaped the fence, stop: the snapshot
no longer represents the complete source of truth and a new reconciliation
plan is required.

## Current blocker

Cutover is intentionally blocked now: storage-class/capacity evidence, the
source-consistent backup/census, all 566 classified decisions, candidate TLS
identity, and independent retained-message hash/replay evidence are not present
in this repository. Apply may stage only the isolated candidate; neither apply
nor this PR performs a live staging mutation. This migration does not alter
central NATS auth/JWT/ACL bootstrap behavior owned by #473.
