# ExecPlan: fresh Auth restore deadline after serialization

Task AUTH-RESTORE-DB-DEADLINE, developer Рудеус-172, gpt-6.1-sol/medium.
Base04f2101656fc2096ac0ad00fd4786c82600ca7b1, branch codex/auth-restore-deadline-172,
own WT36. Found during BE010-IDENTITY-ERASURE-OWNER-CHANNEL; related to BE-019.

## Purpose and sources

An account cannot be restored once its fixed30-day recovery window expires,
including the exact deadline. A transaction opened early must not restore after
waiting on the account row. Sources: docs/features/auth-and-contacts.md recovery
window, docs/microservices/auth-service.md, DATA_MODEL.md,
ARCHITECTURE_REQUIREMENTS.md, TESTING.md, CONTRIBUTING.md and accepted162
owner-channel c4-c9/Q2. No profile grace or new erasure policy is introduced.

## Implementation and decisions

- [x] Deduplicate, separately create and atomically claim this existing A1 bug.
- [x] Reproduce the native database race before the production correction.
- [x] Materialize the exact account row lock before evaluating a strict fresh
  clock_timestamp expiry predicate in the same atomic UPDATE.
- [x] Align the in-memory repository and service precheck at exact expiry;
  preserve strictly-before success and restore event/session semantics.
- [x] Execute focused native and existing restore/race/epoch tests.
- [ ] Freeze/publish source and portable evidence; independent exact-head review.
- [ ] Integrate/CI and environment gates under actual delivery custody.

CURRENT_TIMESTAMP uses transaction start. Merely changing the UPDATE WHERE
predicate to clock_timestamp does not establish post-wait evaluation. The
MATERIALIZED SELECT FOR UPDATE supplies serialization before the outer fresh
clock predicate. Repository row-count behavior and parameter binding stay the
same. InMemoryAccountRepository and AuthService.ensureRestorable use strict
isAfter comparison for the same cutoff. Existing SQL-shape assertions are
updated; correctness is certified by actual PostgreSQL behavior.

Preserve156's separate V27/canonical000028 Moderation lifecycle clock guard.
No migration, issuer, permission, listener, protobuf, OAuth or owner-channel
producer is changed here. Future irreversible terminal guards/recovery/birth
integration remain separate tasks.

## Real test oracles

AuthRestoreDeadlineJdbcIntegrationTest is enabled only with explicit
AUTH_RESTORE_DEADLINE_TEST_JDBC_URL. It requires Linux and applies all26 native
Auth Flyway migrations. It proves ordinary before-deadline restore, refusal
when the transaction-start clock equals the deadline, and two-connection
serialization: A locks the exact account row, B starts before expiry and invokes
the real repository, pg_blocking_pids proves B waits on A, fresh DB time crosses
the fixed deadline, A releases, and B must refuse without changing deleted_at.
Ordinary runs without the explicit external fixture DSN skip this class; that
is not the required native proof.

Initial native RED runner15692 terminal1 at2026-10-19T00:26:16Z:
3run/2 expected failures/0errors/0skips. Both expired cases wrongly returnedtrue;
ordinary allowed restoration passed. JDBC-only GREEN22955 ran3/0fail/0skip.
Its optional whole-class JaCoCo report on the Windows bind mount was explicitly
stopped after preserving Surefire XML; that runner has no whole-build-success
claim. Companion boundary RED84518 terminal1 at00:35:10Z:
6run/2 expected failures/0errors/0skips (in-memory equality and service issuance).

Final GREEN12427 terminal0 at00:38:56Z: BUILD SUCCESS,
15tests/0failures/0errors/0skips. It includes JDBC3, repository3, service expiry3,
restore race2 and restore session epoch4. Main631/test163 compiled during the
initial native run; later runs compiled only the changed Java files and reused
unchanged generated protobuf. Exact final Maven arguments:

    mvn -o -B -Dprotoc.skip=true -Djacoco.skip=true \
      -Dmaven.compiler.useIncrementalCompilation=false \
      -Dtest=AuthRestoreDeadlineJdbcIntegrationTest,AccountRestoreTransitionRepositoryContractTest,AccountRestoreTransitionExpiryTest,AuthServiceRestoreRaceTest,AuthServiceRestoreSessionEpochTest test

Required DSN points to the owned PostgreSQL container over the private Docker
network. Final ByteBuddy dynamic-agent startup was slow; a diagnostic SIGQUIT
of the verified own test JVM produced Surefire native-stream warnings, then
all15 tests completed successfully. The dump is diagnostic, not a test failure
or a fabricated coverage result. No completed coverage/fullAuth suite claim.

## Resources and evidence

Sole WT36 lease860fd1af6cd5718f32a4f33c868e7e8f. Owned network
rudeus172-auth-deadline, PostgreSQL16 container abcbb34795f252df647ca6d4253ee7fa2a843f9ecdfe2281d3b39f90f608637e,
no published ports. Linux Maven3.9.11/JDK25 image
sha256:0f45bb102b13e36ed30cba04675a3a053b13ce6d3a2c93a9b5d0d1efbc66e43b.
Host dependency cache is read-only. Only two public Linux protoc executables
were copied read-only from156's confirmed cache into own tools volume; mode
changed only on own copies. Earlier missing-binary/permission environment
attempts did not execute tests and are not RED.

Logs and RED/GREEN XML live under own tmp/rudeus172, to be preserved in the
external immutable proof package before checkout reuse/return. Graphify55836
is accounted live; no duplicate updater. Direct concrete CodeGraph calls374,
398,401,414 were canceled after31seconds without an answer; current owned
sources were inspected directly. Graph response is not a fabricated proof.

No Windows test executables/listeners, firewall changes, shared stands or
hosted CI runs. Local developer proof is not deployment or acceptance. Source,
CI and delivery remain172 until explicitly accepted ant-to-ant handoff.