# Fresh DEV on Kubernetes

This adapter replaces the explicitly disposable `voice-dev` application/data
environment. It does not physically erase old data. The four old `Retain` PVs,
their claimRefs and backing paths remain untouched. Fresh claims use the existing
`local-path` StorageClass, new namespace/PVC/PV UIDs, and the provisioner's unique
`pvc-<UID>_voice-dev_<claim>` directories. No host root access, hostPath cleanup,
StorageClass update, STAGING mutation, PV delete or claimRef clearing is used.

The selected DEV operator must hold the actual TTenrec DEV resource claim in
recovery, after a real supported handoff. The command records the supplied actor
and claim; it does not acquire, transfer or validate tracker custody itself.
Perform the supported resource check/checkpoint protocol around the execution
window. A reviewer or author has no implied shared-environment execution custody.

## Prepare before deleting the namespace

Use a reviewed adapter commit and the separately pinned product source. Prepare
a separate immutable product-source export; the package manifest is checked
against that export, while actual adapter/helper hashes are captured separately.
The reviewed adapter's bootstrap uses `--source-root` for canonical SQL, Go and
mail inputs; it does not pretend its deliberately changed bootstrap file is the
unchanged product-source blob.
Prepare
a complete package through `reset_package.render` and `write_package` in a new
caller-owned directory under a mode0700 parent. The package contains private
mode0600 `prepared.json`, `secrets.json`, `foundation.json`, `source-revision`
and `package-receipt.json`. Its source manifest, immutable images, DEV frontend
build arguments and all required Secret/public ConfigMap references must be
complete. Existing STAGING Secrets and protected old host archives are not inputs.

Resolve actual registry availability and required CI independently; a locally
built OCI archive or plausible digest is not a published image. Required tool
slots are `space-native-installer` (actual linux/amd64 digest),
`migrate/migrate:v4.18.1`, `flyway/flyway:11.3.0` and `node:24-alpine`. The source
Space installer and its native51 receipt remain mandatory.

Preserve the compatible existing DEV `voice-dev-signaling-tls` identity privately.
Canonical certificate validation checks all five DEV SANs, at least one day of
validity, private-key match and the public CA chain. Include its unchanged data
with fresh DEV ownership metadata in the package. Never emit keys to terminals,
Git or public receipts. Restore checks an already present renewed identity and
preserves it instead of overwriting it with the older package certificate.

Independent review and a private fake-API/filesystem rehearsal precede shared
execution. Record a measured/derived conservative budget from the first writer
stop through early web/TLS/challenge restoration, including command timeouts,
image availability and recovery margin. The CLI rejects less than thirty
minutes and requires an explanation; this lower bound is not an estimate of
the live operation. Do not substitute an optimistic fixed five-minute budget.

On the actual Linux DEV host, with its authorized Kubernetes context:

```text
python3 scripts/dev/reset_bootstrap.py plan --work-dir <private-package> \
  --source-root <immutable-product-source-export> \
  --actor <actual-operator> --resource-claim <actual-claim> \
  --namespace-uid <observed-old-DEV-UID> --staging-uid <observed-STAGING-UID> \
  --budget-seconds <reviewed-budget-with-margin> \
  --budget-evidence '<measured/derived budget and rehearsal receipt reference>'
```

`plan` performs read-only cluster observations and strict client manifest
validation, verifies old retained bindings, existing local-path class/capacity,
the current timer/process window and compatible DEV TLS, and writes a private
operation plan. The original package files are immutable through retries.

The concrete daily renewal timer is checked fresh, including pending systemd
jobs, inactive service/PID0, last trigger today, enabled active/waiting timer,
host boot/clock continuity and the daily base midnight MSK boundary. Randomized
00:55 scheduling is not a guaranteed earliest trigger. Unknown unrelated root
journals are not a universal gate. A real active/queued DEV executor is a gate.

## Run and recover forward

```text
python3 scripts/dev/reset_bootstrap.py run --work-dir <same-private-package> \
  --source-root <same-immutable-product-source-export> \
  --actor <same-actual-operator> --resource-claim <same-actual-claim>
```

One OS file lock owns the package. Before mutation, the runtime copy is fully
materialized and bound to the original package. Writer patches test UID and RV;
the adapter suspends Jobs/CronJobs and scales Deployments/StatefulSets to zero,
then observes actual active Pod exit. An autoscaler or unknown/replaced writer
fails closed and needs its real existing control route. No force Pod deletion is
used. The old namespace DELETE carries exact UID/RV preconditions. Deletion must
actually complete; unexplained finalizers are never stripped.

Bootstrap uses the private runtime directory and fresh dynamic claims. It
creates a namespace exclusively, records its new UID and operation annotation,
and binds all subsequent commands to that exact operation. It restores
the web image, certificate and canonical HTTP challenge/TLS routes before slow
infra/migration work. Canonical NATS bootstrap ordering, native Space installer,
GIS prerequisites and User/Space cycle handling remain in use. All applied tool
images are pinned. Retained PV identity/spec and fresh claim bindings are checked.

A failed/interrupted command retains the private journal, runtime and logs.
The actual Linux execution host supervises its own bootstrap session, observes
the window with a two-second polling interval and bounded observer waits, and terminates its
own process group on unsafe observations or the fixed original deadline. The
reviewed budget reserves fifteen seconds for observer and TERM/KILL cleanup.
Each command also checks the fresh namespace UID/operation and quiet window.
Normal wrapper exit cannot leave a descendant writing in the background.
Accepted Kubernetes Jobs, rollouts and remote exec work are not cancelled by
client termination. An interruption records their exact controller/Job UID and
specification snapshot plus private command start/return boundaries. Every
interrupted bootstrap stays on HOLD, even if the snapshot is unchanged: remote
exec outcomes can remain unknown after local termination. Actual operator
recovery must resolve that concrete phase before any further submissions; the
adapter does not automatically replay bootstrap. It never grants a
safe resource release or a new deadline merely because a client was killed.
The durable `bootstrap: started` journal alone also requires this HOLD: abrupt
supervisor or host loss can prevent any interruption receipt from being written.
Pre-bootstrap continuation uses the same package, actor and live claim. Resume
re-observes the old namespace/controller identities; it never deletes a newly
recreated namespace by name. An already absent old namespace can complete its
recorded pending delete without replay. Separately reconciled forward bootstrap
must preserve its new namespace UID/operation. A changed input, foreign UID or
unresolved outcome stays explicit. Do not delete/reset operation receipts or
automatically roll back to old PVC data.

Success means `READY_FOR_APPLICATION_SMOKE`, not acceptance. The operator must
verify stable current images, all required migrations/bootstrap receipts, actual
public DEV TLS/challenge routing and the real requested application flow, then
record deployment/test evidence in TTenrec. STAGING namespace/PVC/PV identities
and essential workload storage bindings are captured and checked unchanged;
STAGING datastore contents and Secrets are never read by this adapter.
