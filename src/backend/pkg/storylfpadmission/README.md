# Story/MM LFP admission boundary

This package is the shared component ABI for
`BE011-STORY-LFP-RESTORE-CONTROLLER`, following the accepted
`BE011-STORY-LFP-RESTORE-COHORT-v1` decision (BE011-LFP-CONSENT-PRODUCER c7).
Consumers are the Story consent journal and MM registration/ACCEPT composition.
The reviewed v1 wire and its declared hashes remain unchanged.

`Origin` contains immutable environment, installation, external incarnation and
the admitted Story database binding. Its constructor validates data only. Store
it with offers, consents, registrations and new MM source material. Constructing
an Origin never establishes admission, authentication or database continuity.
Public errors contain no private bindings; JSON cannot serialize its fields.

The installation root must inject an `Admission` implemented by its actual bound
controller composition. `Enter` must verify current external authority and the
actual connected database/session binding. It returns a permit whose Origin is
not supplied by the application request. Missing, partial or unknown authority
returns Unavailable; known retired or mismatched origin returns Ineligible. There
is deliberately no JSON/env loader, success stub, permissive default, local
database marker authority or exported operation that opens a cohort here.

`Barrier` wraps that factory with a process-local close/drain. Obtain its permit
before starting a short transaction, check the local admission head and persisted
row Origin under that transaction's locks, and release only **after commit or
rollback finishes**. Network operations belong before those locks. Close rejects
new local permits, including an authority lookup finishing after close, and waits
for all admitted permits to release. Cancellation leaves it closed; it cannot be
reopened. The controller still must close/drain every replica and retire all old
execution, credentials and routes. An uncompleted authority lookup is denied and
released on return; this local wrapper does not drain remote authority operations.

The protected validation transport implements `AuthenticatedPeerAuthority` by
resolving the **actual authenticated responding endpoint/credential** against the
current admitted trust manifest. `ResolvePeer` retains that verified Origin in a
`VerifiedPeer`; MM must carry it alongside the exact validation response and
compare it with the registration Origin and transaction's current admission head.
A caller-provided cohort claim, stable DNS or ordinary service TLS identity alone
cannot implement this contract. There is no default transport adapter here.

## Controller continuation and actual inventory

The repository composition at base `304b49dcf5b771efae5a7d4ddaa28a3d0b9ddb5d`
has Story/MM ordinary listeners, DATABASE_URL from `voice-app-secrets`, and no
separate Story-LFP controller state, admitted launch descriptor, credential cohort
retirement or full replica/transaction-drain protocol. No installed authority is
proved by this package. Existing foreign test containers are not its controller.

The bounded deployment owner still needs a durable external record containing
environment/installation, current incarnation, exact launch manifest hash and
consumer set, plus at most one immutable prepared transition naming predecessor,
fresh incarnation and prepared exact database/credential/route bindings. The
transition cannot commit until authenticated close/drain, old execution and
credential retirement, database prepare and both consumers' readback receipts
match that same transition and manifest. Crash/lost acknowledgement resumes that
transition; missing state never means a clean installation. Its state must remain
outside Story/MM restore domains and application write access. A matching hash,
copied descriptor, signature or component fixture acknowledgement does not prove
this custody. No deployment or credential provisioning is supplied here.

Real installation qualification, managed rollback oracles, simultaneous MM
restore proof, hosted CI, shared-environment checks and independent acceptance
remain open. Ordinary Story and already-consumed sources in unchanged MM retain
their own contracts. The package tests prove only the local typed boundary and
transaction barrier; their synthetic authorities are explicitly component fixtures.

Run affected checks from `src/backend/pkg` with `go test -race
./storylfpadmission` and `go vet ./storylfpadmission`. Compiled tests must use the
authorized isolated Linux runner while the Windows host restriction applies.
