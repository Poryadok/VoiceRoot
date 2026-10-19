# P3 migration014 rollback retention correction

Actor Рудеус-160, original author/developer Sol6.1 medium, focused222 c28/c29,
121 explicit no-competing-writer request. Dedicated bug
BE265-P3-014-DOWN-RETENTION claimed e1; parent BE-265 c20. Own WT39 lease,
source foundatione9af126. Consumer121 delivery; independent correction review
required. No production migration, stand, provider or acceptance authority.

Current DOWN unconditionally drops retained grants/targets and safety metadata.
voice-service.md933-940 requires unresolved credential/target/deny evidence to
survive receipt retention. A rollback may proceed only before any native or
lifecycle evidence exists; terminal/expired state is not credential retirement.
Keep014UP and all serving behavior unchanged.

- [x] Dedicated deduplicated bug/found_during relation and atomic160 ownership.
- [x] RealPG retained grant+pending target+receipt regression: fixture canonical
  identity corrected after initial setup conflict, then meaningful RED16.16s:
  actual old DOWN succeeds when refusal was required.
- [x] Add BEGIN/COMMIT and ACCESS EXCLUSIVE locks over all affected native,
  lifecycle head/receipt, target/grant relations before checking evidence.
  Conservative refusal for any rows; refuse while higher native017 is installed.
- [x] ActualPG -race refusal + full row/schema equality, empty DOWN/UP roundtrip,
  blocked uncommitted writer commits before guard evaluation and is retained.
- [ ] Exact source freeze/diffcheck/portable bundle and independent review;
  121 accepted handoff/integration retains full feature/provider gates open.

Actual -race GREEN run21895 terminal0 PASS74.269s, immutable overlayf79ac313.
Actual writer blocking is observed through pg_locks rather than scheduling
sleep. No fixture deletes evidence to satisfy rollback. Higher017 refusal protects
owner work even for an empty native Space; ordinary reverse migration order is
required. Any failure rolls back the entire explicit migration transaction.

Separate owner238 review P2 concerns retained oldENDED-before-newprojection.
Its failing ordered rejoin fixture is a separate source correction and test
sequence; do not reuse014 evidence or prior208 startup CLEAR as its review.

## Independent delivery branch

The same author SQL/test delta is applied in the SAME owned WT39 on
codex/be265-014-down-guard, based on actual local develop
c72bf3720322120806691d1f24b17fed9f145e9c resolved before branch creation. This
branch imports no unreviewed native owner ancestry. Original6f8f6823 evidence
remains immutable; same SQL/test content, fresh full-backend Git archive checks
on this current delivery base follow before independent review/121 acceptance.
