# ExecPlan: Notification transient NATS retry

## Purpose and context

BE-276 requires the NATS exponential backoff promised by MICROSERVICES.md.
Notification's existing message, moderation, matchmaking, voice, social and
Story consumers share JetStreamConsumeAck, which currently NAKs immediately on
dependency/handler errors. During an outage this repeatedly invokes the same
failed delivery path without any pause.

## Scope and decisions

Use broker-managed delayed NAKs: one second, doubling per broker delivery
attempt, capped at five minutes. This is an internal retry scheduling policy,
not a delivery deadline or attempt limit. Missing metadata uses the first
delay. Compare the unsigned delivery count before any shift or conversion.
The callback must not sleep. Success still ACKs; transient errors never ACK or
TERM, and a disposition failure must not fall back to immediate NAK.

Existing malformed-event TERM handling is unchanged. Durable poison quarantine,
DLQ, alerts, other services, gRPC breakers and the complete BE-276 result remain
open. In particular P3's unlimited-delivery contract is not replaced by a cap.
Named consumer: Notification's currently installed shared acknowledgement helper.

## Steps and validation

1. Add focused tests for scheduling, saturation, metadata failure and success.
2. Add actual embedded JetStream proof that an error does not redeliver early,
   remains pending, redelivers with identical bytes and eventually ACKs.
3. Observe meaningful RED on the existing immediate-NAK implementation in an
   owned Linux container; implement delayed NAK and observe GREEN.
4. Run affected consumer and root regression checks, vet and diff checks.
5. Preserve immutable source/evidence, independent exact-head review and offer
   source publication to the sole develop integrator121. Hosted CI and delivery
   remain separate, with no DEV/STAGING or acceptance claim.

## Progress

- [x] Atomic BE-276 claim and own Treehouse31 lease; immutable base87483dbc.
- [x] Canon, actual callers, CodeGraph and Graphify inspected.
- [x] Real JetStream RED: immediate redelivery unexpectedly succeeded during
  the 250ms absence window; eight delay cases also failed on immediate NAK.
- [x] Focused GREEN: both top-level tests pass in owned Linux, 1.192s.
- [x] Full consumer package: 33 top-level tests and eight subtests pass under
  `-race`, zero skips, 2.181s; `go vet ./internal/consumer` exits zero.
- [x] Existing root message-consumer restart/recovery and participant-fact
  compatibility: two tests pass under `-race`, zero
  skips, 4.172s. This compiles all root callers but is not the full root suite.
- [x] Tagged diagnostic observer retry-to-ACK: initial `-race` check found
  existing global os.Stdout replacement racing the background collector.
  Capture the bounded child process output instead, preserving all routing,
  ACK and scalar assertions. Focused `-race` GREEN: one test, zero skips,
  8.313s under the explicit `voice_compose_fcm_diagnostic` build tag.
  Tagged root `go vet` also exits zero.
- [x] Graphify AST update attempted, stalled and bounded cancellation returned
  terminal exit 1; graph freshness is unverified.
- [ ] Independent source review and accepted delivery continuation.

## Risks and follow-ups

Delayed retry may postpone recovery for a repeatedly failing event by at most
five minutes. NAK transport failure leaves the broker's existing AckWait as
fallback; no new discard path is introduced. This slice does not make delivery
effects exactly once or guarantee poison retention. All test listeners run
inside the owned Linux container, with no host ports or shared stand claims.
