# ExecPlan: local CI Docker isolation

## Purpose

Allow two agents to run selected Go and Java Testcontainers checks and Compose
checks concurrently on a shared Docker host. Each run must own its resources,
clean up after success or interruption, and leave other runs and foreign
containers untouched. The local preflight must consume this isolation before
it can issue a handoff receipt.

## Context

- Requirements: TTenrec `CI-LOCAL-DOCKER-ISOLATION` and
  `CI-SWARM-LOCAL-PREFLIGHT`; `docs/TESTING.md` and `docs/CONTRIBUTING.md`.
- Existing foundation: `scripts/ci/local_docker_owned.py`,
  `local_docker_scheduled.py`, `local_heavy_slots.py`, and
  `local_compose_audit.py`. These isolate only explicitly owned fixtures; they
  do not yet isolate arbitrary Testcontainers or Compose workloads.
- `scripts/ci/testcontainers-prune.sh` must not be used for local run cleanup:
  its global Testcontainers selection can remove another agent's resources.
- Private rootless Docker feasibility was recorded in tracker comments c59–c62:
  separate daemons ran real Go and Java Testcontainers and a synthetic Compose
  workload. These experiments are not a production scheduler or security
  review.
- A later startup probe used a private cgroup namespace. The new
  `src/backend/pkg/cmd/ci-rootless-create` helper and selected-field inspection
  check have been exercised against an owned daemon; the container itself is
  not yet journaled or connected to the scheduler.
- A bounded endpoint resolver rejects remote or ambiguous Docker contexts; a
  real wrong-daemon creator request was rejected before container creation.
  The resolver does not establish helper provenance or complete admission.
- The current unintegrated creator uses only a Unix socket in an owned volume.
  Its freshly built binary and selected-field validator passed a private probe:
  a separate owned client reached the socket, TCP ports 2375 and 2376 refused
  connections, and all exact owned resources were absent after cleanup. This is
  feasibility, not a journaled lifecycle or an exclusive-access guarantee.
- A separate owned client also used testcontainers-go v0.35.0 to create, execute
  in, and remove an inner container over that socket. Its image was preloaded
  and Ryuk was disabled for the disposable inner daemon. A separate journaled
  Maven client also ran a standalone Java Testcontainers 1.21.0 case against
  the socket, with all outer resources removed. A private source snapshot also
  ran the real Auth `AccountDeletionEpochFloorJdbcIntegrationTest` with one
  pass and zero skips. These are feasibility probes: Auth was compiled on the
  host, its dependency seed lacks receipt-grade provenance, and concurrency and
  crash recovery remain unverified for this profile.
- The Unix daemon profile now has an ownership-journal creation primitive. A
  real Docker probe recovered an already-created daemon after a simulated lost
  response, started it, and served an owned client running a Go Testcontainers
  probe. All six journaled resources were removed. The image load used a direct
  experimental exec; creator authentication, socket access, other interruption
  states and a fully journaled workload remain separate requirements. This
  primitive is not preflight proof.
- A source-built Chat `TestSpaceMembersStore_IsAndList` passed without skips
  against two real PostgreSQL Testcontainers, including mapped-port access, on
  the journaled Unix daemon. Image loading used experimental direct exec and
  dependency/tool provenance is not yet bound to a preflight receipt.
- The daemon now has a journaled image-load primitive that hashes transmitted
  bytes, checks the complete image inventory and expected image ID, and keeps
  uncertain operations unresolved. Recovery stops and quarantines the daemon
  and its volumes before cleanup. Real valid and invalid archive probes passed;
  caller-supplied digests and socket access are not provenance proof.
- A synthetic Compose project also started and removed one network-disabled
  service on the journaled private daemon. Compose ran via direct experimental
  exec, so this does not establish the Voice Compose runner or its recovery.
- Two separately journaled Unix daemons occupied both host slots and had their
  synthetic Go Testcontainers workloads observed running simultaneously. Both
  cleaned up while an independent sentinel survived. In a second probe, one
  exact client was stopped and its run recovered while the other workload and
  sentinel remained live. A third Windows-only probe killed one worker and its
  Docker client descendants through a preassigned Job Object during execution;
  recovery removed its exact run while the peer and sentinel stayed live. Linux
  supervision, uncertain daemon-side create and source-bound workload inputs
  remain unproven.
- The tracker records one Windows bind/volume read measurement and one
  network-isolated Go compilation comparison in comments c63–c64. They are
  single paired measurements, not general performance guarantees.

## Scope

- In: exact run ownership, a bounded host-wide scheduler, isolated inner Docker
  resources, scoped build outputs and tags, Compose model and port validation,
  cancellation and recovery, and preflight integration.
- Out: shared DEV or STAGING mutation, global pruning, moving the main checkout,
  and weakening integration tests or treating a skipped test as success.
- Missing fixture behavior must be documented rather than invented.

## Milestones

- [x] Verify a private rootless daemon can host representative Go, Java, and
  Compose clients without the host Docker socket in those clients.
- [x] Measure bind and Linux-backed storage on an isolated source snapshot.
- [ ] Implement a source-bound, run-owned inner daemon and runner lifecycle.
- [ ] Prove recovery after parent crash and uncertain Docker API outcomes.
- [ ] Integrate real selected Testcontainers and Compose checks into preflight.
- [ ] Prove two-run success, failure, and cancellation against a live sentinel.

## Detailed Steps

1. Review the existing ownership journal and slot recovery invariants before
   extending them. Persist intent before each Docker mutation and verify the
   daemon identity, immutable resource IDs, labels, mounts, network and the
   complete security profile before execution or deletion.
2. Add a narrowly typed rootless-daemon profile. Keep the default masked paths;
   expose only the network sysctl subtree required by the tested rootless
   daemon. Do not bind the host Docker socket, use host networking, privileged
   mode, or a writable host checkout. Bind all images and tools to pinned
   identities and record those identities in the journal.
3. Run clients on the owned internal network against only the private daemon.
   Transfer verified source and dependencies into owned storage. Require
   Compose project ownership, safe bind mounts, unique tags, and nonconflicting
   ports; inspect the rendered model before startup.
4. Add recovery tests for every persisted operation state, including process
   death between intent and response. Refuse uncertain ownership rather than
   deleting a resource by a broad label or name. Release a slot only after
   confirmed cleanup, or preserve its recovery marker.
5. Integrate the shared affected plan and require complete test inventories,
   successful commands, no required skips, source/tool/input identity and
   cleanup evidence. Keep the overall preflight incomplete until every selected
   check has a valid receipt.
6. Exercise real Go and Java Testcontainers and a representative Voice Compose
   workload, then run two concurrently with a foreign sentinel. Repeat for
   normal completion, failure, cancellation and parent crash.

## Validation

- [ ] Ownership and recovery tests reject foreign, changed, incomplete, or
  uncertain resources and preserve another run and a foreign sentinel.
- [ ] Representative Go, Java and Compose checks run through the scheduler;
  Docker unavailability and required skips cannot produce PASS.
- [ ] Both host slots are enforced, and cleanup is verified on completion,
  failure, interruption and crash recovery.
- [ ] Source, tool, dependency and generated-output provenance is bound to the
  preflight receipt; changed inputs invalidate it.

## Progress

- [x] Explicit fixture ownership and host-wide slot foundation implemented.
- [x] Real private-daemon feasibility and storage benchmarks recorded.
- [x] Build and exercise a bounded rootless creator with private cgroup
  namespace and selected-field inspection.
- [x] Exercise a Unix-socket-only private daemon with a separate owned client
  and confirm the Docker API is not listening on TCP 2375 or 2376.
- [x] Journal the daemon creation intent and reconcile an already-visible,
  exactly inspected Unix-profile container after a simulated lost response.
- [x] Recover an already-visible, exactly verified resource after a lost create
  response; a real-Docker check preserved another fixture and a sentinel.
- [x] Canonicalize a single verified OCI archive before loading it into a
  journalled private daemon. A real Go Testcontainers client completed and the
  journalled resources were removed. This does not authenticate the caller's
  image identity or establish exclusive socket and directory access.
- [ ] Generic private-daemon ownership, client execution and recovery.
- [ ] Complete local preflight and representative no-skip receipts.

## Decisions

- A separate rootless daemon per run is the leading approach because ordinary
  Testcontainers resources cannot be reliably attributed to the parent run by
  labels alone. Its narrow Docker host profile and recovery still require
  implementation and review.
- Prefer the Unix-socket-only profile for further integration. The owned socket
  volume and its clients need source-bound identity, lifecycle journaling, and
  access controls; a Docker host administrator can still access host resources.
- A local diagnostic is always incomplete until the complete selected plan is
  executed and its evidence is bound; GitHub merge-candidate checks remain
  mandatory.

## Risks And Follow-Ups

- Rootless daemon confinement and host Docker API configuration require an
  independent security review and failure injection before use as a scheduler.
- The earlier TCP profile exposed an unauthenticated endpoint to network peers.
  The Unix-only profile removes that listener but its shared socket volume is
  still accessible to an authorized Docker host administrator. A cooperative
  lock and repeated snapshots cannot exclude other Docker clients. The
  scheduler must derive resource and peer identities from its locked journal,
  verify the actual peers, and bind the socket volume and daemon identity;
  current snapshots do not establish those properties.
- Existing experiments use private dependency seeds whose provenance is not
  sufficient for a complete preflight receipt.
- Some selected integration tests need owner-provided fixture or authority
  work; they must not be silently omitted or converted to successful skips.
