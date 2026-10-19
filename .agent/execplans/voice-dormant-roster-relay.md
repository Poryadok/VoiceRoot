# Keep the unqualified native roster relay dormant

## Purpose

Ordinary Voice and existing Space media configuration must not start the new A3
roster relay before its broker authority and durable consumer are qualified.

## Context

TT bug `VOICE-A3-UNQUALIFIED-RELAY` is linked to `SWARM-DEVELOP-20261018`.
The latter's independent Astra decision c28/c29 identifies the unconditional
`runRosterOutbox` call in `src/backend/voice/main.go`. The current canonical v5
does not qualify A3 publication. `docs/microservices/voice-service.md` defines
native facts and restrictive P3 retention; source qualification is not rollout.

## Scope

Remove only the new roster relay from production composition. Preserve native
transactional facts, counters, pending outbox rows and fixed envelopes, existing
admission delivery and restrictive P3 reconciliation. Keep isolated relay tests.
Repair six existing hosted Voice lint findings without changing active behavior.
Migration 014 retention repair is owned separately by its original author.

## Milestones and detailed steps

1. Add a regression inspecting the actual production root, rejecting any roster
   relay reference for every configuration and retaining compatibility Run calls.
2. Preserve a real RED, remove the startup call, then run the regression GREEN.
3. Run affected Linux checks/lint; reuse unchanged native PG evidence only with
   exact input comparison. Record any uncovered composition proof explicitly.
4. Obtain independent source review, publish and follow exact-head hosted CI.

## Validation and progress

- [x] Root regression meaningful RED on c72 production wiring under Linux Go
  1.26.5: one test failed at the actual `runRosterOutbox` identifier; exit 1.
- [x] Root regression GREEN after removal of that startup call: Linux Go1.26.5,
  one test, zero skips, package 0.021s; both owned runners removed successfully.
- [ ] Voice lint and affected tests.
- [ ] Applicable native/P3 PG evidence and production composition qualification.
- [ ] Independent review and current-head CI.

The exact root regression is an exhaustive source composition check across
configuration branches, not a real broker/provider or full process startup test.
Affected short package checks and repository-pinned Linux lint run separately;
complete c29 native/P3 source qualification remains explicitly open.

## Decisions

Use no operator boolean or v5 readiness label to claim A3 qualification. Removing
the sole startup call makes the new path dormant across ordinary and Space
configuration, while retaining the isolated implementation.

## Risks and follow-ups

Complete source checkpoint remains blocked on separately proved retention-safe
014 DOWN and full source closure review. A3 activation requires the future v6,
scoped rights and actual consumer/inbox proof. No deployment or acceptance here.
