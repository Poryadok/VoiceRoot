# Scheduled owned Docker fixtures

`scripts/ci/local_docker_scheduled.py` combines the two-slot local heavy-run
lease with `OwnedRun`. `ScheduledRun.start_host` uses the common host-local
slot directory for participating checkouts; the lower-level `start` requires
callers to supply the same trusted directory themselves.
Within a `ScheduledRun` context, create resources through the returned
`OwnedRun`. On exit, the owner validates and removes its exact journalled
resources before releasing the slot. A body exception still triggers cleanup;
an uncertain cleanup retains the busy slot and journal for reconciliation.

This API covers only resources created by `OwnedRun`. It is not a general
runner for Go or Java Testcontainers, Compose, image builds or arbitrary child
processes. `recover_host` can reconcile a journal-bound abandoned owned run;
uncertain resource operations that it cannot reconcile retain the slot. This
does not provide mandatory local preflight. Do not clear a busy marker merely
because its original process exited: resources or child processes may still be
active. Only cooperating callers use these slots.

`local_compose_audit.py` rejects known unsafe settings in a rendered Compose
model, including host-network or privileged builds, forwarded SSH or build
secrets, insecure entitlements, and Dockerfiles outside their build context.
Passing this audit does not start a stack, establish ownership, or prove cleanup.

The opt-in real-Docker check is
`VOICE_LOCAL_DOCKER_TEST=1 python scripts/ci/local_docker_scheduled_integration_test.py`.
It creates explicitly owned disposable Redis fixtures and a separate sentinel.
For pull requests, the shared affected plan selects the gated
`local-docker-isolation` job for the explicit ownership, scheduler, audit and
session-helper inputs listed in `.github/ci/path-filters.yml`, as well as broad
global changes. The job checks the Go session helper and exercises scheduled
fixture cleanup and owned-fixture failure and termination on a Linux runner.
It does not establish isolation for arbitrary Testcontainers or Compose runs.
