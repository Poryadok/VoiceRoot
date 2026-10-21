# BE-264: expiry event recovery

## Purpose and context

Preserve the existing `file.expired(file_id, chat_id?)` event when legacy expiry
commits terminal status before storage deletion, then storage/claim/publication
fails. Canon: File service event table, BE-264 accepted durable deletion contract.
Review finding: PR875-REVIEW-STORY-BOT-FILE-20261021 c1 and Astra480 c6.
Base: 4ea5dbc551aa0abf890d0c745bb8d341e8622808. Owner134 repairs;121 publishes.

## Scope and design

Use a narrow File expiry outbox following the existing music/preview patterns.
Persist exact event envelope in the same transaction as first `expired` status.
Dispatch bounded `FOR UPDATE SKIP LOCKED` rows; only acknowledged publish marks
complete. Retry exact envelope after response loss/restart. Publication follows
logical expiry independently of storage GC. Preserve reference/alias veto and
unchanged legacy retention policy; no new lifecycle/owner scope or P3 activation.

## Steps and validation

- [x] Fresh owner checkpoint and WT33 lease before source writes.
- [x] Write real-PG regression using actual expiry+GC error path: storage failure,
  logical expiry, restarted GC, failed broker attempt, stable successful retry.
- [x] Observe causal RED on original production source: real PG, expected one
  delivered event but zero after R2 failure, terminal status, restarted GC and
  three worker retries (0.52s, exit1).
- [x] Add transaction-bound envelope and retry dispatch in the existing worker.
- [x] Observe GREEN and negative/no-event/migration guards: three real-PG tests
  and two publisher tests PASS, zero skips. A real JetStream accepts a publish
  whose PubAck is discarded, then deduplicates the exact-envelope retry.
- [x] Final private-Linux checks: scoped five tests PASS/zero skips; selected
  lease/lost-PubAck race tests two PASS/zero skips; short suite234 PASS/97 expected
  integration skips, zero failures; `go vet ./...` exit0.
- [x] Lint final: initial staticcheck ST1020 reported an exported-method comment;
  corrected its prefix, final recheck0 issues. No runtime behavior changed.
- [ ] Independent review; deliver immutable commit/bundle/proof to sole publisher121.

Private Linux PG/Go containers on an internal network have no host ports, Docker
socket or deployed environment access. Module ZIP cache is copied into native
Linux storage; no direct host module metadata walk. No GH/CI/DEV/stand actions.

## Risks and decisions

Delivery is at least once; exact event ID/time/body remain stable. An ACK lost
after broker publication can replay; no false acknowledgement on failure. The
new migration must not erase pending intent on DOWN. Noop/NATS-disabled mode
must retain pending events. Existing full-P3 and retention-policy gates remain.

## Progress

Runtime results above are from actual execution, independently of fixture/source
presence. Independent source review and integration acceptance remain pending.
CodeGraph/Graphify are explicitly excluded by current user override relayed by121.

Executed proof files: `tmp/swarm/rudeus134-expiry/red.jsonl`, `green.jsonl` and
`final-scoped.jsonl` outside the product worktree. Linux Go1.26 and PostgreSQL16;
three independent schemas and real PostgreSQL row locks/trigger rollback.
`BE264_TEST_POSTGRES_DSN` optionally selects an isolated test-only database;
without it the tests use the repository's normal Testcontainers fixture. For
the private no-host-port route, run `go test ./internal/grpcsvc -run
'^TestBE264ExpiryEvent' -count=1` with that environment variable. Do not set it
for `-short`: those checks deliberately skip PostgreSQL via the normal fixture.
