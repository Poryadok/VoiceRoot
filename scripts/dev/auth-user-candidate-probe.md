# Auth/User candidate node and controller probe

This bounded DEV tool qualifies the published f194 Auth/User artifacts on actual
Kubernetes nodes and one credential-free Job deadline. It does not start either
application, connect to PostgreSQL, change a Deployment/StatefulSet, create a
Secret, select recovery, record deployment/test acceptance, or release TTenrec.
Existing-data adoption remains `auth_user_adoption.py` and its separate gates.

The only candidate is source `f1946355a084bed4c62a602ec1dc04e2d57e52ee`:

- Auth: `ghcr.io/poryadok/voiceroot/dev-auth@sha256:67de6a724618f1d7a98a95103690d315f5b0fa069071b384c98b93c712a87885`.
- User: `ghcr.io/poryadok/voiceroot/dev-user@sha256:42b84cfe7766bdb702ac73f50f21dc92bd60456220d7e04d6e27c0f4bfc14a61`.

No image, shell command, kubeconfig, namespace or endpoint override is exposed.
Kubernetes access uses the existing ordinary Linux controller identity,
`/home/pmd/.kube/config`, context `default`, namespace `voice-dev`, and exact
namespace UID `15950fc8-0392-4053-9156-e95e78b596f9`. STAGING and production
are outside this route. No node/containerd/root escalation is involved.

## Source and window admission

Use a reviewed immutable source package containing this module plus its sibling
`auth_user_adoption.py` and `auth_user_adoption_runtime.py`. Preserve the source
commit/archive receipt and verify the exact three-file fingerprint on Linux.
Install into an operator-owned private directory; do not execute an unrelated
checkout or replace files during an operation. The source fingerprint covers
the executed helpers as well as the probe. A different source needs a new actual
review and operation, not an edited approval field in an old journal.

The actor is Dimas-16. The actual adoption task must be owned/assigned to that
actor in `dev_ready_for_deploy` or `dev_deploying`, without pending impact review.
Use the supported TTenrec resource protocol to reconcile and claim DEV for that
task and the exact candidate. Do not hold the resource while waiting on builds,
source CI or review. The tool requires the actual held `prepare` phase and
fresh check API; it never claims, transfers, unlocks another owner, or releases
the resource itself.

Prepare a public-metadata binding file from the actual source review and current
resource receipt. Placeholder values below are intentionally not executable:

```json
{
  "protocol": "voice-auth-user-node-probe-v1",
  "candidate_source": "f1946355a084bed4c62a602ec1dc04e2d57e52ee",
  "namespace_uid": "15950fc8-0392-4053-9156-e95e78b596f9",
  "probe_source": "ACTUAL_REVIEWED_PROBE_COMMIT",
  "review": {
    "reviewer": "Dimas-8",
    "source_sha": "ACTUAL_REVIEWED_PROBE_COMMIT",
    "fingerprint": "ACTUAL_REVIEWED_LINUX_FINGERPRINT",
    "evidence": "ACTUAL_SOURCE_REVIEW_RECEIPT_URL"
  },
  "resource": {
    "claim": "ACTUAL_CLAIM",
    "generation": 0,
    "window": "ACTUAL_WINDOW",
    "candidate_sha256": "CANONICAL_SHA256_OF_ACTUAL_TYPED_CANDIDATE"
  }
}
```

The plan review alone is not a frozen executable-source review. Supply the actual
independent reviewer result. The binding is evidence custody, not an authenticated
identity or a replacement for live tracker/resource gates.

`candidate_sha256` binds the complete actual TTenrec `ResourceVersion` object
(including its known state, immutable material and evidence). Compute it with
the existing `auth_user_adoption_runtime.digest` helper: SHA-256 of UTF-8
`json.dumps(card['candidate'], sort_keys=True, separators=(',', ':'))`, with
Python's default ASCII escaping. It is not a candidate name/version string and
does not replace the server's typed candidate. The retained test fixture is
historical shape evidence, never authority to reuse its old claim or pins.

Run on the authorized Linux controller through the established RTK/SSH wrapper:

```sh
python3 scripts/dev/auth_user_candidate_probe.py fingerprint
python3 scripts/dev/auth_user_candidate_probe.py prepare --binding PRIVATE_BINDING.json --journal PRIVATE_OPERATION
python3 scripts/dev/auth_user_candidate_probe.py run --journal PRIVATE_OPERATION
```

`prepare` performs read-only cluster/resource observations and creates only the
private 0700/0600 journal. It requires verified fixed-route public DEV health200
without redirects and binds all original workloads, Pods, storage, actual
ready Space peer/endpoints, Service/controller specs, and ConfigMap/Secret/
ServiceAccount metadata. Secret values are never retained in that metadata
baseline or printed. The shared journal's private opaque credential members are
unused by this probe and are never issued to a database or projected to a Pod.
The initial command window is fifteen minutes.

`run` atomically creates the mandatory common `voice-dev-image-update-lock` after
durable intent, then serial Auth/User hash Pods and one sleep Job. Each scratch
Pod has no service-selector labels, ports, environment/Secret projection, volumes,
token, host namespace or privileged context. It runs as 65532 with read-only
root, dropped capabilities, RuntimeDefault seccomp and bounded CPU/memory/time.
It only hashes `/auth.jar` or `/service`; it never boots the application. Existing
Service and controller selectors, including expressions, must not match it.
Server dry-run and actual admission may add only recognized harmless defaults.
Unrecognized credentials, sidecars, mounts, pull Secrets or scheduling changes
fail closed. No registry login/Secret mutation is provided by this route.

The artifact receipt requires actual node/container/image IDs, terminal exit0,
and the exact published file hash and size. The Job uses the same fixed User
image with `sleep 60`, backoff0 and a five-second active deadline. A deadline
result requires the actual container to have started, a terminal failed Job
with `DeadlineExceeded`, and the exact owned terminal Pod/container chain.
Pending/image-pull failures and client timeouts cannot satisfy it.

Cleanup uses UID and current resourceVersion preconditions and requires actual
Pod, Job/child garbage-collection and common-lock absence. A DELETE acknowledgement
alone is insufficient. Same-version uncertain deletion is polled without repeating
the request. A fresh changed version on the same verified Pod/Job UID, without a
deletion marker, invalidates the old fenced request; only then can a newly guarded
CAS be issued. Unexplained common-lock version/ownership drift stops.

## Interrupted operation and safe abort

Inspect historical private state without cluster access:

```sh
python3 scripts/dev/auth_user_candidate_probe.py inspect --journal PRIVATE_OPERATION
```

Uncertain creation remains durable `pending`. It is never blindly re-created or
deleted by a mutable name. Known returned UIDs are retained as recovery hints
before admission validation, without granting cleanup authority. If the actual
existing identity, operation timestamp, source spec and owner can be verified:

```sh
python3 scripts/dev/auth_user_candidate_probe.py reconcile --journal PRIVATE_OPERATION --target auth
python3 scripts/dev/auth_user_candidate_probe.py run --journal PRIVATE_OPERATION
```

Other targets are `user`, `deadline` and `lock`. An absent or unrecognized pending
creation remains recovery-required; keep its journal/resource/lock responsibility
and resolve the exact server state. No name-based force cleanup exists.

Expired command time does not authorize further writes or safe release. An
explicit refresh rechecks the same still-held claim/generation/window/candidate,
source, lock and original baseline, and appends a recovery window without
replacing the original intent, credentials or baseline:

```sh
python3 scripts/dev/auth_user_candidate_probe.py refresh --journal PRIVATE_OPERATION --reason "Actual same-claim environment and owned identities reobserved unchanged"
```

A different resource fence needs separate reconciliation, not adoption through
this refresh command. To stop the probe, cleanup can terminate only its verified
credential-free hash/sleep objects through ordinary foreground deletion:

```sh
python3 scripts/dev/auth_user_candidate_probe.py abort --journal PRIVATE_OPERATION
```

Abort persists before cleanup, forbids later normal execution, requires actual
owned-object absence and unchanged original environment, and emits `ABORTED`,
never artifact/controller PASS. Unknown ownership or pending creation preserves
recovery responsibility. No force delete, namespace/data reset, database restore
or foreign lock removal is exposed.

After an actual settled result, reconcile the final unchanged environment and
use the supported tracker protocol to release safe/NONE with the exact journal
receipt. A successful node/controller report is not a DEV application deployment,
credential-isolation PASS, original-OCI rollback qualification, configured-app
startup proof, STAGING eligibility, or final acceptance.

## Scoped verification

Run the focused Python tests inside Docker Linux, alongside the unchanged
adoption module tests. The fake API exercises admission, creation loss, expiry,
CAS/UID drift, foreground GC and positive/abort lifecycle contracts. It does not
claim actual API-server/node execution. The live bounded protocol above provides
that separate version-sensitive evidence only under its real owned window.
