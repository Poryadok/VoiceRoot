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
- [ ] Generic private-daemon ownership, client execution and recovery.
- [ ] Complete local preflight and representative no-skip receipts.

## Decisions

- A separate rootless daemon per run is the leading approach because ordinary
  Testcontainers resources cannot be reliably attributed to the parent run by
  labels alone. Its narrow Docker host profile and recovery still require
  implementation and review.
- A local diagnostic is always incomplete until the complete selected plan is
  executed and its evidence is bound; GitHub merge-candidate checks remain
  mandatory.

## Risks And Follow-Ups

- Rootless daemon confinement and host Docker API configuration require an
  independent security review and failure injection before use as a scheduler.
- Existing experiments use private dependency seeds whose provenance is not
  sufficient for a complete preflight receipt.
- Some selected integration tests need owner-provided fixture or authority
  work; they must not be silently omitted or converted to successful skips.
