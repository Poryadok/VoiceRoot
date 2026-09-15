# Realtime delivery boundaries

## Purpose and context

Harden three existing A1 delivery boundaries: Role events cannot invent a chat
audience, Chat membership pagination cannot return partial success, and presence
RPCs carry the authenticated connection identity without ambient credentials.

Sources: `docs/PLAN.md` A1, `docs/microservices/realtime-service.md` role_update,
subscriptions and presence, `docs/features/presence.md`,
`docs/features/notifications.md`, `docs/microservices/notification-service.md`,
`docs/ARCHITECTURE_REQUIREMENTS.md`, `docs/TESTING.md`.

## Scope and decisions

1. Route only exact supported Role subjects. Voice overrides and Space metadata
   have no authoritative subscription index and must not use a payload chat ID.
2. Read all Chat membership pages under one bounded deadline. A missing response,
   missing list, invalid member, duplicate member or repeated/cyclic cursor is an
   error with no partial recipient map. A valid empty terminal list stays valid.
3. Presence reads carry target profile and the connection's viewer account,
   profile and account type explicitly. Presence writes carry their account and
   profile explicitly. Ambient identity, privilege and authorization metadata
   cannot override those inputs. User remains the privacy and ownership authority.

Membership-wide notification delivery is not enabled by this change. Existing
Notification RPCs do not return an authoritative routing decision; the NATS
decision/payload contract is absent. A future enabling change must bind that
decision to the event and recipient, preserve mute/type suppression, and combine
complete Chat membership with current per-recipient Chat/Social authorization.

## Steps and validation

- [x] Author regression tests before production changes and review them.
- [x] Push RED tests and obtain hosted Realtime CI failure evidence.
- [x] Implement the three boundaries and preserve valid routing/presence cases.
- [x] Review security, metadata handling, pagination and concurrency.
- [x] Obtain hosted short/full Realtime tests and lint for the implementation.

Final integration is gated by the checks on the current head of PR #397,
including `ci-gate`, and a merge commit with current master. The PR check records
and merge record are the authority for those final integration results.

No local Go executable, tests, race, Compose or localhost server may run for this
task. Test source may use bufconn and embedded NATS only on hosted CI. Regression
coverage includes genuine JetStream subscription fan-out, gRPC wire metadata,
cross-account User policy responses, errors/deadlines and complete/invalid pages.

## Risks and follow-ups

This PR does not prove membership-wide notification delivery, replace User or
Social policy, establish Space voice watchers, or claim the A1 vertical DoD.
Chat pagination is not a versioned snapshot: concurrent membership changes may
cause an error and ephemeral notification loss, reconciled via durable REST.

## Evidence

Tests-only commit `d98f4f4a`, PR #397, hosted CI run `34935487815`:
Realtime short job `104273022050` and full job `104273022096` failed. The short
job log confirms the voice override chat leak, every malformed-page regression,
missing lookup deadline, ambient account/profile precedence and missing explicit
identity rejection. An independent read-only security review accepted these
tests before implementation. Production fixes follow this recorded RED state.

Implementation `09c5aa42` plus normal master merge `77316765` passed hosted
Realtime short tests and full integration (`104274564596`) in run `34936105233`;
`golangci` also passed. A separate implementation/security reviewer approved
the Role routing, pagination, identity stripping, response binding and
concurrency behavior without findings. Test assertions were not weakened.
