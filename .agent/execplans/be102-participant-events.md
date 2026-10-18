# ExecPlan: committed ordinary Voice participant facts

## Purpose and boundaries

BE102 c3/c5 defines NATS participant_joined/participant_left for logical active
ordinary DM and standalone group calls. BE106 owns the one committed admission,
membership generation, terminal transition and transactional outbox authority.
Author1 owns ordinarycall/handler/main/migration12; author13 owns this wire,
publisher/encoder, exact NATS subjects and consumer no-effect slice in WT31.
No tentative Redis emission, second journal/outbox, new WS event, Phase0, Space
watcher, GIS, federation, deployment or acceptance activation.

## Milestones

- [x] Atomically claim released task; consume c3/c5 and actual author1 API.
- [x] Agree ParticipantEvent/EncodedParticipantEvent constructor API with1.
- [x] Canonical transition matrix and additive proto20/21 + generated Go/Dart.
- [x] Stable-ID/time encoder and durable publisher wire tests: meaningful RED
  on stub; GREEN22 tests including mismatched/malformed payloads. Scoped lint0
  issues, buf lint/format/breaking against localmaster PASS. No remote proof.
- [ ] Exact subject/ACL/bootstrap preservation and consumer ACK/no effects.
- [ ] Integrate encoder through BE106 committed EventEncoder adapter (author1).
- [ ] Real isolated NATS/bootstrap/ACL and journal integration proof, independent
  review, exact-head checks and ordinary PR delivery when transport recovers.

## Contract and verification

Group start/join commits joined only for new committed membership; no-op,
tentative admission or failed compensation emits none. Leave emits left once;
last leave does not duplicate terminal left. Whole-call terminal leaves the
remaining committed snapshot. DM ringing emits none; accept atomically joins
both; active end/leave leaves both once. Rejoin has new generation/IDs.
Post-commit failure preserves persisted intents for recovery. No exactly-once or
global network-order promise. Existing legacy events keep semantics.

Payloads contain room_id/profile_id only, envelope event_id/occurred_at. New
oneofs20/21 use voice.participant_joined/voice.participant_left on voice_events.
Realtime/Notification/Analytics ACK unsupported facts without external effects.

Generate canonical Go/Dart stubs, buf lint/format/breaking against cachedbase698.
Test stable metadata/bytes and subject/oneof binding, invalid inputs, PubAck/error
semantics; never regenerate committed IDs/time. Current enumerated NATS lists
change additively; immutable deployed-baseline fixtures remain historical.
Reuse author1 journal concurrency proof at exact source. Real NATS/PG require a
faithful environment. Native Windows Realtime execution remains prohibited.

## Custody

WT31 lease6674bd5dbc23cdb682b4a3b0297990a8, branchcodex/be102-participant-wire
base698d4f0. BE130 source9c2623177 is preserved on codex/be130-audit-ingestion;
Graphify15902 deliberately interrupted after obsolete AST183/183, no live command.
BE130 final graph/caller/review/PG delivery and BE247 PR874 remain dormant owner13.
No equivalent GitHub outage probes. TT owns task facts and pending continuations.
