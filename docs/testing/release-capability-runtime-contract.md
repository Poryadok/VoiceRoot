# Owner release capability contract

This is the PL-A4-02 runtime contract accepted in TT c4-c9 (Astra277), alongside
PLAN A4/G1. The v1 offline evidence format remains inventory-only. The Go v2
runtime producers below qualify only their named guards, not other owners.

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

## Source-only deployment wiring

Production and staging application manifests now require a bound release input.
This is static wiring, not image/runtime/rollout qualification. No final policy
bytes are committed: generate them only after the protected final candidate
freeze. Any source change requires a new candidate binding and policy digest.

The trusted stdlib helper is
`scripts/staging/nats-rollout-preservation/release_policy_wiring.py`. Production
`render-and-apply-prod.sh` and `apply-app-manifests.sh` require
`VOICE_RELEASE_POLICY_INPUT` (a local closed JSON descriptor). The outer caller
validates before any infrastructure/gateway/migration dispatch; the app helper
validates/renders before its first cluster command.
Captured staging `compiler.py` and `source_plan.py` accept the same descriptor
as the explicit `release_policy` parameter; target code and environment variables
do not supply that protected input. A later authorized root caller must carry it
through its approved source/parameter provenance. Missing input blocks new
source; no root authority, preservation gate or historical old-source check is
bypassed. Older source without this new placeholder contract stays readable.

The descriptor has exactly `profile`, `source_sha`, `policy_sha256`,
`policy_object`, `owners`. Profile is only `free-invite-alpha`; source must equal
the protected caller's candidate/tag. `policy_object` is a complete external
immutable v1 ConfigMap descriptor with exactly apiVersion/kind/metadata/immutable/
data. Metadata is exactly name/namespace; data has one validated key containing
the exact UTF-8 v2 policy string. Its SHA256 and source binding are verified,
with six disabled capabilities and all nine restricted intents disabled. This
source stage neither creates that object nor attests its actual live content.

`owners` has exactly auth/user/notification/realtime, each with caller-provided
`policy_directory` and `snapshot_file`. The policy directory is absolute and
mounted read-only, with the supplied ConfigMap key mapped to `policy.json`.
All four variables are explicit on each application container, including Auth.
Snapshot files are direct `/tmp/<filename>` children, on the container writable
layer, without a covering volume or read-only root filesystem. The four current
images have direct service/JVM ENTRYPOINTs; a service exit restarts its container
and its writable layer. Persistent emptyDir/PVC snapshots or merely PodUID-unique
filenames cannot provide freshness across a container restart. Runtime O_EXCL /
CREATE_NEW and 0600 creation remain authoritative. Semantic CI rejects entrypoint
overrides, covering mounts and read-only roots; actual image filesystem and
restart validation remain live HOLD. No actual mount path is selected for an
environment by this source contract.

`make release-policy-wiring-test` checks synthetic offline inputs and both
rendered environments, and is included in `ci-script-tests`. Live image tuples,
actual immutable objects/mounts, snapshot readbacks and an authorized rollout
owner/resource claim remain separate requirements. Static GREEN closes only
the missing source wiring finding; it cannot attest the 18 qualification groups.

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

## Implemented User and Realtime custom-status boundaries

User and Realtime load the same strict v2 policy schema before dependency startup.
Their Dockerfiles embed the independently supplied `VOICE_SOURCE_SHA`; local
Compose explicitly selects development. Alpha requires the private exclusive
snapshot target. User requires PostgreSQL and presence Redis; Realtime requires
the actual gRPC User updater. Snapshots are written before listeners/consumers,
bind the actual serving policy pointer, and name only
`user.update_profile_custom_status.v1`, `user.update_presence_custom_status.v1`
or `realtime.presence_update_custom_status.v1`. User's route digest uses the
generated User service descriptor; Realtime inventories its implemented HTTP
and inbound WS operations. Config projections record the resolved nonsecret
profile/presence store or User-updater modes. No token/endpoint is exposed.

With alpha paid disabled, an authenticated nonempty custom status returns
`FAILED_PRECONDITION/capability_unavailable` at User before profile/presence
writes or events, retaining the owner-before-capability ordering for a profile.
Realtime returns WS `error` with `code=capability_unavailable` before its User
call and local/Redis fanout. It also stops fanout after a failed authoritative
User update. The production gRPC adapter preserves omission versus explicit
empty clearing. Ordinary presence, clearing, free profile edits and existing
reads are preserved. Development retains the current premium domain checks;
this does not implement or qualify future live revisioned A7 entitlement grants.

Linux regression fixtures use PostgreSQL, Redis protocol/miniredis, actual WS
connections and a direct User owner fixture. A processed error response plus
the next allowed observer/Redis event forms the finite effect barrier. These
are source/component proofs, not the full 18-group UI/REST/WS qualification,
live provider/replica/image binding, DEV/STAGING or acceptance.

## Implemented Notification producer/consumer

Notification bootstrap loads the policy before DB/provider/listener work and
constructs both RegisterDevice and PushDispatcher using the same immutable
object. Alpha rejects recording/debug/offline-presence test mode. Canonical
android+fcm and ios+fcm/apns/voip_apns are mobile; web/desktop+fcm are non-mobile.
Unknown platform/provider and conflicting enum/legacy metadata fail closed.
Authenticated mobile registration is rejected before token writes. Unregister
and allowed web registration retain their owner behavior.

Final dispatch rechecks every stored target under the existing lifecycle gate.
After BE195 composition it also checks target eligibility inside that gate
**before** the lazy quota/grouping callback; suppressed/invalid mobile targets
spend nothing. Allowed web targets retain BE195 logical admission and retries.
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
digest hashes sorted JSON entries `dispatch:PushDispatcher.Send`,
`dispatch:PushDispatcher.PrepareAndSend` plus every
`grpc:/<ServiceDesc.ServiceName>/<MethodName>` from the actual generated descriptor.
Snapshot generation checks the serving object's policy/dispatcher identity.
FCM is `real` only for an actual nonnil HTTPSender, `absent` when unconfigured;
test/noop wrappers cannot masquerade as real. No tokens/URLs/secrets are exported.

The Notification Docker build accepts `VOICE_SOURCE_SHA` and embeds it through
`-X main.buildSourceSHA`. An empty/unbound value cannot start alpha. Current
release image jobs/rollout still need independently resolved build-arg, digest
and mount binding; this source change does not claim they already supply it.

## Implemented Auth phone owner guard

Auth parses the same closed v2 artifact in an early bean-factory postprocessor,
before ordinary database/provider/listener beans. Its Docker build writes the
protected caller's `VOICE_SOURCE_SHA` build argument into the packaged
`voice-build-source.sha` resource. Runtime environment source strings cannot bind
an alpha artifact. The Auth image CI job binds this argument to its protected
`github.sha` context. Compose, the Auth image smoke and test fixtures explicitly select `development`;
an operator alpha configuration needs the bound policy and a fresh absolute
`VOICE_RELEASE_SNAPSHOT_FILE` target. There is no implicit production fallback.

The same immutable object is installed once on both AuthService and OtpService.
Nonempty phone input, including mixed email+phone input, fails before account,
password, session, OTP throttle/code or provider effects at register/login and
OTP send/verify. Conversion first preserves current authenticated anonymous-guest
checks, then denies before password hashing or account/session effects. Old
phone-only accounts cannot refresh into a replacement session or complete 2FA
session issuance; refresh denial preserves the unconsumed credential. Profile
switch denial occurs before floor preparation or access revocation and preserves
existing access and refresh credentials. Restore denial occurs after validating
the restore token/account but before floor preparation, token consumption,
account restoration or restored-event publication; the token remains reusable.
Email profile switches and restores retain their normal issuance behavior. The final
session writer also guards phone-only issuance. These failures map to HTTP 412
and gRPC `FAILED_PRECONDITION`, with `capability_unavailable`. Existing email,
guest, email OTP/reset, reads, logout and safety cleanup retain their behavior;
phone contact-sync and unimplemented methods are unchanged.

Before lifecycle listeners start, Auth verifies serving-object identity, real
JDBC account/refresh/OTP repositories, the configured Resend sender and absence
of TOTP test bypass. It creates a private exclusive 0600 snapshot without
following existing symlinks. The snapshot joins build/policy/config/route digests
and actual adapter modes; route inventory includes the generated gRPC descriptor,
registered REST mappings and direct owner guard entries. Guard IDs are
`auth.register_phone.v1`, `auth.login_phone.v1`, `auth.convert_guest_phone.v1`,
`auth.refresh_phone.v1`, `auth.verify_2fa_phone.v1`, `auth.issue_phone_session.v1`,
`auth.send_otp_phone.v1` and `auth.verify_otp_phone.v1`.

These are source and local component checks. They do not qualify a provider,
running alpha image/replica window, Flutter projection, other paid Auth benefits,
the full 18 groups, shared environments or acceptance.
