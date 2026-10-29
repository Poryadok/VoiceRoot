# Scheduled owned Docker fixtures

`scripts/ci/local_docker_scheduled.py` combines the two-slot local heavy-run
lease with `OwnedRun`. A caller supplies the checkout, Docker engine and the
same trusted host-local slot directory used by every participating checkout.
Within a `ScheduledRun` context, create resources through the returned
`OwnedRun`. On exit, the owner validates and removes its exact journalled
resources before releasing the slot. A body exception still triggers cleanup;
an uncertain cleanup retains the busy slot and journal for reconciliation.

This API covers only resources created by `OwnedRun`. It is not a general
runner for Go or Java Testcontainers, Compose, image builds or arbitrary child
processes. It does not yet recover abandoned slots, enforce a shared directory
across independent callers, or provide mandatory local preflight. Do not clear a
busy marker merely because its original process exited: resources or child
processes may still be active.

The opt-in real-Docker check is
`VOICE_LOCAL_DOCKER_TEST=1 python scripts/ci/local_docker_scheduled_integration_test.py`.
It creates explicitly owned disposable Redis fixtures and a separate sentinel.
