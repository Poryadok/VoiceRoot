# ExecPlan: durable User last seen (BE-050)

## Purpose

Keep viewer-filtered last-seen timestamps after Redis expiry or restart.

## Context

`features/presence.md` and `microservices/user-service.md` require PostgreSQL
last_seen_at, written at heartbeat/session boundaries and read through existing
show_last_seen enforcement. PresenceStore currently owns Redis only.

## Scope

Add a User-owned timestamp table, monotonic writes, batch reads, production
wiring and finite store/SQL integration tests. Preserve current privacy checks,
live status semantics, event publishing and authenticated profile resolution.
No deployment or acceptance is authorized.

## Milestones and detailed steps

1. Add injectable durable-store seam and failing expiry/failure tests.
2. Persist every heartbeat before Redis acknowledgement; monotonic PG upsert
   avoids older requests moving last seen backward. Merge durable timestamps
   for single/batch snapshots, preserving live fields and privacy filtering.
3. Wire production with its existing User PG pool; add expand/down migrations
   and PostgreSQL round-trip/ordering coverage. Update storage documentation.
4. Run User short tests, vet and lint; obtain independent privacy/migration
   source review and real PostgreSQL CI evidence before integration.

## Validation

Focused tests cover expiry past 30 days, restart, single/batch parity, live
field preservation, offline heartbeat, durable failure denying acknowledgement
and absence of stale cache fallback on durable read failure. Real SQL test
covers monotonic concurrent/older writes and migration round-trip.

## Progress

- [x] Canon and current code inspected; BE-050 atomically claimed.
- [x] Meaningful RED then GREEN, production wiring and migrations.
- [x] User short suite 221 tests, 8 focused presence race tests, vet and golangci PASS.
- [ ] Independent review and exact-head PostgreSQL CI/merge after transport recovery.

## Decisions

Persist each heartbeat, not only graceful disconnect: the service spec names
heartbeat boundary and session TTL expiry has no reliable disconnect callback.
No Redis backfill fabricates historical observations; retained legacy cache can
still supply an existing interim value during transition.

## Risks and follow-ups

PG failure prevents a successful heartbeat; Redis failure can leave a durable
timestamp without a live session, which records real observed activity only.
Existing Redis status/event semantics stay authoritative for live routing.
Real PostgreSQL/runtime proof and independent review are required; local short
tests alone cannot establish migration correctness or product acceptance.
