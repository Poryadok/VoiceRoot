# Alpha operator recovery and triage

This is the small operator entrypoint for the A4 alpha slice. Use the selected
release's source, [operations policy](OPERATIONS.md), [deployment authority and
diagnostics](DEPLOYMENT.md), and the environment's own runbook. Commands below
do not appoint an operator or authorize a rollout. A source check, ready pod,
backup upload, or historical receipt is not current product acceptance.

## Before an operation

Record the environment, cluster context, namespace, full source SHA, image
digests, task package, current/previous release, operator, checker, and UTC start.
Keep private logs and backups in an access-controlled evidence directory; publish
only redacted summaries and hashes. Do not record tokens, key material, raw
signed URLs, or customer content in TT/CI comments.

For shared DEV/STAGING, the executing operator must read current TT resource
state and obtain its atomic claim before rollout or version-sensitive checks:

```sh
tt --project voice resource list
tt --project voice resource show DEV       # or STAGING, for the actual target
```

Prepare an immutable candidate before acquisition. An occupied resource needs a
`resource wait` continuation and other work. `recovery_required` needs actual
stand/process/external-lock reconciliation, supported recovery, a fresh known
observation, then separate `confirm-safe`; it is not free. Task ownership does
not reserve a stand. Keep the same protected window through rollout and bounded
checks; transfer with offer/accept, reread rotated authority, and hand off tasks
separately. Release only after actual safe completion; uncertain partial effects
remain in recovery. TT guards records only: retain the DEV updater lock and the
Kubernetes/NATS preservation locks, and reconcile unintegrated external writers.

Use the explicit context/namespace from that recorded target. These observations
do not acquire a resource or prove a stable version:

```sh
: "${VOICE_CONTEXT:?select the reviewed cluster context}"
: "${VOICE_K8S_NAMESPACE:?select the reviewed namespace}"
kubectl config get-contexts "$VOICE_CONTEXT"
kubectl --context "$VOICE_CONTEXT" -n "$VOICE_K8S_NAMESPACE" get deploy,sts,pods,pvc
kubectl --context "$VOICE_CONTEXT" -n "$VOICE_K8S_NAMESPACE" get deploy \
  -o custom-columns='NAME:.metadata.name,IMAGES:.spec.template.spec.containers[*].image'
kubectl --context "$VOICE_CONTEXT" -n "$VOICE_K8S_NAMESPACE" get pods \
  -o custom-columns='POD:.metadata.name,IMAGES:.status.containerStatuses[*].imageID,READY:.status.containerStatuses[*].ready'
```

Verify the intended services, actual immutable image IDs, readiness, current
migration state, and absence of a conflicting writer. Missing/ambiguous image IDs
or a different context/namespace stop the operation; do not repin the candidate
silently to what happens to be deployed.

## Deploy and rollback

| Target | Supported entrypoint | Required evidence / stop condition |
|---|---|---|
| DEV image-only | [DEV updater instructions](DEV_SERVER.md#обновление-wip-и-откат-образов) and [tooling](../scripts/dev/README.md) | Preview the exact digest set, then use the same set with `--apply`; retain old digests. The updater does not apply migrations/configuration. Failed locks require reconciliation, not manual deletion. |
| STAGING | [Staging deploy workflow](../.github/workflows/staging-deploy.yml) and [protected release/rollback authority](../scripts/staging/nats-rollout-preservation/README.md#release-and-rollback-authority) | Exact reviewed source/CI and image selection; normal captured-source, cold-cut, encrypted-backup/readback, isolated restore and native preservation gates. Preserve workflow/run/attempt, request nonce, operation and terminal receipt. |
| Production | [Production workflow](../.github/workflows/prod-deploy.yml), [manifest instructions](../deploy/prod/README.md), [host runbook](PRODUCTION_HOME_SERVER.md) | Independent production credentials, explicit immutable release and environment approval. Staging pins/Secrets/backup evidence cannot be substituted. |

Do not invoke an inner `rollout-*.sh`, `deploy-changed.sh`, `kubectl set image`,
or reset script to bypass its outer admission/preservation workflow. A new
migration follows expand → compatible binary → separately approved contract;
an image rollback does not undo schema or already-written data.

For staging rollback, select the latest eligible successful operation under the
[preservation runbook](../scripts/staging/nats-rollout-preservation/README.md).
Dispatch the protected workflow's `rollback_operation` route with that actual
operation, its required reviewed source inputs, and a fresh cut of current data.
Root derives the exact changed workloads/old digests from its receipt. Preserve
post-release records; never substitute `kubectl rollout undo`, replay an old
runner, or use a failed/ABORTED operation as a rollback source. Follow the
production host's separate immutable-release procedure for production.

If deployment fails, stop blind retries. Read the current request report on the
authorized staging runner, using the actual retained request nonce:

```sh
python3 /var/lib/voice-nats-preservation/installed/code/nats-rollout-preservation/bridge_client.py report REQUEST_NONCE
```

Bind report run/attempt/source/helper/operation and timestamp to the failed run.
`UNKNOWN`, missing report, or a `STARTED` journal is not no-effects evidence.
Preserve original journal/checkpoint/report and private diagnostics; inspect
current phase/status/fence/error before choosing the matching recovery route.
Do not replay historical one-shot recovery commands from incident notes.

## Canary observation and release decision

[Canary policy](OPERATIONS.md#релизы-canary-и-rollback) starts at 5–10% for
user-facing services. Before changing traffic, record the actual routing
mechanism, selected old/new images, traffic allocation, synthetic scenario,
baseline, observation duration, and concrete error/latency rollback thresholds.
The policy does not provide a universal traffic-split command. No implemented
router, absent thresholds, or zero samples means the canary drill is pending;
do not describe a rolling Deployment as a traffic canary.

Observe Gateway 5xx and latency for auth/messages/chats/WS plus the affected
service's own readiness and domain errors. Use
[observability queries and dashboards](features/observability.md) and its
[actual scrape/rule definitions](../deploy/observability/README.md). Require
nonzero synthetic traffic and the chosen source's product assertions. Record
before/after sample counts, query range, values and pass/fail against the recorded
thresholds. A regression, migration error, contract incompatibility, or confirmed
critical bug takes the supported rollback/recovery route above. Do not change
the threshold after observing failure. The full automated drill is
`PL-A4-04`; this document does not claim that harness is shipped.

## Isolated restore

Inventory the authoritative stores from [DATA_STORES.md](DATA_STORES.md),
including object/event stores and required private credentials. State which
cache/projection is excluded and how it will be rebuilt. Preserve image/schema
versions, snapshot time, checksums, encryption/key custody and off-node readback.

For the existing home-production restic repositories, an authorized operator
can inspect a selected immutable snapshot without changing production:

```sh
sudo env RESTIC_PASSWORD_FILE=/etc/voice/restic-local-password \
  restic snapshots --repo /var/backups/voice-prod/restic
sudo env RESTIC_PASSWORD_FILE=/etc/voice/restic-local-password \
  restic check --repo /var/backups/voice-prod/restic
```

Use the [host backup/restore procedure](PRODUCTION_HOME_SERVER.md#резервные-копии)
for the separately managed off-host repository. Restore only into a newly owned
isolated directory/namespace/cluster with no source-store write route. Never
restore into an existing production data path or deploy archive Kubernetes
resources against the live context. Check complete transfer bytes and hashes,
`zstd -t` for compressed dumps, and actual owner-schema restore into disposable
databases. An archive/hash check alone is not database or product restoration.

Validate synthetic tenant/object invariants, cross-service references, attachment
download/hash/delete access and the declared projection rebuild. Keep Voice
durable lifecycle/divergence evidence; a Redis restore/flush must not remove PG
fences. Record recovered snapshot time and elapsed restore/recovery time to
measure RPO/RTO; owner-approved thresholds remain the release decision.
The existing Space-only scratch-PG helper in the preservation workflow is not
a full-profile restore. `PL-A4-05` owns the complete manifest/restore harness;
missing stores, keys, invariant checks or target isolation leave that drill open.

## Redis and NATS: bounded triage

First correlate exporter health with Postgres, pod restarts, disk and Gateway
errors. PostgresDown inhibits derivative alerts; silence is not recovery. Use
the deployment names actually present in the recorded namespace:

```sh
kubectl --context "$VOICE_CONTEXT" -n "$VOICE_K8S_NAMESPACE" get pods -l app=voice-redis
kubectl --context "$VOICE_CONTEXT" -n "$VOICE_K8S_NAMESPACE" logs deployment/voice-redis --tail=100
kubectl --context "$VOICE_CONTEXT" -n "$VOICE_K8S_NAMESPACE" get pods -l app=voice-nats
kubectl --context "$VOICE_CONTEXT" -n "$VOICE_K8S_NAMESPACE" logs deployment/voice-nats --tail=100
kubectl --context "$VOICE_CONTEXT" -n "$VOICE_K8S_NAMESPACE" get pvc
```

Logs remain private until redacted. A legacy/absent deployment name is a signal
to reconcile topology, not to create a replacement broker or delete storage.
Use [infra dashboards/exporters](../deploy/observability/exporters/README.md)
for Redis availability/memory/connections and NATS availability/consumer pending,
redelivery and ACK progress. Distinguish missing telemetry from a zero backlog.

For Redis, check rate-limit/blacklist/auth effects and WS cross-instance fanout;
privacy and protected-principal failures remain fail-closed. Never `FLUSHALL`,
delete lifecycle keys, weaken ACL/blacklist policy or restart unrelated pods to
obtain a green probe. Follow [degraded behavior](OPERATIONS.md#tier-0-отказ-redis-и-nats-сценарии-и-degraded-ux)
and the service's current implementation; do not promise one global fail-open
policy for every Redis consumer.

For NATS, distinguish persisted-message/live-delivery degradation from failed
REST writes. Reconnect history is Messaging REST per chat; no global WS catch-up.
Do not ACK/discard stuck messages, reset consumers, purge streams, remove the
PVC or run a reset/rotation/bootstrap as generic recovery. Correlate stream and
durable-consumer state with the current preserved operation/generation using the
[NATS preservation instructions](../scripts/staging/nats-rollout-preservation/README.md).
Recovered broker health requires a new bounded product check: two clients,
persisted message, receiver delivery/reconnect history, ACK/backlog convergence,
and no duplicate visible effect. A healthy socket alone does not prove delivery.

## P1 receipt, acknowledgement and escalation

[P1 means immediate reaction](features/observability.md#p1--немедленная-реакция).
Before relying on paging, verify the receiver is configured, not `null`, and the
target environment's real exporter/rule coverage is active. The shipped
[base Alertmanager config](../deploy/observability/alertmanager/config.yaml)
intentionally discards notifications; the notifications template also needs real
destination/SMTP configuration. Do not label firing-rule visibility as delivery.

For a disposable alert drill, use the synthetic alert procedure in the
[observability runbook](../deploy/observability/README.md#5-synthetic-alert-test-optional)
with its designated test destination and authorized Linux runner. It starts
loopback port-forwards: do not run it as a new Windows listener during unattended
execution. Record alert labels/fingerprint, firing time, delivery time and actual
destination receipt without exposing credentials.

The real responding operator records human acknowledgement, owner, impact,
incident/task reference and next action. Alertmanager silence/inhibition is not
human acknowledgement and does not establish recovery. If no responder accepts,
the incident progresses, or authority/data safety is uncertain, escalate
immediately to the environment/service owner using the prearranged contact path;
record who accepted. No contact roster, secondary responder, or actual receipt
means the drill is blocked. `CI-004` owns the real destination; this runbook
does not invent contacts, numeric acknowledgement SLA, or send a notification.

Close only after the affected user scenario and alert resolution are observed on
the exact selected version. Retain failure history and a concise cause/fix,
remaining impact, receipt evidence and follow-up. Keep a healthy shared resource
only through bounded checks; publish pending independent acceptance and release
safely rather than claiming it passed.

## Drill evidence checklist

| Drill | Required retained result |
|---|---|
| Deploy → canary → rollback | Old/new immutable material, migrations/compatibility, protected operation receipts, nonzero traffic, fixed thresholds/window, trigger observation, fresh-cut rollback preserving post-release records, final product checks. |
| Backup → isolated restore | Complete durable-store manifest, off-node bytes/hash/key evidence, target isolation, actual schema/data/object restore, synthetic cross-domain assertions/projection rebuild, measured RPO/RTO. |
| Redis/NATS failure → recovery | Isolated failure injection authority, baseline/failed/recovered observations and user-visible expectations, no unsafe bypass/reset, recovery convergence and cleanup. |
| P1 | Active rule and configured destination, real delivery receipt, human ACK/owner, actual escalation acceptance when exercised, resolved observation. |

Run these only on explicitly selected disposable targets, retaining their locks
and exact version. Missing prerequisites are pending work, not skipped PASS.
Offline syntax/link/contract tests validate the instructions' source surface;
they do not replace these operational drills or `G1` acceptance.
