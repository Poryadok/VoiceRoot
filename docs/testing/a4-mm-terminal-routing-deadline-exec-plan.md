# Local Matchmaking terminal-routing deadline

This separate A4 slice is stacked on PR954 commit
`289263fc747d25f0420bbb6085f7a6378bd831b5`, including unmerged migration19.
It does not change PR954. Owner: Anton-172; task:
`A4-MM-TERMINAL-RELEASE-ERASURE`. Canon: PLAN A4, DATA_STORES P30D and agreed
`A4-ACCOUNT-LIFECYCLE-PRODUCER-v1` semantic fixture at producer commit6bf8e225.
Auth/User implementation remains its producer owner's scope.

The outcome is local deadline enforcement for bound terminal routing during
Redis outage, delayed delivery/restore and stale-cycle replay. The protected
adapter must supply accepted immutable authority and complete coverage; a valid
digest alone proves neither trust nor enumeration completeness. No wire is
invented. Local ROUTING_ERASED is distinct from Auth ACCOUNT_PURGED.

Progress:

- Causal baseline RED: legacy shared Redis generation retained an affected raw
  session past the original deadline while unrelated control remained.
- Domain and Redis GREEN: original deadline, delayed predeadline restore as
  fresh admission only, newer deletion wins, backup reconciliation denied when
  absent, and exact int64 generation comparison. A precision regression first
  failed at generation9007199254740993 before decimal comparison fixed it.
- Real PostgreSQL RED: a paused Redis callback held the row lock and prevented
  deadline deletion (SQLSTATE55P03). Claims now commit before I/O; independent
  expiry ignores claims, deletes raw routing and clears temporary roster hash.
- Real PostgreSQL GREEN: controlled in-flight deletion, actual database-clock
  expiry, late exact-bound coverage, restarted stale-cycle handling, restored-cycle batch
  fairness and sweeper with Redis/session projection absent.
- Migration20 empty DOWN/UP, concurrent writer refusal and permanent evidence
  refusal pass. Migration19 runner regression remains pinned to its exact
  migration and preserves pending obligations/dirty marker.
- Coverage binding also refuses to guess an absent old-cycle decision when
  current authority has advanced. Both same-row and separate durable-head
  regressions failed before the corresponding guards; fresh post-restore work
  must not inherit an old cycle's deadline merely from profile identity.
- Affected short suites, vet and pinned lint pass. Queue/store/timeout scoped
  race checks and the full domain race selection pass. Final store selection
  (including migration19 runner and migration20 controls) passes in31.169s.
  Graphify and independent source/test review remain required before delivery.

Independent review of historical `ce2fa8f` found three defects: a copied
unbound dispatch could escape later binding, cross-service timestamps wrongly
classified cycle ownership, and expired cleanup bypassed newer-generation
comparison. The repair refuses matching unbound rows under a row lock, removes
timestamp-based conversion/deletion entirely, and compares exact generation
before destructive cleanup. An actual Sweeper/Redis dial barrier and expired
older/newer-same-session requests first failed; deterministic skewed-clock fresh
and delayed-old rows must both remain unqualified. These are safety fixes to
the local seam, not delivery of missing exact-writer/legacy Redis integration.

Evidence logs are retained under ignored `tmp/swarm/anton172-postmatch` in the
main checkout and attached/checkpointed to the task. This plan records limits,
not a runtime or privacy acceptance claim. Protected producer delivery, all new
and legacy writer binding, existing Redis copies, actual restore-before-serving
integration, rollback capability floor and Space/account aggregate erasure
remain activation work. Pending identity has no new retention exemption.
