# Disposable full-profile storage recovery

`durable-stores.v1.json` is the versioned inventory derived from
[`DATA_STORES.md`](../../docs/DATA_STORES.md). It covers eighteen service database
names, including target `search_db`, plus object storage, JetStream, ClickHouse
and Redis. Federation remains unprovisioned. Redis is a mixed authority store:
never discard it wholesale as a cache. Meilisearch/Elasticsearch indexes,
instance-local WS state and LiveKit sessions require owner-controlled rebuild or
re-admission rather than resurrection.

The executable drill is intentionally limited to **fresh synthetic data**. It
creates its own UUID-named Docker volumes/containers and an internal network,
with no published host ports. It cannot select existing containers, namespaces,
credentials, Docker volumes or Kubernetes contexts. All image digests are pinned
in the script; preload those images through the normal approved image process.
The command uses Docker's Linux engine and Python 3.11 or later:

```sh
python3 scripts/dev/full_profile_restore.py --synthetic-drill \
  --source-sha "$(git rev-parse HEAD)" \
  --artifact-dir tmp/recovery-proof/new-run
```

The artifact directory must not exist. `recovery-evidence.json` is updated before
resource creation so an interrupted operation retains its intended resource
names and run label `voice.recovery.drill`. There is no implicit resume/recovery
or cleanup of another invocation. Normal completion removes only resources with
the current invocation's exact label; unknown inspection or ownership mismatch
leaves cleanup incomplete.

The drill creates one synthetic account/profile/object binding in each database,
an object with exact bytes, an unconsumed JetStream message and durable consumer,
a retained Redis receipt and a ClickHouse canary. It stops every source engine,
observes them stopped, then creates cold tar backups through read-only source
mounts. Archives restore into exclusively new, disjoint target volumes. Only
the target engines restart. Native queries verify all bindings, exact object
bytes, the retained Redis receipt and pending JetStream/consumer state. One named
synthetic cache key is rebuilt only in the target; no blanket flush is used.
Source archive hashes must remain identical after target verification. The five
actual archives are exported and rehashed before the backup volume is removed.

The report binds the caller-supplied commit, exact executed script/inventory file
hashes and immutable image IDs. It measures synthetic RPO as failure time minus
the recovered Auth canary commit time, and synthetic storage RTO from failure
injection through all native readbacks/source-unchanged checks. The 24h/4h values
are provisional recommendations, never an operator commitment. This tiny fixture
does not measure scheduled backup freshness or restored application readiness.

`SYNTHETIC_STORAGE_PASS` proves the disposable storage mechanism. The databases
contain a **fixture table**, not application migrations. It is not proof of real
service schemas, deletion/retirement/session fences, key/config recovery, broker
ACLs, cross-service application invariants, application cache rebuild or external
provider/media recovery. No application serving is started. G1/PL-A4-11 must
replace the fixture with the exact version's application schemas and owner-level
checks, reconcile current permanent privacy/security fences before serving,
recover operator-owned encrypted keys/config through their existing custody
procedure, and run the authorized isolated operator drill. Shared DEV/STAGING
requires its separate TTenrec resource window. Product/stand/acceptance credit
does not follow from this command.

Run the negative inventory/ownership/cold-boundary tests with
`make a4-disposable-recovery-harness-test`; CI script tests invoke that target.
Native Docker execution is explicit and is not silently substituted for unit
tests or selected by the legacy dry-run harness.
