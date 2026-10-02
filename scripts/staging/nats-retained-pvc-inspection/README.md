# Original retained NATS PVC: guarded file inventory

This public stdlib Python helper inventories the original retained volume on
`pmdebook`. The operator runs a reviewed command manually as root. Agents may
upload public scripts as `pmd`; agents must not execute the root command.

The pinned source is PVC `voice-staging/voice-nats-jsdata`, PV
`pvc-52c42e20-c7e6-4182-b5d3-ecaa6e1f9855`, local path
`/var/lib/rancher/k3s/storage/pvc-52c42e20-c7e6-4182-b5d3-ecaa6e1f9855_voice-staging_voice-nats-jsdata`.
No override of source, node, broker, credential, or guard is accepted.

## Preconditions and limits

The operator must prevent new mounts/writers throughout the inspection window.
Read-only GETs and process checks are observations, not a Kubernetes lock.
If that window cannot be guaranteed, do not launch. Existing root-owned k3s
binary `/usr/local/bin/k3s` and config `/etc/rancher/k3s/k3s.yaml` supply existing
read access. The helper invokes only Kubernetes GET; it does not output or copy
the config. Config/binary/source ancestor trust and provenance guards must pass.

Any Pod referencing this PVC, any process FD/mapping into it, or mount namespace
bind/nested mount fails. Process races, permission errors, missing/malformed API
fields, file changes, symlinks, hardlinks for selected files, or special files
also fail. There is no automatic retry or weakened mode. Failure may leave a
root-only partial output directory; it does not publish completion metadata.

Mount/handle checks map the source and each namespace path into coordinates on
the same backing device. This includes a separate `/var/lib` filesystem, ancestor
binds exposing the source, and FD/mapping aliases such as `/data`. Only the
canonical backing mount is allowed; ambiguous/unknown coordinate mapping fails.

The walk opens the source read-only with `O_NOFOLLOW` and `O_NOATIME`; failure to
use these flags fails inspection. Only JetStream `meta.inf`, `meta.sum`, `o.dat`
and `*.blk` file bytes are hashed; credential/seed files are not opened. Reports
contain paths, sizes, inode/device, timestamps and SHA256, never file contents.
No source chmod/chown/copy, broker startup, mount, deletion, replay or ACK occurs.

Selectors use stream `meta.inf/meta.sum`, numeric `msgs/*.blk`, and consumer
`obs/<consumer>/{meta.inf,meta.sum,o.dat}`. The exact current proof runtime's
[NATS 2.12.12 file-store constants](https://github.com/nats-io/nats-server/blob/v2.12.12/server/filestore.go#L279-L302)
define this layout; it also exists in
[NATS 2.11.9](https://github.com/nats-io/nats-server/blob/v2.11.9/server/filestore.go#L275-L296).
The retained original volume's writer version remains unverified. Selecting
recognized files does not establish that writer version or certify completeness
of an unknown historical storage format.

**A complete file inventory is not a canonical consumer/message census, custody
verification, data-disposition decision, replay/idempotency evidence, or
preservation PASS.** Historical 566-consumer/eight-message claims remain separate.

## Reviewed launch and retrieval

Independently review `inspector.py`, its tests, and `root-launch.template.sh`.
Record the reviewed SHA256 of `inspector.py` and source commit. Upload those same
public bytes to `/home/pmd/voice-nats-preservation/20261002/inspector.py` over the
existing `voice-staging` pmd connection. Do not upload credentials or data.

Replace `REVIEWED_INSPECTOR_SHA256` in the reviewed launch template with that
exact hash. Paste the resulting command into the operator's **root shell on
pmdebook**. Never run an unverified pmd-owned launcher as root. The pasted
bootstrap clears the environment, uses isolated Python, copies the public source
to a fresh root-owned directory under `/root`, hashes the completed private copy,
and only executes that copy after a digest match. A modified upload fails closed.

Successful inspection prints one metadata path under
`/var/lib/voice-nats-preservation/original-pv-*/metadata.json`. The report is
`root:pmd` mode `0440`, its directory `root:pmd` mode `0750`; pmd cannot alter
either. Use the exact printed path with `scp voice-staging:<path> <local-path>`
as pmd after completion. No payloads or account/operator/user seeds are retrieved.
If the helper reports failure, provide only that failure status for review.

## Disposable Linux checks

Run with Python 3.12+ on Linux as root inside a disposable fixture container:

```sh
docker run --rm --network none --label voice.task=a1-pvc-inspector \
  --mount type=bind,source="$PWD/scripts/staging/nats-retained-pvc-inspection",target=/work,readonly \
  python:3.12-slim python -I -S /work/test_inspector.py
```

Fixtures are generated under container `/root`, contain synthetic bytes only,
and never reference a real volume/config/broker. Tests do not run `main()` against
staging. The host repository mount is read-only; the container has no published
ports or network. This command does not require a privileged container.
