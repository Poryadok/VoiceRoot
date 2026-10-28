# DEV media backup GitHub custody

`build_bundle(root, key, expected_binding, manifest)` verifies the private
encrypted set and writes a new 0600 `media-backup.tar` under the private 0700
root. The tar contains exactly canonical regular USTAR headers for
`manifest.json`, `file_db.enc`, `messaging_db.enc`, `roles.enc` and
`story_db.enc`, in that order. It contains no key or plaintext database data.
Each encrypted member is limited to 8 GiB minus one byte, the bundle to 64 GiB;
capacity is checked before creating the bundle. The key is a separate 32-byte
custody input and must never be placed in the artifact, output or logs. The
binding itself must contain operation metadata only, not secrets or rows.
`verify_bundle(path, key, expected_binding)` rechecks the complete authenticated
bundle on a single private no-symlink file descriptor and returns only byte
count and SHA-256. The uploader must copy the verified ciphertext into private
runner scratch and recheck the copied bytes against that result before upload.

The consumer must durably bind the actual operation and snapshot, upload
attempt, workflow run/head, fresh challenge, creation time and exact bundle
size/SHA-256. In the fixed repository `Poryadok/VoiceRoot`, the intended
GitHub Actions artifact name is `voice-media-backup-<operation_id>-<challenge>`,
with exactly one member `media-backup.tar` and compression level zero. The
operation format is `media-c1-<32 lowercase hex>`, snapshot is
`capture-<32 lowercase hex>`, and challenge is 32 lowercase hex. Upload is
owned by the operation workflow, not this library. Unknown upload outcomes
require journal reconciliation rather than automatic replay.

`readback_extract(token, artifact_binding, artifact_id, bundle, key,
expected_binding, destination)` fetches fixed-repository GitHub artifact metadata
and completely downloads the ciphertext ZIP using the staging custody transport
and its HTTPS/redirect restrictions. It verifies run/head/name/challenge,
timestamps, expiry, size, ZIP structure and full bundle SHA; verifies every
authenticated manifest/member; then copies only the five verified members into
an empty private 0700 destination using exclusive no-symlink creation. It returns
the custody receipt and manifest only after destination verification. The
operation/snapshot must match the separately protected expected manifest binding.
Receipt fields and hashes describe encrypted custody, not a PostgreSQL restore.
The caller must preserve separately custodied key access for an actual isolated
restore and migration, and retain the fence on any unknown or partial outcome.
The GitHub artifact retention window is not a substitute for the operator's
separate disaster-recovery retention/key policy.

The code reuses `scripts/staging/nats-rollout-preservation/github_custody.py`
for bounded GitHub transport and must be captured and reviewed with that
dependency. Local tests replace that transport with synthetic bytes. The Linux
integration fixture additionally restores populated PostgreSQL databases after
synthetic readback and applies migration SQL directly; neither test establishes
a real GitHub upload, independent custody, production migration-runner result,
MinIO/NATS preservation or permission to deploy.
