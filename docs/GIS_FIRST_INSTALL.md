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
