# Existing Role20 Bootstrap persistence proof

Owner183, consumer342/186, c120/c121/e128/c122. Exact base
ea55d287c2f6a4ba078cf3c9bd9b9b2e3ba0211b. NEW test/fixture/plan only;
production Store/core/DDL/main/wire/identity/stopped work excluded.

Plan frozen before runtime:
1. Official source migrate driver to clean Role20 on an isolated PostgreSQL16.
2. Real Redis7, HTTPS JWKS, RSA Space signer and mTLS production server factory
   with actual RoleGRPC/RoleStore. No recorder or alternate authorization callback.
3. Start with no roles; exact signed twoUUID Bootstrap persists five canonical
   system roles, their permission masks/positions and one exact Owner relation.
   Empty response is not a receipt; request UUID is fresh transport binding.
4. Exact spent-token denial, fresh dispatch idempotence, service reinstantiation
   retaining the same durable rows; different Owner cannot replace/add Owner.
5. Existing prepared ownership fence denies Bootstrap; abort restores access.
   Invalid UUID/unknown binding has no effects. No claim of current external
   identity or native21 contribution qualification from these predicates.
6. Inject cancellation at actual member_roles INSERT in a separate traced pool;
   check no partial system roles/Owner survive, then ordinary signed retry works.
7. Preserve exact executed archive/before/after source hashes and raw results.

Private resource constraints CORRECTION before runtime: no privileged DinD,
Docker socket, daemon or added capabilities. Proposed NEW test-only DSN seam:
ordinary owned PG16/Redis7/Go containers on an isolated named network, no host
published ports. Ordinary official pinned migrator container uses exact source
migrations through20 before Go; test verifies clean version20 and actual Store
catalog. Fixture credentials stay private and are not printed. Explicit outer
container/network teardown even on failure. No
shared DEV/STAGING resource or peer runtime is acquired or borrowed. No Graph,
GHCI/deployment, producer source changes or duplicate Space186 database test.

If actual canonical qualification requires absent Space publication/current
identity/contribution evidence, report the exact gap and stop before production
changes. A durable Role20 Owner result is not full native45/public Creator proof.

Exact DSN/resource seam accepted121 c123 before start; earlier DinD plan was
superseded and never run. Source084e08d was archived before execution. Official
migrate4.18.1 goto20 exited0; actual source catalog/clean20 checked in test.
The owned real PG/Redis/mTLS production-handler test passed without skips,
test0.66s/package-race1.702s. No failure was hidden and no actual test rerun needed.

Evidence limit: this proves existing Role20 durable canonical Owner and system
defaults, not authoritative current Space identity/guest/membership/publication,
native21 contributions, full Creator45 assembly, external inactive-owner status
or public POST success. Current source has no such owner observer in this path;
pure signed Space asserts the already committed creation subject. Caller342 must
bind that exact typed committed Space/owner before dispatch. A fresh transport
UUID is not a durable native-operation ID or receipt. No absent predicate is
invented or repaired by Store/core/DDL changes. Prepared-role ownership fencing
is tested; broader existing retirement tests are not relabelled as this run.

Separate default-selection control passed with the expected explicit fixture
SKIP when environment inputs are absent. Additional Role-short attempt failed
at setup only: nats-server/v2 v2.10.24 was not cached and the offline runner
could not fetch it. Other affected package results passed, but no whole-short
PASS is claimed. Its guard did not run vet/build. Raw FAIL is retained; no rerun
was performed. This is a verification limitation, not a hidden actual-PG failure.
