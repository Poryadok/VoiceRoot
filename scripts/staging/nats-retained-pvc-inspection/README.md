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
also fail. A numeric process that vanishes during a scan may cause a complete
scan restart only after fresh `/proc/<pid>` directory lookup verifies absence;
at most three complete passes are attempted. Live processes with missing or
denied inner references fail immediately. A detected source handle, mapping or
mount veto is preserved even if that process exits during error cleanup. There
is no weakened mode. Failure may leave a
root-only partial output directory; it does not publish completion metadata.

Mount/handle checks map the source and each namespace path into coordinates on
the same backing device. This includes a separate `/var/lib` filesystem, ancestor
binds exposing the source, and FD/mapping aliases such as `/data`. Only the
canonical backing mount is allowed; ambiguous/unknown coordinate mapping fails.
Recognized Linux `nsfs` network namespace object roots (`net:[inode]`, virtual
device major 0) are retained as object records, not filesystem coordinates.
Their mount targets still reject mounting at/inside the source. Malformed or
unknown non-filesystem roots fail; filesystem alias checks remain unchanged.

An exactly empty process mount table may be chroot filtering. It can use the
inspector's complete runtime table only when the target shares the inspector's
mount namespace, PID 1 shares its namespace and root, and the inspector root is
the runtime `/` with a matching `/` mount record. The helper pins process,
namespace and root descriptors and checks fresh process references, start times
and all three mount-table byte snapshots before and after the FD/maps scan.
Missing, denied, different or changing evidence fails closed. Every FD/mapping
and source mount/alias check still runs, using the kernel's caller-root paths.

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
Failure JSON contains only pinned public guard/class/function labels and bounded
numeric PID/errno and pinned process-operation labels when available; exception
text, paths and tracebacks stay private.

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

## Exact retained stored-layout observations

`decoder.py` is a separate bounded diagnostic for the successful original-PV
inventory at `/var/lib/voice-nats-preservation/original-pv-sckcykyd/metadata.json`,
SHA256 `12d2b844a3af68362abe3614c4119eb677fb243eb0c2a982137bd9dfc910c2c6`.
The protected report path/hash and existing inspector SHA are compiled into
reviewed code; a pmd-owned manifest cannot authorize source reads. The decoder
requires root:pmd0440 report protection, exact PV/PVC identities,165 selected
files and maximum2MiB selected bytes per pass (two byte passes). Its metadata-only
census caps4096 entries and12 directory levels. FD-relative NOFOLLOW/NOATIME
reads match inode/device/size/times/SHA and fresh path identity. Existing node,
Kubernetes, source, mount and handle guards surround the bounded read passes.

Output contains numeric/bool/literal enums and subject/filter counts, never raw
names, subjects, descriptions, custom metadata or payloads. Files use inventory
row index and path SHA. Consumer `o.dat` versions1/2 require bounded complete
varint validation; empty/unrecognized state is explicit. Unknown JSON/enum
layouts are unsupported. Native `meta.sum` is HighwayHash64: recognizing its
hexadecimal syntax is `NOT_VERIFIED_HIGHWAYHASH64`, distinct from inventory SHA256.
Block output is `HEADER_IDENTIFICATION_ONLY`, with no interpreted sequences or
payloads. It establishes no plaintext, writer version, message count, native
checksum, deletion map or index state. Compressed/encrypted/unknown input is
unsupported or compatibility-only, never certified.

Format references are NATS [v2.12.12](https://github.com/nats-io/nats-server/blob/v2.12.12/server/filestore.go)
and [v2.11.9](https://github.com/nats-io/nats-server/blob/v2.11.9/server/filestore.go).
Layout observations do not establish the historical566 consumers/eight messages,
ownership/disposition, archive, replay, idempotency/fences or preservation PASS.

Independently review decoder, tests and `decoder-launch.template.sh`. Replace
`REVIEWED_DECODER_SHA256`, upload only public decoder and instantiated launcher
as pmd, and have the operator use the reviewed captured-memory shell shape with
the exact launcher SHA. The bootstrap pins both public code captures into fresh
root0700 private storage before execution. Output is root:pmd0440
`observations.json` within a root:pmd0750 directory. No broker, key/seed read,
decryption, mount, source mutation or automatic root run occurs.

Run `/work/test_decoder.py` in the same pinned disposable root/networkless Python
fixture container. The hosted fixture workflow runs both suites.
