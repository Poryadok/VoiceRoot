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

Private resource constraints: own named Docker-in-Docker engine and Go runner,
no host published ports/listeners; runner shares only its own engine network
namespace/socket. Testcontainers PG/Redis/migrator belong to that private engine
and are terminated by cleanup; explicit outer teardown even on failure. No
shared DEV/STAGING resource or peer runtime is acquired or borrowed. No Graph,
GHCI/deployment, producer source changes or duplicate Space186 database test.

If actual canonical qualification requires absent Space publication/current
identity/contribution evidence, report the exact gap and stop before production
changes. A durable Role20 Owner result is not full native45/public Creator proof.

Execution pending; no runtime started at plan freeze.
