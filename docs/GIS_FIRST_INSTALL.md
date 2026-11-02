# Protected GIS first install

The Game Integration Service (GIS) first install is a one-time, protected
22-to-23 workload transition. It is a separate operation from the normal
caller-wiring rollout. The first operation creates only the source-approved
`voice-gameintegration` Service and Deployment in `voice-staging`; the second,
freshly authorized `full` or `app-only` rollout applies the GIS Role trust and
Notification/Realtime caller wiring through the ordinary snapshot and rollback
path.

The first operation must use the existing source-CI, stack-lock, NATS custody,
and protected rollout authorization. It is unavailable to `images-only` mode
and cannot create arbitrary resources. Before planning and immediately before
creation, both exact GIS identities must be absent. Only an authoritative API
NotFound qualifies; partial presence, access errors, timeouts, and create races
block the operation.

The operation journals its ID and both intended spec digests before mutation.
It uses Kubernetes create-only calls and creates the Deployment with zero
replicas. The regular paused apply retains that replica count and binds the
operation marker to the exact Service/Deployment pair. Only after the standard
apply and preserved workload restart does the protected finisher revalidate
the pair's UIDs, marker, and semantic spec, journal the observed resource
versions, and guardedly start that same Deployment. A lost API response can be
resumed only under the same operation and only when the live object has the
exact operation marker and intended spec. Another operation cannot adopt it.

If this operation fails after creating the GIS pair, its guarded cleanup may
delete only the same operation's unchanged objects. The database role and
migration prerequisites remain in place because they are additive and
idempotent; cleanup never downgrades the schema or touches stored data. If
ownership or semantic content has changed, cleanup stops and leaves the live
objects for same-operation recovery.

Rollback is limited to the Service and Deployment created by that operation.
It verifies the recorded UID, operation marker, semantic spec, and current
resourceVersion, then deletes the Deployment before the Service with UID and
resourceVersion preconditions. Drift or a delete conflict stops cleanup and
preserves the journal. The operation never deletes a namespace or changes
database resources, NATS data, PVCs, or PVs.

For the later ordinary rollout, a successful source/CI gate is not a recovery
plan. A read-only failure before the FENCE checkpoint can be retired only by
the runbook's verified read-only closure. A failed proof after fencing reports
BLOCKED and attempts ownership-checked refencing; only a verified checkpoint
proves the fence remains closed. Otherwise treat fence status and the TT claim
as uncertain. Inspect the exact operation checkpoint,
installed helper binding, resource identities and TT journal using the
[preservation runbook](../scripts/staging/nats-rollout-preservation/README.md)
before choosing a separately qualified recovery. Preserve the encrypted backup
and its recovery key and reconcile submitted effects; do not clear an
uncertain TT claim or repeat the rollout merely because a timeout elapsed.
If publication fails after the durable PASS checkpoint, the root executor may
mark its TT journal uncertain. The saved PASS is retained, but a direct handler
replay is not authority to bypass that journal; use separately qualified
operator reconciliation before attempting publication again.
The historical 22-workload operator closures do not qualify a 23-workload GIS
baseline. The existing generic FULL/app-only reverse rollout is unsupported,
and the direct root `--rollback` command is rejected by the resource-fenced
bridge gate. This procedure does not claim an automatic whole-baseline recovery.

## Database prerequisite

Before either GIS workload object is created, the protected root operation must
provision the restricted GIS database role, run the source-pinned migrations,
and run the source-pinned postcondition verifier. The root collector writes a
non-secret prerequisite receipt into the same private operation journal only
after create-only Job execution, exact UID and image checks, fresh API reads,
and schema/role/grant predicates succeed. A Job status, log line, detached JSON,
or caller-supplied success flag is not evidence. The receipt binds the exact
source SHA and CI attempt, target namespace UID, PostgreSQL Service and ready
endpoint identities, Secret metadata, migration inputs, immutable Job and Pod
identities, postconditions, and existing preservation/custody evidence. It
contains no SQL bodies, DSNs, credentials, or credential hashes.

The receipt is private to the operation that produced it. Resume re-reads the
recorded live identities and verifier results; a new source or CI attempt,
namespace or database endpoint change, Secret rotation, replaced Job, failed
postcondition, or unknown create outcome blocks. The receipt is consumed once
as the Step 2 transition input and never replaces the normal live workload,
storage, or NATS checks.

The GIS database role and ordered migration inputs, pinned Job templates, and
fixed verification SQL are defined by the source-owned GIS deployment change.
The protected rollout root owns Job execution, API evidence collection, and
receipt custody. Neither layer may publish credential values or treat a
hand-authored artifact as authority.

## Disposable DEV first install

The same source-pinned database prerequisite and create-only paused pair may be
used for the fixed `voice-dev` namespace by the DEV installer. It does not
change the STAGING admission contract above: STAGING still requires its source
CI attempt, stack lock, preservation custody, and protected rollout checks.
DEV instead requires a clean committed source SHA and an immutable
Game Integration image reference bound to a successful first-party GitHub
Actions master-push image job. The installer queries the run, workflow, job,
and private job log from GitHub, requires the exact successful
`Build and push gameintegration image` step, and checks its `Digest` output
against the published OCI index and linux/amd64 config. It compares the builder commit's exact
`src/backend` Git tree with the operator checkout. A clean docs/scripts-only
change can therefore reuse an existing trusted image without fabricating a CI
receipt or requiring a new build.
The DEV prerequisite receipt has its own schema and binds that build receipt,
the namespace and PostgreSQL identities, exact prerequisite ConfigMaps and
completed Jobs, and the complete migration verifier result. It is not CI or
STAGING evidence and cannot be replayed into `voice-staging`.

The DEV secret preparation validates the supplied Game Integration JWKS HTTPS
leaf against `voice-media-ca`'s constrained outbound `jwks-ca.crt` bundle,
including hostname, server-auth usage, validity, and key match, before
generating application material or writing a Secret. The inbound `ca.crt` key
is preserved and is not used for JWKS trust. The installer does not create or
replace CA material, copy STAGING credentials, or write secret bytes to command
arguments, receipts, or logs. An invalid or absent leaf blocks before writes.
After preparation, the protected prerequisite operation
provisions the restricted database role, applies the ordered migration, and
checks the verifier before the zero-replica Service/Deployment pair is created.
The installer keeps the pair paused through migration and verification, then
starts that exact Deployment under UID/resourceVersion custody and waits for
readiness. Caller wiring and NATS/music activation are separate operations.

The executable entry point is `scripts/dev/gis_dev_first_install.py`. Run it
from a clean, committed checkout with GitHub CLI authenticated for read access,
Docker, and `kubectl` pointed at the disposable cluster, and the existing
`voice-dev/voice-gameintegration-jwks-tls` and `voice-dev/voice-media-ca`
Secrets already installed. The default builder evidence is the successful M12
master-push run and GameIntegration image job recorded in the stage image
inventory; the run/job IDs are locators and are revalidated against GitHub.
Use the recorded immutable image digest:

```sh
python3 scripts/dev/gis_dev_first_install.py \
  --image-ref 'ghcr.io/poryadok/voiceroot/gameintegration@sha256:482eb267824cffc4cebcae958e4ec7126ffdeaaea632f40c26bb1c6baf71ea96' \
  --state "$XDG_STATE_HOME/voice/dev-gis-first-install.json" --apply
```

An operator with GHCR package-write access can instead build and install in one
process. The fixed Buildx command publishes, pulls, and inspects the exact
digest, then the installer verifies it against the clean source context before
continuing. The private operation journal records receipt metadata but no
credential material. A resumed invocation re-pulls and revalidates that image
receipt and continues with the same image and operation:

```sh
python3 scripts/dev/gis_dev_first_install.py \
  --build-and-publish --image-tag "dev-$(git rev-parse --short=12 HEAD)" \
  --state "$XDG_STATE_HOME/voice/dev-gis-first-install.json" --apply
```

For another published image, pass its successful master-push run and
`staging-images-push (gameintegration)` job IDs using `--builder-run` and
`--builder-job`. Those IDs only locate evidence: the tool checks the completed
workflow, job and Buildx step through the GitHub API, downloads the run's
`.dockerbuild` artifact, and verifies its BuildKit source context, SLSA
provenance, producer-step time window, linux/amd64 output descriptor and exact
registry digest. It also compares the builder commit's `src/backend` Git tree
to the operator checkout; a digest mentioned in unrelated job output cannot
authorize the image. The installer pins Kubernetes requests to `voice-dev`, checks
the OCI index/child/config against the registry digest and pulled image, and
verifies the source backend tree at the CI builder commit. It also verifies the
stable TLS Secret and runs a read-only `pg_database` existence query before any
Secret write. A missing database, TLS input, changed namespace/PostgreSQL
identity, source tree, Secret version, or same-operation resource ownership
blocks the run. Secret bytes stay in process memory and Kubernetes request
bodies; the private journal stores only source, UID/resourceVersion, job, and
readiness evidence. The installer applies only the three GIS prerequisite
Jobs and source-approved GIS Service/Deployment pair. It verifies migration
version 33 and the authority singleton, triggers, and functions, guardedly starts
that exact Deployment, and checks a ready
EndpointSlice. On a later failure it removes only the unchanged pair owned by
the journaled operation; additive database prerequisites are retained.
Retrying with the same state path resumes the operation, while changed source
or image is rejected.
Historical version 31 and 32 receipts remain valid only with their original
source-pinned verifier and matching migration set.
