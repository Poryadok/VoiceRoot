# ExecPlan: account-delete activation preparation

## Purpose

Keep the User account-delete consumer disabled while making its durable
`user_db` migration rollout reproducible. This branch does not enable NATS
authentication or the consumer; it records the identity foundation required
before activation.

## Context

- `docs/DEPLOYMENT.md`, `docs/OPERATIONS.md`, `docs/TESTING.md`, and
  `docs/CONTRACT_MATRIX.md` define deployment order, rollback, CI, and
  JetStream delivery.
- `src/backend/user/internal/userevents/jetstream.go` contains the disabled
  lifecycle consumer. Migration `000017_account_lifecycle_search_tombstone`
  supplies its inbox, overlay, and Search tombstone storage.

## Scope

- In: default-off User configuration, User migration Job/order, ClusterIP and
  NetworkPolicy intent, optional Auth/User credential-capable client settings,
  and the external identity-foundation runbook.
- Out: broker auth, credential Secret mounts, `no_auth_user`, shared
  compatibility identities, Compose changes, and staging/prod activation.

## Milestones

- [x] User remains disabled by default and refuses incomplete configuration if
  a later operator explicitly enables it.
- [x] `voice-migrate-user-db` is hash-rerun-safe through the staging migration
  script and precedes User activation.
- [x] NATS Service is explicitly ClusterIP; a policy template records intended
  4222 app and 8222 observability isolation without applying it prematurely.
- [ ] External full per-service JWT identity foundation is provisioned and
  proven before a separate activation PR.

## Validation

- [ ] Hosted User configuration tests.
- [ ] Hosted `scripts/staging/apply-migrate-jobs_test.sh` and CI gate.
- [ ] Hosted manifest/render checks; no local Compose, NATS, Go, or Postgres
  runtime proof is attempted on this host.

## Decisions

- The lifecycle consumer remains `false`; rollback disables that flag first and
  never down-migrates `user_db`.
- `AUTH_NATS_CREDS_FILE` and the User credentials file remain optional client
  inputs so the present anonymous broker continues unchanged. A future JWT
  foundation can use them without another constructor redesign.
- No Search credentials are added because Search is not a direct consumer.

## Risks And Follow-Ups

- NATS Server v2.11.0 rejects `no_auth_user` with `TrustedOperators`.
  `nats.UserCredentials` and Java `credentialPath` require that JWT operator
  mode, so the rejected compatibility shortcut cannot share the broker account.
- A safe activation requires externally provisioned operator/account JWTs and
  one unique service credential for every current NATS publisher/consumer;
  update every client, pre-provision streams, and cut over the broker only when
  all workloads have credentials. Do not use a shared compatibility principal.
- The target secret manager must own NKey seeds, `.creds`, operator/account
  JWTs, and rotation. None may be committed or substituted with placeholders.

### Needs user attention: separate activation PR prerequisites

Before enabling the User flag, an operator must prove in the target cluster:
each service has its own JWT credential; Auth alone gets PubAck for
`user.account_deleted` (and guest/restore subjects); all other identities are
denied deletion core and JetStream publish; User can bind/pull/ACK only its
durable and cannot publish, alter, delete, mirror, or republish into the
deletion subject. Verify `schema_migrations=(17,false)` and lifecycle/Search
tombstone schema before a disabled User rollout, then prove deletion, Search
disappearance, and replay before production canary.
