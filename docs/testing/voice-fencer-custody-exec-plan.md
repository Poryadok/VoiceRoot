# ExecPlan: finite Voice resource-fencer custody

## Purpose and contract

BE011-NATIVE-COVERAGE-CUT c12 / BE011-FENCE-CREDENTIAL-CUSTODY-v1 supplements
owner-retirement c4 and provider-history c6. Administrative resource handles
belong to an installer-controlled finite process. The executor, native
maintenance and serving processes never receive its administrative connection,
secret mount, inherited descriptors or runtime-control socket.

The chosen first implementation is Linux, installer-launched one-shot
`cmd/voice-party-fencer`. It accepts only operation and UUID/hash references to
installed canonical commands or proofs. There is no listener, arbitrary SQL,
DSN/path/config flag, serving mode, role-membership exception or SECURITY DEFINER
bridge. PostgreSQL administrative capability remains honestly within the
installation's trusted computing base; it is not a data-blind SQL principal.

## Implemented component boundary

`internal/fencecustody` checks installer UID0 ownership and non-writable
ancestors through descriptor-relative no-symlink opens. It rejects writable,
nonregular, linked, oversized or noncanonical documents. Secrets additionally
deny group/other access. Receipt publication uses fsync and atomic no-replace
rename; exact retry preserves bytes, differing retry conflicts.

Fixed paths are installer-controlled mounts:

| Path | Fencer | Executor reader |
| --- | --- | --- |
| `/var/lib/voice-party-fencer/plans` | read-only | read-only |
| `/var/lib/voice-party-fencer/proofs` | append | read-only |
| `/var/lib/voice-party-fencer/observations` | append | read-only |
| `/var/lib/voice-party-fencer/locks` | private write | absent |
| `/run/secrets/voice-party-fencer` | private read | absent |
| `/run/voice-party-installation` | read-only | read-only |

The installer admits command identity/body hash, complete cut bindings,
predecessor execution cohort, exact database and target-role identity,
credential custody reference, fencer login OID/name, executable hash and Unix
peer endpoint. A root-owned external installation window admits exact command
references. Its execution UUID separates current observations from prior
installer executions/recovery. The window is an authorization record: its bytes
alone never prove closed ingress or complete topology custody.

Before mutation the process requires actual UID0 without a setuid disguise,
protected fixed artifact bytes, the admitted window, the exact private Unix
endpoint and server peer UID, actual current/session/authenticated backend login
OID/name, the installed administrative principal and physical database identity.
Catalog queries use fixed `pg_catalog` search_path. Remote/TLS/failover routes
and scoped CREATEROLE/member-administration variants are unsupported here.

The existing PostgreSQL algorithm preserves all privilege, inherited authority,
prepared transaction and ANY incoming delegation refusals. NOLOGIN commits
before positive-timeout termination of the exact old authenticated sessions.
The actual database/role/session binding and full postconditions are reobserved
before publishing sanitized evidence. The fencer issues no native owner DML and
does not advance the coverage head.

Receipts have stable command-derived identities. Reobservations are separate
append-only records. A non-root reader validates local provenance, complete
command/receipt/evidence bindings and the current external execution window.
Reading history does not perform a fresh resource observation. The installer
must launch bounded reobservation before cleanup/recovery; a cached reference
is not a substitute. Subsequent execution-window rotation invalidates old
observation readback while retaining historical receipt bytes.

## Validation performed

Owned Linux Go1.26/Docker, private existing network and no host ports/listeners.
The real PostgreSQL fixture uses Unix peer mapping for the administrative
fencer and rejects its TCP login. The fencer container has no network, a
read-only root filesystem, dropped capabilities and private administrative
mounts. The UID1000 reader has no network, PostgreSQL socket, private secret or
runtime-control mount; its proof mounts are read-only. Installer/test access is
separate and is never attributed to the executor.

Tests cover protected filesystem permissions/symlinks/conflicting append;
actual UID1000 receipt read with secret/forgery denial; canonical source/cut/
receipt/installation bindings; wrong command/body/OID/name/database/peer/
artifact/endpoint/incarnation without mutation; an actual paused predecessor
transaction killed before commit, subsequent login refusal, unchanged other
role/owner rows, immutable retry/readback and fresh observation after external
execution rotation. Sanitized canonical component bytes and exact compiled
source/artifact hashes are retained in the ignored evidence directory. No media
protocol test was rerun for this custody change.

The separated executable also refuses an actual predecessor prepared
transaction without committing or rolling it back, and refuses an incoming
role delegation without changing LOGIN or revoking membership. Only the
installer fixture resolves those blockers. After the installer deliberately
re-enables the predecessor, fresh observation fails without mutating it;
explicit fencing then terminates the restored session and retains the stable
receipt with a new observation. Historical readback alone cannot prove this
current resource condition.

A separate owned backend is suspended inside its PostgreSQL container using its
actual authenticated PID. Positive-timeout termination fails, NOLOGIN remains
committed and fresh observation refuses the still-established predecessor. The
installer resumes that exact process; recovery fences and reobserves the same
command, retaining the original receipt identity. This exercises a real partial
failure after NOLOGIN, without changing the production termination algorithm or
signalling host/foreign processes. An initial fixture attempt lacked `/bin/kill`
and failed before suspension; the final fixture uses the installed shell builtin.
That setup failure is not a product RED result.

The same stopped-backend schedule also observes committed NOLOGIN and sends
SIGKILL to the exact fencer container created by the fixture. It verifies exit137,
retained denial and fresh-observer refusal while the predecessor remains
established. After installer-only resume, same-command recovery retains the
receipt and issues a new observation. This covers an actual fencer crash after
NOLOGIN during termination; it does not stand in for every later publication
crash boundary.

The prepared-work race uses an installer-held `pg_authid` ShareLock. The test
observes the real fencer blocked at ALTER ROLE after its read-only prechecks,
then the predecessor prepares a transaction. Releasing the lock allows NOLOGIN
and termination, but the final observer refuses positive proof and leaves the
prepared transaction unresolved. Only the installer fixture rolls it back;
same-command recovery then succeeds. The barrier uses actual catalog lock
state, not a production hook or a timing assumption. An initial fixture closed
the replacement client at subtest exit; that lifetime error was corrected and
is not a product failure.

The same observed catalog-lock schedule also installs an incoming delegation
after the real fencer's prechecks. Final observation refuses proof, leaves the
new membership intact and retains NOLOGIN. Only installer cleanup removes that
delegation before exact-command recovery; there is no trusted-role exemption.

A distinct admitted command also exercises actual receipt-store failure after
resource completion. Its cap-drop-all root process cannot write a root-owned
0555 proof directory. The call fails after NOLOGIN/session termination and no
receipt file exists. Installer repair of that directory permits exact-command
recovery and authenticated readback. This is a real publication error, not an
assertion that an actual process crash at that later instruction was tested.

These are component proofs. Fixture source/schema/writer/provider manifest
hashes do not constitute the assembled native package or closed catalogue.

## Remaining work and honest assumptions

- [ ] Exercise an actual process crash after termination before evidence
      publication; the store-error recovery and earlier real SIGKILL cases do
      not replace this exact crash schedule.
- [ ] Installer-enforced complete deployment/controller/credential history,
      non-restored external incarnation and all writable/promotable endpoints.
- [ ] Bounded installer launch/response composition for fresh observations,
      then authenticated issuer component ingestion and maintenance authority.
- [ ] All genuine source-owner receipts, actual native factory validation,
      maintenance/fencer settlement, complete isolated activation/startup proof.
- [ ] Independent security review of this component and existing coverage core;
      source publication is separate from develop/CI/runtime acceptance.

The installation root must prevent competing administrators changing roles,
membership, resource routes or the admitted window during a cut, and must stop
or revoke transient component access in the accepted order. The application
cannot establish those facts from SQL locks, labels or config booleans. Restoring
both resources and their external trust root to old identities is outside the
guarantee; authorized restore requires a fresh external incarnation and closed
admission reconciliation. No production authority or full activation is claimed.
