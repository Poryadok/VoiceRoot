# DEV Space fresh replacement contract (implementation in progress)

This operation is restricted to the explicitly authorized disposable DEV
`space_db`. STAGING and every other database remain preservation targets. The
existing `native-migrate --mode=provision` owns creation and the real
44/45/50/51 installer chain; neither a generic SQL migration nor a fabricated
fresh-creation receipt is a substitute. No live execution is authorized by
this document.

Before the first effect, the controller must bind the same namespace, cluster
fence, physical PostgreSQL identity, full database catalog, active writer cut,
source and qualified separate installer platform image. It must derive the
old Space OID from that protected catalog, qualify all other databases and
obtain the exact fixed Job/config plan before reserving the drop. Existing
Space backup is not a prerequisite for the explicitly disposable DEV target;
ordinary preservation and backup requirements for other databases remain.

The drop is one-shot: durable local journal and shared ledger pending record
precede any effect. A fixed `psql` command reads its SQL from stdin, checks
the old OID and drops only `space_db`; it does not use `FORCE`. PostgreSQL
does not provide an atomic OID precondition for `DROP DATABASE`: this check
and the drop are separate statements in one session under the established
cooperative fencing and trusted cluster/administrator boundary. Recheck the
engine, guard, zero affected sessions and all other database identities before
and after. Loss of the response or durable acknowledgement requires uncertain
hold and explicit reconciliation; no replay or name-based adoption.

Only a guarded, observed absence and unchanged other catalog can complete the
drop record. A single create-only source-bound installer Job follows: durable
pending before POST, accepted Job UID/resourceVersion durably acknowledged
before other guards, and actual owned Pod/platform identity. The installer's
first genuine `created` JSONL event identifies its generated run UUID and new
database OID. Bind that exact event to the Job/Pod/log and actual database in
the protected journal/ledger before the source-owned replacement catalog
successor is used. Unknown/missing logs, ambiguous Pod or an uncertain create
remain hold; a live matching name does not establish ownership.

The installer uses `native-installer-<run UUID>` for its admin PostgreSQL
session and target/migrate sessions. The central observer may correlate this
actual logged UUID and role with the verified owned Pod IP in both `postgres`
and new `space_db`; this is operational correlation, not cryptographic
identity. Ambiguous network attribution, foreign sessions or prepared
transactions hold. The create-to-acknowledgement window is not an atomic
revocation mechanism.

Completion requires actual complete original installer evidence, the same new
target OID/system identity and real 44/45/50/atomic51 receipt chain, followed
by the canonical non-owner serving-role setup, fresh full51 verification and
zero migration sessions. Job/Pod/runtime platform and original evidence are
retained by the existing protected controller custody. The legacy evidence
ConfigMap or an existing Job must never be silently replaced or adopted.
Candidate C2 requires read-only verification using the original immutable tool
and a separately source-owned compatibility result for the new candidate;
it must not claim the new image performed the original creation.

The current implementation and tests are not yet complete. In particular,
source-bound tool artifact evidence, the typed replacement-catalog/Job guard,
the one-shot phase, real PostgreSQL/Kubernetes composition, runtime grants and
C2 continuity remain required before live use.
