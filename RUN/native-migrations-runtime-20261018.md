# Native migrations runtime adapter — 2026-10-18

## Outcome

Added the independent source-backed `native_migrations_runtime.py` adapter and
focused tests. The adapter consumes the canonical `migrations.py` plan and
keeps the all-window `apply-migrate-jobs.sh` semantics: fixed migration images,
immutable operation ConfigMaps/Jobs, the legacy DSN Secret projection, and
explicit empty plans for `images-only`.

The frozen consumer ABI is:

- `build_request(plan, mode, operation, authority_secret=None)` returns a
  canonical `voice-native-migrations-v1` request for `DATABASE_MIGRATIONS`.
- `apply(api, request, journal_event, revalidate, journal_events=())` accepts
  only typed `get_resource`, `create_resource`, `wait_job`, and `get_job_pod`
  methods and returns a receipt containing resource UIDs, Job UIDs, Pod UIDs,
  and the exact Pod profile digest.
- The typed API caller is fixed to `voice-nats-staging-runner`; migration Jobs
  retain `automountServiceAccountToken=false` and the canonical non-root
  profile. No shell/kubectl fallback is part of the ABI.

Every create is preceded by durable intent and issued events. An uncertain
create gets one read-only exact-object reconciliation and is never retried;
an uncertain wait or Pod read fails closed. No live Kubernetes, Secret, or DB
reads/writes were performed by this slice.

## Evidence

- Direct consumer ACK: `/root/staging_runtime` confirmed the exact plan rows,
  images, refs, bot prerequisite boundary, and create-only lifecycle.
- SSH Linux private test (operator UID 1000, no cluster access): 19 tests passed
  (`native_migrations_runtime_test.py` + `migrations_test.py`) under
  `PYTHONDONTWRITEBYTECODE=1`, `ulimit -v 524288`, and `ulimit -u 64`.
- Windows host checks: 8 focused tests passed with import-only compatibility
  flags, `py_compile` passed, and `git diff --check` passed.

## Source custody

- Branch: `feature/native-migrations-runtime`
- Commits: `20ad0af` (implementation/tests), `71de291` (evidence), plus the
  final profile-projection compatibility fix on the same branch.
- No runtime/shared VAP/transaction/kube API files were changed.

## Remaining integration

`staging_runtime` must add the typed API wrapper and call this adapter from its
rootless transaction path, preserving its existing authorization, journal,
preflight, and post-migration phase transitions. This slice intentionally
does not claim runtime integration or staging rollout readiness.
