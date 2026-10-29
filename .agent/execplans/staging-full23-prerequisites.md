# STAGING full application prerequisites

The target is one canonical immutable 23-application build deployed to DEV and
then to STAGING with the same application image digests. A repeat build follows
after the required four-hour interval. Source changes here do not authorize a
deployment or establish installed-helper eligibility.

## Observed state and scope

On 2026-10-29, read-only STAGING prepare claim generation 5 observed 22 of 23
application Deployments, with Game Integration absent. The three frontend
runtime ConfigMaps, the Realtime database URL key, seven P3 objects and the
three GIS Secrets were absent. `realtime_db` was absent from PostgreSQL's
catalog; `auth_db` and its Flyway history existed, but migration currency was
not established. The Realtime role, certificate validity and release-policy
content remain unknown. Two complete workload snapshots were unchanged and
the claim was safely released with outcome NONE. Private receipts are
`93eb55db…`, `1908fb62…` and diagnostic `4a085916…`; no Secret values or
business rows were recorded.

The protected full/app-only compiler must capture the Realtime SQL sources and
require its existing credential before allowing its pinned migration job. The
three fixed frontend ConfigMaps may be created only when an authoritative
namespace list proves absence, with server-normalized source content bound to
the target, a durable pending record before each create, and an exact UID and
resourceVersion readback. A lost response is uncertain and cannot be adopted.
They are not included in the generic apply list. GIS first install remains a
separate add-only transition; frontend creation is deferred to a later ordinary
full/app-only transaction. Images-only continues to preserve existing config.

The existing protected preflight requires Realtime DB and credential before
the ordinary rollout fence. Provisioning them therefore needs a separately
reviewed source-owned protected subphase with actual TT and root-lock authority,
fixed database/role scope, credential custody and durable uncertainty handling.
The first-install request may run this prerequisite before its GIS-only apply;
the prerequisite has its own receipt and is not part of the pair-only apply.
Subsequent rollouts need a root-loaded, read-only admission linked through the
previous durable PASS to the original receipt and to current database and
credential identities. Merely finding an existing database or URL is not proof.
P3 requires STAGING-specific authorized material; DEV keys are not a source.
The four owner release-policy input is required independently of unfinished
broader product qualification.

## Verification and remaining work

Focused Linux tests cover migration source/secret binding, source-plan frontend
absence, create-only journal and failure seams, and existing GIS lifecycle.
Independent compiler/Session tests and security review are in progress. The
first-install compiler preflight previously rejected its own extra GIS field
and then rejected valid actions shared across checks. The candidate validates
the closed non-NATS plan before attaching the separately validated source pair
and permits the already-validated shared action references. Independent actual
compiler tests pass for both full and app-only with a synthetic API and TT
journal. Security review and real runtime qualification remain separate.

Complete the Realtime bootstrap and frontend integration, review the exact
source and tests, then rebuild and independently review the helper transition
against the actual installed cohort. CI, installed idle eligibility, actual
DEV PASS, matching STAGING digest map, protected rollout and runtime evidence
remain distinct requirements. No live deployment or credential issuance is
authorized by these source-level results.
