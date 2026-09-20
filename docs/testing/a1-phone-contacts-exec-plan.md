# ExecPlan: A1 Phone-Contact Pipeline

## Purpose

Replace the Contacts-tab empty-hash sync stub with a consented, privacy-safe
phone-contact discovery flow. The client must only transmit the canonical
opaque hashes required by Social, show truthful permission, empty, failure, and
success states, and refresh the active profile's contacts after a successful
match.

## Context

- `docs/todo/client.md` defines the current gap: platform contact permission
  and a hash picker must replace `syncPhoneContacts(const [])`.
- `docs/features/auth-and-contacts.md` requires phone-book permission and
  reciprocal opt-in to phone discovery; it separately defines the phone-search
  and DM privacy controls.
- `docs/features/friends.md` requires matching Voice users to become contacts
  and gives contacts the DM-request bypass.
- `docs/features/privacy.md` makes User Service the privacy source of truth.
- `docs/microservices/social-service.md` requires protected, request-bound
  Social-to-User privacy calls that fail closed.
- `docs/features/platforms.md` and `docs/PLAN.md` make Web and Windows the
  current alpha clients; Android/iOS are after stabilization and their beta is
  a later planning gate. The platform document requires unavailable features to
  be hidden or explained.
- Current code: Flutter has no contact-platform adapter; `SocialPanel` calls
  `syncPhoneContacts(const [])`; Gateway forwards opaque `hashed_phone_numbers`;
  Social applies privacy before inserting `phone_sync` contacts.

## Scope

- In: an honest A1/G1 capability boundary, stable direct-call rejection,
  hidden Web/Windows action, tests, and truthful documentation/TODO status.
- Out: raw-contact upload/storage/logging, QR camera scanning, phone
  registration/OTP, or any mobile phone-book protocol implementation.
- Deferred contract: post-alpha/G3 mobile uses E.164 normalization, a
  versioned digest, server HMAC token, `PhoneDiscoveryMode`, staged sync,
  quotas, and redacted failures.

## Milestones

- [x] Document the current contract and identify the blocking input-format gap.
- [x] Obtain the owner decision: real OS phone-book sync is post-alpha/G3;
  Web/Windows hide it and direct calls are `FAILED_PRECONDITION` / HTTP 409
  `phone_contact_sync_unavailable`.
- [x] RED: add focused UI and Gateway rejection tests.
- [x] GREEN: add the smallest Social/Gateway guard and remove the empty client
  action while preserving manual/contact/friend flows.
- [ ] Run hosted exact-head CI after the PR is pushed and merge after `ci-gate`.

## Validation

- [x] Focused Gateway rejection test proves stable 409 capability-unavailable
  semantics; Gateway short suite and Social short suite pass.
- [ ] Focused Flutter widget test: blocked locally because the lease lacks the
  sqlite3 Windows native asset; defer this check to hosted CI.
- [ ] Hosted PR CI at the pushed exact head, including `ci-gate`, is green.
- [ ] Multi-account proof confirms the active signed profile alone owns added
  contacts and denied audiences do not match.

## Progress

- [x] Reviewed the feature, privacy, Social-service, plan, TODO, testing, and
  contribution sources plus the post-#401 code path.
- [x] Confirmed the backend already fails closed when privacy lookup is absent
  and Gateway forwards the signed caller context.
- [x] Mapped existing test seams: `friends_client_test.dart` owns hashed-only
  request serialization; `social_panel_test.dart` owns the action feedback;
  `social_contacts_integration_test.go` owns privacy and active-profile contact
  persistence. The current localizations only describe unsupported and matched
  states, not consent, permission denial, cancelled selection, or no-contact
  outcomes.
- [x] Resolved owner decision; RED confirmed Gateway's prior 500 mapping.
- [x] GREEN confirmed the Gateway 409 domain response; no hash is read after
  the signed-caller check and the Contacts tab exposes no sync action.

## Decisions

- Do not infer a client hash algorithm or add a raw-contact package in A1/G1:
  mobile protocol work is explicitly deferred.

## Risks And Follow-Ups

- Future mobile implementation still needs a versioned API and exact quotas,
  retry policy, and redaction behavior before enablement.
