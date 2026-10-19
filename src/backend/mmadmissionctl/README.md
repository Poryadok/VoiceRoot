# MM admission controller components

This module is a source checkpoint of the MM-only controller library. It is
**not an installed authority, executable installer, or completed native rollout**.
`NewLocalController` admits only `LOCAL_OWNED`; no option relabels it `INSTALLED`.
The source contract is BE011-MM-INSTALLATION-ADMISSION c4–c9 (producer v1,
SHA256 `e767ab002c7b2b20c3a9f567fad7141cf7db13ef7ba96b8662e1c86d2556b481`)
and c12–c13 (cross-UID peer v1, SHA256
`7546b2690315c5e5b34c46091233a42d6a882478aa4c7a155587465fe06015e6`).

## Implemented boundary

- SQLite intent/result/effect journal, revision CAS, exact operation replay and
  an exclusive Linux operator lock. An interrupted effect remains pending;
  readback cannot certify it as a fresh admission.
- Typed prepare/arm/open/close/reconcile driver boundary. Intent is durable
  before external calls; no SQL transaction is retained over those calls.
- Independent read-only journal handle and authenticated readback. Every
  connection requires the current exact driver registration, kernel PID/UID/GID
  from that same Unix socket, and mutually pinned per-launch TLS 1.3. Legacy raw
  readback is disabled. Authenticated TLS close-notify is mandatory.
- A concrete per-launch private FD3 producer delivers independent expectations
  and only the serving process's own TLS identity to the shared native loader.
  It uses a root-driver Unix socketpair, CLOEXEC/ExtraFiles custody, real recipient
  PID/UID/GID and exact descriptor bytes; public pins cannot replace private input.
  The loader installs no-exec and closes the one-shot channel before returning.
- PostgreSQL head installation/closure against MM024 and MM025, using the
  existing stable admission gate. The installer validates separate serving and
  controller roles, exact immutable receipt indexes and retained continuity.
  Old evidence is never reconstructed after a restore loses it.
- Owned Docker object creation records, exact resource-ID/creation-nonce
  readback, private network membership checks and bounded cleanup. This is a
  resource component, not a complete implementation of `Driver`.

The MM025 schema used by the integration check is the consumer-owned immutable
commit `ed3b7d795a28a327f0b2fb41bd503953ef8a9c66`; this module does not own its
migration. Serving needs SELECT on admission evidence and its separately granted
domain writes, never controller head-update privileges.

## Evidence and reproduction boundary

Run ordinary source checks inside an owned Linux Go 1.26 runner:

```sh
go test -race ./...
go vet ./...
```

Two explicit integration tests skip by default:

- `MM_ADMISSION_IPC_TEST=1` enables `TestActualCrossUIDLaunchTLS` in an isolated
  root runner. It launches readback UID0 and serving UID/GID65532 in the same PID
  namespace, verifies both actual binaries with an owned driver observer before
  delivering keys, and exercises ARMED startup followed by OPEN fresh readback.
  Both dedicated processes install the process-wide no-exec sandbox and actually
  fail an exec attempt. Serving's read of the root executable remains denied.
  Only these owned processes use the fixed `/run/voice/installation` test paths.
- `MM_ADMISSION_DOCKER_TEST=1` and a fresh `MM_ADMISSION_LOCAL_SCOPE` enable
  `TestOwnedDockerActualPrivateResources` in an owned root runner whose Docker
  socket and scope label match the test. The pinned MM migrations must exist at
  `/proof/mm-migrations/src/backend/migrations/matchmaking_db`. It creates its own
  private PG/Redis/NATS resources, publishes no host ports, checks real data-plane
  replies, role isolation, head idempotency, a gate-blocked close and generations
  1→2→3, then removes its own objects. Never enable it in a foreign runner.

The IPC and PostgreSQL tests deliberately use synthetic upstream, source-factory,
route, image/mount/custody contract inputs. They prove component behavior and real
kernel/service interactions. They do not prove an admitted native source, provider
qualification, full launch custody, DEV/STAGING rollout or installed authority.

## Outstanding before a product admission

There is no controller CLI, complete native Docker driver, qualified installed
launch/credential custody, full native source/all-session/intent cycle or successful
restart/restore qualification in this checkpoint. Unknown domain operations
currently close admission rather than claiming completed BE012 settlement.
Reconciliation retains the original interrupted operation as unresolved; it does
not manufacture its effect result. The complete ten-oracle peer matrix and full
producer twelve-oracle cycle remain outstanding.

Launch image, executable, argv, mounts, namespace mapping, key/FD custody and
upstream/storage control must be independently observed by an authorized driver.
A public descriptor or peer-binding file, TLS success, test fixture, or matching
hash cannot supply that authority. No existing Voice/Story keys or operations
172/191/230/372 are reused. The next implementation must retain these gates.
