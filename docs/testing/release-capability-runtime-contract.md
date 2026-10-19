# Owner release capability contract

This is the PL-A4-02 runtime contract accepted in TT c4-c9 (Astra277), alongside
PLAN A4/G1. The v1 offline evidence format remains inventory-only. The Go v2
runtime producer and Notification consumer below do not qualify other owners.

## Policy authority and schema

One immutable mounted JSON policy is parsed into the owning process at startup.
`VOICE_RELEASE_PROFILE=free-invite-alpha` requires `VOICE_RELEASE_POLICY_FILE`
(absolute path) and `VOICE_RELEASE_POLICY_SHA256` (full lowercase SHA256 of the
exact file bytes). The source SHA must equal compiled build metadata supplied
independently by the protected build caller. A runtime SHA environment string is
not build authority. Actual qualification also joins the image/build digest,
mounted config, client artifacts and every live replica in one stable window.

The closed schema has exactly `schema_version` (integer 2), `profile`,
`source_sha` (full lowercase nonzero Git SHA), `capabilities` and `rich_intents`.
The parser rejects duplicates, unknown/missing keys, trailing documents, wrong
types, source/digest mismatch and artifacts over 64 KiB. Its serving state is
private; exported projections are copies. No hot reload or production fallback.

The capability IDs are `phone_auth`, `paid`, `mobile_push`, `public_discovery`,
`unfinished_rich_types`, `federation`; alpha requires all six `disabled`.
`rich_intents` is a complete per-intent table: text/photo/video/document/music
are qualified baseline admission, preserving the existing domain ACL/ready/clean
requirements; voice/sticker/gif/article/location/video_note/send_silent/schedule/
when_online explicitly name `qualified` or `disabled`. Initial qualification
fixtures disable all nine. A policy update cannot manufacture intent completion:
promotion requires the owner's completed candidate-specific proof. Unknown
future intent is denied, and disguised/contradictory content cannot be normalized
to an allowed document/text send. Schedule remains UNIMPLEMENTED where its owner
does not implement it. This package is not that shared Messaging/File classifier.

The synthetic [shared fixture](../../src/backend/pkg/releasepolicy/testdata/free-invite-alpha.json)
uses a deliberately synthetic source SHA; it is for Go/Java/Dart interoperability
tests, not an operator release artifact. The earlier Python v1 manifest has a
different purpose and must not be passed as this runtime artifact.

Development explicitly selects `VOICE_RELEASE_PROFILE=development`, with no
policy/digest or snapshot file variables. It cannot generate alpha evidence.
The local Compose fixture makes that choice explicitly. Alpha configuration
must be supplied before a corresponding image rollout; old configs/pods cannot
qualify. No rollout is authorized by this source contract.

## Required owners and qualification

| Capability | Required enforcement owners / preserved behavior |
|---|---|
| phone_auth | Auth HTTP/gRPC issuance/convert and Flutter projection; preserve email/guest/invite. No phone OTP/session/provider effects. |
| paid | Subscription provider/grant writer; Auth issuance; User, Messaging, File, Chat, Space, Voice and Realtime custom status; Flutter. Cap effective access at policy AND trusted live entitlement AND domain ACL. Preserve free paths, existing facts, reads/cancel/safety cleanup. |
| mobile_push | Notification registration and final provider dispatch, Flutter. Include preexisting tokens and queued/internal/retried work; preserve web push, unregister/settings/email/in-app. |
| public_discovery | Search viewer-bound member-only Space projection before counts/pagination and Space public catalog; Flutter. Preserve ordinary user/contact/member/chat search, templates and invites. |
| unfinished_rich_types | Messaging common send/forward/edit/scheduled/alternate ingress and File purpose admission; Flutter. Before intents/references/insert/events; preserve basic attachments and qualified Music. |
| federation | Actual rendered image/rollout/readiness/alert/network inventory and routing owners/Gateway/Realtime/Flutter; prove absence, keep local flows. |

Gateway/UI project policy; they grant no authority. An old paid JWT, old mobile
token, internal RPC, signed provider event or queued item must still encounter
the final owning guard. Use `FAILED_PRECONDITION/capability_unavailable` (existing
HTTP 412 mapping) for newly guarded actions. Gateway domain-key registration and
Auth REST mapping are required consumer work, not implied by a Notification RPC.
Existing phone contact-sync 409 and UNIMPLEMENTED contracts remain unchanged.

The 18 capability×UI/REST/WS entries are groups of required subassertions, not
18 requests or PASS strings. Use actual compiled UI, direct navigation, current
authenticated/ACL-valid fixtures (valid provider/S2S identity for internal paths),
and live controls before/after. A 401, malformed payload, missing dependency,
unobserved/noop provider or disconnected observer is not capability PASS. Prove
owner state/reference/outbox/provider/fanout absence at a finite processed/drained
correlation barrier; do not infer absence by sleeping. Basic/in-app/control effects
remain allowed and must actually occur under distinct control IDs.

There is no current WS message.send/message_send: prove authenticated
unknown_operation, exact route inventory and no owner/fanout effects. Do not add
WS send to test its rejection. Paid `presence_update.custom_status` is an actual
path and requires Realtime AND User guards; ordinary presence/clear remains.
Mobile suppression is proved after ordinary allowed send/in-app events reach
Notification with existing tokens. Public-discovery tests retain member Spaces
and ordinary user results without foreign IDs/count/cursor leakage. Rich tests
cover explicit enum AND attachments/options/forward commentary atomically.

Runtime reports need a new closed assertion registry/schema, candidate/policy/
registry/image/client hashes, reached owner guard, valid control/fixture result,
exact response, before/after state digests, effect cursor/barrier, allowed and
forbidden effects and portable artifact **content** digests with trusted
provenance. URI/PASS strings from v1 cannot attest that. Missing owner/assertion,
stale instance/policy, swapped artifact, fake/noop adapter or missing barrier is
failure/unknown. All other owner/Java/Dart/UI/runner work remains open.

## Implemented Notification producer/consumer

Notification bootstrap loads the policy before DB/provider/listener work and
constructs both RegisterDevice and PushDispatcher using the same immutable
object. Alpha rejects recording/debug/offline-presence test mode. Canonical
android+fcm and ios+fcm/apns/voip_apns are mobile; web/desktop+fcm are non-mobile.
Unknown platform/provider and conflicting enum/legacy metadata fail closed.
Authenticated mobile registration is rejected before token writes. Unregister
and allowed web registration retain their owner behavior.

Final dispatch rechecks every stored target under the existing lifecycle gate.
Mobile attempts complete as suppressed without provider calls, so old events
can ACK instead of retrying forever; stored tokens are not deleted. APNs/VoIP
adapters are disabled in alpha; the default FCM noop is normalized to absent,
making a web attempt unavailable instead of claiming delivery. Configured real
FCM remains available for web targets. Provider-spy tests are not production
adapter qualification. Existing generic push-sent telemetry alone is not an
effects/readback barrier or proof of external dispatch.

`VOICE_RELEASE_SNAPSHOT_FILE` is a required absolute private file target in
alpha. After actual sender/config resolution and before consumers/listeners,
bootstrap writes it once with O_EXCL/0600. Existing files/symlinks fail startup;
use a fresh process-owned path, read via operator transport and retain the
snapshot only together with live readiness/provenance. No public diagnostic
HTTP route is introduced. Missing DATABASE_URL cannot serve alpha health-only.

Snapshot fields: schema_version/service_id/instance_id/build_source_sha/
policy_sha256/profile/config_projection_sha256/capability_states/guard_ids/
route_inventory_sha256/adapter_modes. Hostname supplies instance identity.
Notification guard IDs are `notification.register_mobile_device.v1` and
`notification.dispatch_mobile_push.v1`. Only those guards are claimed; desired
global capability bits do not attest other services. The config digest hashes
resolved recording/debug flags and actual effective provider modes. The route
digest hashes sorted JSON entries `dispatch:PushDispatcher.Send` plus every
`grpc:/<ServiceDesc.ServiceName>/<MethodName>` from the actual generated descriptor.
Snapshot generation checks the serving object's policy/dispatcher identity.
FCM is `real` only for an actual nonnil HTTPSender, `absent` when unconfigured;
test/noop wrappers cannot masquerade as real. No tokens/URLs/secrets are exported.

The Notification Docker build accepts `VOICE_SOURCE_SHA` and embeds it through
`-X main.buildSourceSHA`. An empty/unbound value cannot start alpha. Current
release image jobs/rollout still need independently resolved build-arg, digest
and mount binding; this source change does not claim they already supply it.
