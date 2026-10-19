# Per-rollout NATS preservation

## Fixed A3 roster authority v6

The separately versioned `voice-nats-additive-contract-v6` migration is
`chat-space-social-role-media-music-a3-roster-v6`; its census receipt is
`voice-nats-additive-census-proof-v5`. It selects the complete immutable
four-template owner147 tuple, adds only `voice.roster_membership_changed` to
voice13 and the exact pull/all/explicit `matchmaking_voice_roster_v1` durable.
The exact source bytes are held as qualification fixtures in
`testdata/a3-roster-source`; this work does not install those templates or keys.
Every v3/v4/v5 literal source, declaration, API and proof pairing remains fixed.

Fresh `voice-bootstrap-auth-v5` evidence contains exactly five ordered normalized
rows. Its full four-row Authv4 projection must equal the immutable enrollment;
existing enrollment is never replaced. Actual installed bootstrap, Voice and
MM permissions remain a separate pre-fence gate. Signed disposable credentials
qualify only their fixture policy, never the installed identities.

Before mutation, `voice-nats-deliver-all-cut-v1` binds the original archive,
manifest/census, complete stream state, operation/plan/enrollment/live-auth and
the exact retained matching-subject count from authenticated filtered INFO on
an isolated restore. The selected store is never pulled or ACKed. A newly
created durable must prove native original absence, an owned durably issued
and closed CREATE attempt, checksum-validated native Created and exact native
state. For original first sequence F and last L, S is 1 when F=0 and otherwise
min(F,L+1): CLOSED Delivered is (0,S-1), AckFloor (0,0), pending is independently
counted M, with no activity, waiting, native pending or redelivery. Matching
configuration alone cannot recover a creation. An already-existing durable
retains its exact old state instead of being reinitialized.

Only successful preservation permits the existing four guarded ConfigMap
CAS/readbacks and serialized v6 active receipt. Broker delivery and lost-ACK
checks use separate post-proof clones. Actual product MM/PostgreSQL inbox
commit-before-ACK and duplicate-effect evidence is owned by147/145 (C4/C5),
and remains open until those consumers supply it. Producer relay activation
also requires accepted installed v6 authority and product consumer gates.

## Fixed participant/Auth observation authority v5

The v5 subject expansion is a distinct migration,
`chat-space-social-role-media-music-participant-auth-observations-v5`, with plan
schema `voice-nats-additive-contract-v5` and census proof
`voice-nats-additive-census-proof-v4`. It retains the exact four v4 consumer
declarations and unchanged `voice-bootstrap-auth-v4` enrollment semantics.

Root selects one complete fixed four-file tuple: realtime, analytics-chat,
search and notification. All four target script hashes come from those pinned
files; no per-file old/new mixture, arbitrary caller hash or pin refresh is
accepted. Voice declarations must agree in realtime/analytics/notification;
user declarations must agree in realtime/analytics/search. The preexisting
historical contract and previous receipts remain byte-identical. The fourth
notification source is required because its strict stream comparison must
recognize the same expanded voice subjects; omitting its root-derived hash
correctly fails the compiler.

Only complete old or target sets are eligible: voice 10 or 12 subjects and user
10 or 14, independently. V5 adds exactly `voice.participant_joined`,
`voice.participant_left`, `user.registered`, `user.logged_in`, `user.logged_out`
and `user.2fa_enabled`, using exact stream UPDATE APIs. It deep-copies the live
config and changes only the ordered missing subjects. Missing streams, partial
sets, duplicates, foreign ownership/wildcards or unrelated config drift fail.
Dynamic and declared old consumers are preserved without recreation.

V5 execution, actor INFO/WRITE, issued-intent recovery and receipt readers admit
these two extra stream targets only for v5. Old v3/v4 plans cannot acquire their
authority. V5 receipts require all four exact source digests; the unpublished
three-source prototype is invalid. After semantic/native preservation proof,
custody performs the same UID/RV/operation-annotation guarded CAS for all four
ConfigMaps. A partial or foreign fourth CAS cannot authorize the next rollout;
the serialized complete receipt and next compile must produce a no-op.

The isolated populated fixture checks pending voice/user consumers including
`notif_voice`, old message/index and consumer state bytes, actual interruptions
after both new UPDATEs, all six new subjects and no-op repeat. Exact UPDATE
grants are exercised with the existing disposable authenticated identity on an
empty scratch realm before mutating its populated store. This does not modify
installed credentials, renew permissions, run a bootstrap Job or grant stand
access. It uses the repository's exact pinned NATS image; its result is separate
from hosted CI, helper installation, rollout and appointed acceptance.

Normal staging deployment uses a root-owned captured bridge installed once by
the operator. Every subsequent release creates a new operation and cold cut;
an earlier reset baseline or successful backup cannot authorize another release.
The selected PVC/PV UID and physical path remain bound throughout the operation.

### Explicit baseline restoration after media startup failure

The separate, exact `0e2be62a3326` original-application restoration failed during
startup and was refenced. Its original checkpoint, failed restoration checkpoint,
private progress, compatibility note and absent finish response remain unchanged
evidence. The frozen original-image runner must not be rerun.

The reviewed alternative establishes a `VERIFIED_GATED_TARGET_BASELINE` using
current master source captured through the existing CI/image-lock authority,
canonical rendered manifests and dedicated
[media principal material](../../../deploy/staging/media-principal.md). It retains
the existing Hub/PVC, 45-consumer contract and all 16 completed database Jobs;
neither NATS nor database migrations are replayed. Before releasing the marker,
it proves the exact 22 target templates/UIDs and readiness, all 19 signed service
actors and 18 live leaf connections, and restores the owned User/Space startup
override. A bounded private error/status/container-log capture precedes refencing
if startup fails; public output contains only fixed stage and safe error fields.
The immutable prepare record binds the canonical full normalized resource set;
both apply admission and terminal verification reject any subsequent change,
including non-Deployment configuration, even if closure hashes are recomputed.

The single terminal remains `ABORTED`, with deployment PASS and rollback
eligibility both false. It admits the existing `--install --upgrade-idle-media-v1`
transition from the exact retained 1295 helper. A fresh ordinary populated FULL
and actual application/principal checks provide new-build acceptance separately;
the ignored recovery runner is not a permanent installed CLI.
Any nonzero recovery result after mutation requires stopping and reconciling the
saved private progress manually. Do not rerun it: the exact checkpoint admission
refuses a changed or partially restored state.

A request reported as `BLOCKED` may already have completed migrations or other
side effects. Correlate its journal, checkpoint and resource identities before
choosing recovery; retain private failure evidence before cleanup or refencing.
Passing source/image gates does not prove runtime readiness or product usability.
Revalidate current source authority and the one-hour prepared binding at execution;
never reuse expired authority. The [deployment diagnostics runbook](../../../docs/DEPLOYMENT.md)
provides the bounded status/report commands for that investigation.

The subsequent target apply stopped after 22 Service/NetworkPolicy responses and
an Auth Deployment CAS: Kubernetes added its controller-owned
`deployment.kubernetes.io/revision` annotation after the template changed.
Canonical resource semantics exclude that top-level annotation only on
Deployments; application annotations and the complete Pod template remain bound.
An absent completion event does not prove that its preceding CAS had no effect.

The exact failed-apply successor uses the distinct
`operator-media-target-reconciled` evidence namespace. It preserves the original
target files, failed checkpoint and frozen runner, reconciles every resource
against the retained before/target inventory, and obtains fresh source/CI
authority. The canonical terminal verifier checks this fixed boundary and rejects
cross-schema replay, mixed partial states and changed retained evidence. Admin's
three outside-owned objects remain preserved; their updated semantic digests are
derived from retained descriptors, never repinned from live state. Ordinary FULL
still includes Admin normally. Capture, cleanup and refence outcomes are recorded
separately: a cleanup error cannot discard a completed refence proof, and zero
replicas alone cannot establish that proof. Historical UNKNOWN is not rewritten
merely because a later read-only physical fence check succeeds.

Recovery inventory fixtures must use independently recorded literal resource
names. A successor admission initially omitted `voice-` from six NetworkPolicy
names; tests generated their inventory from that same incorrect constant and
therefore missed the mismatch. The corrected regression uses all 22 names from
the retained failed progress, including their prefixes and order, and rejects
missing-prefix aliases and reordered events. That admission refusal created only
the byte-identical retained checkpoint and original-runner copies; these files
are preserved and accepted only when `retain()` confirms exact existing bytes.

The subsequent target restart reached readiness for 21 workloads, but Gateway
refused startup because its staging manifest omitted
`GATEWAY_SESSION_EPOCH_STRICT=true`. Compose's wiring test already required this
flag; it did not cover the staging rendered target. Canonical staging compilation
now requires the explicit strict value, existing Redis ConfigMap reference and
Gateway signer Secret references/mount before planning application changes.
This static check does not prove runtime readiness or authenticated product flows;
acceptance still needs fresh Auth tokens carrying the required session epoch.
The failed restart was refenced successfully. Preserve its checkpoint, progress,
private failure captures and all earlier evidence; the existing reconciliation
runner cannot be replayed against that new boundary.

The fixed `operator-media-target-gateway` successor separately retains the exact
successfully refenced restart checkpoint, prior prepared/normalized/source and
progress bytes, original runner and both Gateway failure logs. Its canonical
verifier retains the earlier failure chain, requires all 46 resources to match
the previous target while fenced (21 Deployment targets, 19 Services and six
NetworkPolicies), and checks all 22 workloads including the independently
retained Hub. UID drift, an old/target mixture or a residual User bootstrap
override blocks admission. A fresh accepted source capture and exclusive build
directory bind the corrected Gateway configuration; the old authority is never
reused. Its distinct terminal schema retains the same full readiness, signed
actor, storage, resource and migration checks and does not claim deployment PASS.

### Explicit reset after retained records expire

The native `/msgs/` boundary includes `index.db`. A retention expiry can change
its message count, first sequence and subject counts while payload block files
remain byte-identical. That is a real preservation failure; neither index-file
exclusion nor rewriting record timestamps is an acceptable repair.

The exact `a83c5bdb26f1` operator reset is separate from FULL recovery. It retains
the failed checkpoint and original/off-node and post-migration archives. It
records and rechecks the old PVC inventory during reset; this is an at-reset
observation, not an ongoing preservation guarantee. A root-reviewed runner
creates a separate UID-bound empty PVC,
bootstraps the retained 15-stream/43-consumer contract with existing identities,
and proves exact configuration, zero records/consumer floors, cold native bytes
and isolated restore before the off-node copy checkpoint. The original workload
templates are restored with only the hub `jsdata` claim changed by CAS.

`voice-nats-reset-active-contract-v1` binds that new baseline and PVC explicitly;
the old migration receipt remains an unchanged historical predecessor. The old
active-release record is retained privately and invalidated, and the failed FULL
ends `ABORTED`, never deployment PASS or rollback eligibility. The exact helper
successor is installed with `--install --upgrade-idle-expiry-reset-v1` only after
the verified reset closure. The reviewed runner is a recovery artifact, not a
permanent installed CLI.

The zero-baseline proof describes creation, not a restriction on subsequent
application writes. Create recent application data after reset, then run ordinary
FULL: its new cut must preserve those records while the canonical target contract
reaches 45 consumers. Verify the same application message/file after deployment
and restart; the empty reset proof alone does not establish new-build quality.

The workflow prepares the cut, proves an isolated restored copy, uploads only
an authenticated encrypted CMS backup to GitHub, independently reads the entire
artifact back through root, authorizes one paused apply, proves the original
native bytes and complete account census again, and then resumes applications.
No bootstrap Job runs against the retained store. NATS ConfigMaps, Secrets,
credentials, streams and consumers are retained. A failed proof retains the
owned fence and reports BLOCKED; it does not restart clients or overwrite data. A failure during
read-only capture, before the FENCE checkpoint, instead records
`FAILED_READ_ONLY / BLOCKED / NOT_ACQUIRED` with an immutable snapshot and a
finite public guard label. This terminal attempt has no deployment PASS or
rollback eligibility. Idle admission authenticates the captured code, complete
unchanged snapshot, failure receipt and matching failed prepare journal; a raw
BLOCKED status is insufficient. A new operation repeats every normal capture,
backup and off-node restore check. An earlier stale read-only checkpoint may
only be classified separately with explicit operator provenance and its exact
original checkpoint and journal retained.

## V4 full-master safeguards and compatible backend proof

V4 delivery uses the separate absent-only directory
`/home/pmd/voice-nats-rollout-v4`. The reviewed captured launcher runs
`--install --upgrade-v4` against the exact installed V3 binding. It preserves
the original code as `installed/code-v3-preserved`, existing enrollment,
policy, recovery material and units. It does not issue or renew service actors.
The sole known failed prebuild request can receive a root-owned disposition
under the operation lock only when its exact request/response, active marker,
source directory and absent build/operation match the reviewed predecessor.
Its original STARTED journal remains unchanged; every other unfinished request
or identity drift vetoes the upgrade. Published V3 assets remain immutable.

Full deployment validates selected existing mounted actor identities, signed
effective permissions and actual isolated server authentication. This grant
compatibility evidence does not claim exercised business publication, ACK or
delivery. Captured generation, role credential key/path/container, source ACL
and Secret identities are rechecked before fencing and after custody.
Unsupported authority is a prerequisite veto, never a silent grant renewal.

Crossing Space migration16 requires one coherent MVCC snapshot containing
version and pre-counts, a restored dump proof, and full encrypted off-node byte
readback before migration. The snapshot closes before ALTER; version, count and
default postconditions gate restart. Already-applied16 does not repeat its
backfill or erase later opt-ins. Bot004 duplicate non-NULL interaction tokens
(including empty strings) are rechecked under the writer fence before any
native contract mutation or database Job; conflicting data is not rewritten.
The same read-only prerequisite accepts clean Bot schema versions1 through6,
including the canonical `000006_bot_chat_create_requests` target after migration;
newer versions, dirty state and duplicate token groups still refuse admission.

The hub renderer is a separately bound root-only init image transition; the
runner cannot apply arbitrary hub changes. Exact captured inputs and private
old/new output equality precede startup. Actual completed init identity,
generated configuration and unchanged broker are checked after hub readiness
and before applications resume. Failure refences; native migration/CAS still
require closed writers. Renderer retry validates the recorded completed DB
state rather than rerunning migrations or assuming the original version.
Renderer continuity retains complete metadata authority and spec while allowing
controller status revision churn; mutations CAS against the current live revision.

The first compatible populated backend witness is the reviewed Story image
pair, whose four schema UP files are identical and whose old binary has passed
the documented text-write probe on current clean4. Root independently binds
the actual mounted Story actor, application DSN, PostgreSQL authority, clean4
and exact image/source witness; the disposable probe is not a live receipt.
Unknown old Story content and unproved backend rollback selections, including
historical Bot/Search, fail closed. Ordinary frontend rollback remains separate.

A rebuilt current Story component requires its own actual-binary/current4
witness even when its source has not changed. The locked root preflight now
admits a genuinely missing child/config pair automatically before capture or
fence. It runs the bundled canonical Story probe against that exact immutable
image and a disposable PostgreSQL clean4 store initialized with the four pinned
migration files. Actual health, text creation, SQL defaults and archive timing,
missing-profile refusal, the negative count control, unchanged complete schema
catalog and complete owned cleanup must all pass before registration. This
fixture accepts no selected store, staging address or service credentials.

Automatic evidence binds child/config/application binary, migration and catalog
hashes, authenticated helper/probe/fixture hashes and actual result/cleanup.
An image revision label is retained when present; absence is recorded as null,
not replaced with an invented source or publisher approval. Current source CI
and immutable target image admission remain separate mandatory deployment gates.

The existing root-private registry and locked atomic selector are reused. Both
captured Story preflight and fresh rollback re-read the selected enabled record.
Revoked, corrupt or conflicting evidence fails closed and never triggers a new
fixture. Adding another automatically proven pair preserves an unchanged
captured origin record; changed evidence or revocation still vetoes rollback.
Manual v1 records retain their exact registry-revision comparison and publisher/
review evidence contract. Code upgrades preserve every approved/revoked record.
The runner cannot register evidence or supply a compatibility boolean, and
schema, PostgreSQL identity, mounted actor and target-contract checks remain.

Build the probe once on Linux with Go 1.26.8 from
`src/backend/story/cmd/owned-story-probe`, using
`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o LINUX_STORY_PROBE .`,
then bundle it with the existing kernel and renewer:
`python bundle.py KERNEL OUTPUT LINUX_BOOTSTRAP_RENEWER LINUX_STORY_PROBE`.
The captured-file limit for a probe is 16 MiB; these build flags keep the
executable within that bound without changing the limit.
The launcher admits at most64MiB total with each member still bounded to32MiB;
exact archive bytes/hash, complete manifest membership and root custody remain
mandatory. Subsequent ordinary FULL and rollback releases require no manual
per-image fixture, witness promotion or helper installation.

Acceptance order is populated Story transition, an ordinary postrelease record,
fresh latest-operation rollback preserving that record and all dynamic native
consumer/ACK state, current Story re-upgrade, then the entire current-master
catalog (22 apps and renderer) with migrations, readiness and smoke evidence.
Final state must be current master. Keep automatic deployment disabled until
that full state and preservation proof pass. An incompatible failed full apply
stays fenced for separately authorized forward repair; no SQL DOWN, previous
DB archive or earlier native cut is a recovery shortcut.

## Historical V3 upgrade and explicit existing-actor enrollment

V3 uses a separate absent-only delivery directory,
`/home/pmd/voice-nats-rollout-v3`. Preserve the published V2 assets and its
existing download directory. Build both Linux executables with Go 1.26,
`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`: the known-baseline kernel and
`src/backend/pkg/cmd/nats-jwt-issuer` bootstrap renewer. Then run
`python bundle.py KERNEL OUTPUT LINUX_BOOTSTRAP_RENEWER`. Review every archived
member, both executable hashes and the complete captured launcher command.
Unreviewed build output is not an installation command.

After the reviewed V3 bytes have been published and downloaded, the human root
captured launcher first runs `--install --upgrade-v3`. This accepts only the
recorded immutable V2 predecessor, preserves it as `installed/code-v2-preserved`,
and swaps the verified staged code under the global operation lock. It retains
the existing policy, recovery key/certificate, units, requests and journals.
Queued or interrupted requests, a non-active marker, a different predecessor
or an untrusted staged tree veto the upgrade. A retry between the two renames
accepts only the same recorded new binding. A partial staged copy fails closed;
inspect its exact root-owned receipt and custody before any manual recovery,
never delete `installed/`, the preserved V2 code, policy or recovery material.

The separate human command `--enroll-existing-bootstrap` uses only the already
validated private account signer at
`/var/lib/voice-nats-issuer/app-account-r20260930a4.seed`. The CI request bridge
has no issuer action. Fresh signed-chain, revocation, scoped-authority and
immutable old Secret checks precede renewal. The same account, actor and user
seed retain all existing claims and deny rules; renewal permits only the three
fixed stream UPDATE subjects and the exact consumer INFO/CREATE subjects for the
declared durable pairs. A create-only immutable new bootstrap Secret is
authenticated with its existing reply prefix in a pinned network-none scratch
broker before an atomic root enrollment. The old Secret and Windows signer
original remain intact. Never paste seed, JWT or credentials into a receipt.

Music's v4 activation preserves `bootstrap-enrollment.json` and its three
normalized consumer declarations. The same explicit root command selects that
already enrolled actor, writes only `bootstrap-renewal-draft-v4.json` and
`bootstrap-enrollment-v4.json`, and binds the complete prior receipt by digest.
The new isolated CREATE/INFO proof must retain the prior three normalized
defaults and all shared account/server/reply authority. Missing v4 enrollment
blocks current backend preparation; CI cannot renew credentials implicitly.
The added bootstrap grants are exactly INFO/CREATE for
`file_events.messaging_music_metadata`. Messaging's existing-actor renewal adds
only that consumer's INFO and ACK tail, retaining its already owned inbox.
Neither renewal grants Messaging consumer CREATE or arbitrary publication.
All Secrets are create-only; historical enrollment and active-contract receipts
continue to validate through their exact v1/v3 schema dispatch.

The v4 migration adds one `DeliverNew` consumer on the existing `file_events`
stream. It does not change that stream's subjects, retention or stored records.
Consumer creation and native preservation proof must finish before applying the
new File producer or Messaging runtime. Existing consumer ACK/delivery state and
the new consumer's progressed state on retry remain authoritative; never replay
bootstrap Jobs or reset the stream to activate Music. New signed credentials,
isolated server proof and hosted preservation integration are required before
deployment; an earlier three-consumer PASS is not v4 activation evidence.

Normal backend prepare binds that enrollment, the approved source, pinned
server and complete current contract. The authoritative closed native cut and
encrypted full off-node readback precede the three fixed stream UPDATEs and the
exact CREATEs for any missing v3 consumer declarations. Preserve every old record, existing consumer
config/ACK/pending/redelivery state and dynamic resource. The post-config cold
ledger is the app-apply baseline; do not compare changed config bytes to the
pre-migration ledger or restore that old archive. Only proved pinned recovery
timestamp observations are classified; durable and untouched-stream creation
identity remains strict. The public script ConfigMaps and root active-contract
receipt advance only after the complete proof, without replaying bootstrap Jobs.

The v3 plan adds only the declared `voice_space_media_chat` and
`voice_space_media_role` durables alongside the existing friend-removal
durable. It accepts only the exact reviewed predecessor stream declarations
and adds the missing target subjects to `chat_events` and `role_events`.
Starting from the captured 9ae2e0c baseline, the v3 plan adds `chat.updated`,
`chat.deleted`, and `space.voice_room_access_invalidated` to chat, plus
`role.voice_policy_invalidated` to role. An explicitly invoked existing-actor
renewal may add only the three stream UPDATE subjects and matching consumer
INFO/CREATE subjects for the declared durables. There are no wildcard
grants, and every existing deny remains a veto. The actor, account,
seed, saved enrollment and its legacy proof are retained; the v3 scratch proof
is validated separately and bound into the new plan. This source change does
not issue credentials or rewrite installed enrollment or recovery receipts.

Root preflight also pins the exact reviewed Search bootstrap template, whose
required Chat inputs include `chat.updated` and `chat.deleted`. Its extracted
script hash joins the Realtime/Analytics target authority passed to the compiler;
the legacy contract remains unchanged. Modified Search bytes fail root preflight,
and Notification still requires its unchanged legacy script hash. This source
binding authorizes target compilation. After the existing native preservation
proof, the same Search script may advance through the existing UID/resourceVersion
ConfigMap CAS and active-contract receipt, alongside Realtime and Analytics.
No bootstrap Job is executed, and Notification gains no new script authority.

The already-installed idle helper requires its own separately reviewed,
versioned predecessor transition. First compare the installed captured helper
binding and manifest with the reviewed candidate; a release label or prior
successful rollout does not identify the installed helper. The existing
`--upgrade-idle` route remains limited to its exact V15 predecessor. The new
`--upgrade-idle-ceccb-v1` route accepts only the reviewed installed binding
`ceccb8a607957d9c436247fb183d8467d48ac33ece8b3e6c70850404c569e032` and writes
its separate `upgrade-idle-ceccb-v1.json` receipt and `code-ceccb-preserved`
backup. Both routes retain the same idle, closure, marker, queue, journal,
policy, recovery-key, witness and history checks; neither permits a changed
predecessor or replacement of earlier evidence.

The next exact transition, `--upgrade-idle-source-capture-v1`, is limited to
the captured installed binding
`65717bd51213624f7c0890e97b2cdf1ef9149f57fb27f22f3bdbebca5fb963e5`. It writes
its own `upgrade-idle-source-capture-v1.json` receipt and
`code-source-65717-preserved` backup through the same strict idle transition.
Its only additional journal admission is the immutable full-prepare request
from run `38469787490`, whose source-capture step was rejected before a
checkpoint or source build existed. The disposition binds the exact request,
journal and response hashes and rechecks that the operation/build are absent
and its private source workspace is still empty. It does not infer success,
claim that no external effects occurred, or admit another failed request.
The smaller GitHub Actions page size preserves the existing complete 10,000
job/artifact inventory bound after the observed 100-row endpoint failed; every
page and stable total is still required. The route also retains all existing
idle, closure, marker, queue, journal, policy, recovery-key, witness and history
checks. It neither changes the V15 nor CECCB predecessor routes nor rewrites
the failed request history.

The next captured-launcher route, `--install --upgrade-idle-search-bootstrap-v1`,
accepts only installed binding
`a025ffb62b4c0716a44611c4ac00fe9cae60b8bcb5e8b48204b44a62dc541749`.
It preserves the predecessor as `code-search-a025-preserved` and records its own
`upgrade-idle-search-bootstrap-v1.json` receipt. All existing idle, terminal
closure, policy, key, marker, queue and history guards still apply. Its two
additional dispositions are exact failed FULL prepares from runs `38478016683`
and `38482268019`: respectively rejected at NATS preflight and target build,
with no rollout directory/checkpoint. Both are explicitly abandoned before a
checkpoint, never classified as successful, effect-free or terminal releases.
Admission binds their complete request, journal and response bytes plus the
retained source/build tree hashes and custody; these are checked again after
the code swap. Original request history and workspaces remain intact. Any
different request, added rollout directory, changed tree or replaced custody
refuses this transition. Use only the reviewed captured bundle and launcher;
this route does not renew credentials or alter existing grant receipts.

The prepare associated with d4 run `38274239522` is a hard stop until its
exact journal, matching checkpoint and response are accepted by the existing
operation-closure checks. Use the `Record exact operation status after failure`
step in [`staging-deploy.yml`](../../../.github/workflows/staging-deploy.yml)
when it has an operation ID; it records the finite summary fields `operation`,
`phase`, `status`, `fence_status` and `error`. Correlate these with the exact
checkpoint and request journal, including `prepare_stage.name`,
`prepare_stage.status` and `prepare_error`. A `COMPLETE` journal by itself does
not prove that the run has a valid terminal closure. If the workflow did not
record an operation ID/status, associate only an existing
`installed/journal/<nonce>.json` whose request has `action=prepare` and exact
`run_id` and `source_sha` matching that workflow run and reviewed source. Its
paired response is `installed/responses/<nonce>.json`; its checkpoint must name
the same operation. Report only the nonce prefix, journal `phase`,
`prepare_stage.name/status`, `prepare_error`, response `status/error`, and—when
present—checkpoint `phase/status/fence_status`. If there is no unique exact
match, the association remains unknown: no helper transition or deployment
may proceed, and no ID should be inferred from unbounded logs. Preserve its
journal, response, checkpoint and enrollment bytes; do not delete, rewrite,
re-enroll or reset them to make admission pass.
The helper transition does not renew bootstrap credentials or prove
service-actor grants. Recheck the same actor/account/seed identity and exact v3
grant scope through the existing approved authority path; never broaden
wildcards or rotate keys as a workaround.

Keep historical and current proof receipts distinct. The successful historical
9ae full/rollback/full evidence remains valid for that release, but does not
prove that a later helper or actor grant accepts the v3 contract. The current
hosted native fixture separately passed with seven preserved records, 45 prior
consumers and two declared consumers created; its fresh rollback retained eight
records. This is fixture evidence, not a live staging receipt. Before any
ordinary release, require current source CI and the native preservation gate,
then use a new non-reset operation with a fresh populated cut, authenticated
encrypted backup and complete readback, isolated restore, paused apply, native
byte verification and full-account census. A rollback must take a fresh cut
after release and preserve post-release records. The fixture is not staging
acceptance; only actual staging release and fresh rollback receipts prove those
gates.

Service-actor authorization remains a separate prerequisite. Re-prove the
current Role publish grant for `role.voice_policy_invalidated`, the Space
publish grant for `space.voice_room_access_invalidated`, and the Voice
consumer INFO plus exact ACK grants for `chat_events/voice_space_media_chat`
and `role_events/voice_space_media_role` against the approved service source.
The existing `_INBOX.voice.voice.>` subscription scope already covers their
delivery subjects. This bootstrap-renewer change does not modify those service
actors; any service-actor renewal must retain its existing identity and use its
own reviewed exact grant transition.

### Grant drift and prepare-failure diagnosis

Updating `deploy/nats/acl-intent.yaml`, deploying a new application image or
installing a new preservation helper does not change already signed user JWTs.
Staging has encountered both service-actor grant drift and, after its correction,
separate enrolled-bootstrap grant drift. A valid signed chain and successful
authentication alone do not prove that a credential permits the current source
contract. Check the credentials actually selected by the current generation and
mounts or enrollment, not newly generated fixture credentials.

| Credential authority | Required comparison |
| --- | --- |
| Service actors | Signed effective publish/subscribe grants against the canonical service ACL: this includes Role/Space invalidation publications and Voice consumer INFO/ACK rights. Existing owned inbox wildcards may already cover concrete delivery subjects; do not add redundant authority solely to equalize lists. |
| Enrolled bootstrap | Signed grants against the canonical bootstrap ACL and the fixed migration plan: stream UPDATE and consumer INFO/CREATE for every declared migration pair. Service-actor acceptance does not prove bootstrap authority, or vice versa. |

Ordinary deployment does not renew either credential class. An explicit reviewed
operator transition must retain the account, user identity and seed, all denies,
expiry and limits; add only the bounded missing subjects supported by the issuer.
Use the existing protected signer and private pipe interface, never wildcard
widening or root rotation to make preflight pass. Immutable service Secret changes
need their own exact identity/CAS, protected original-byte backup and affected
workload stop/readback procedure. Bootstrap enrollment uses a create-only immutable
candidate Secret and an atomic protected receipt switch after proof; retain the
old enrollment evidence, original draft and Secrets. A saved draft is bound to its
original issuer and inputs: blindly replaying the initial enrollment command can
refuse or reuse stale credentials. A reviewed incident wrapper is not a permanent
installed CLI or permission to rewrite historical receipts.

Before retrying, verify the complete proposed grant set and unchanged signed
claims, including deny overlap. Then prove the actual candidate credentials with
the pinned, isolated broker: service authentication and effective ACL compatibility,
or the complete bootstrap consumer-declaration proof as applicable. Read back the
selected credentials/enrollment and repeat the normal source preflight. These
checks do not attest live business delivery, a successful FULL deployment or its
preservation gates; those require the ensuing operation's own evidence.

For any failed prepare, first correlate the exact workflow run/source/attempt
with its request journal and response using the bounded procedure above. Record
the failing stage and whether a matching checkpoint exists. A `target-build`
failure before checkpoint creation is a prepare/build failure; do not call it a
fenced rollout or run rollback based on the workflow label alone. Conversely,
when a checkpoint exists, its `phase/status/fence_status` and journal determine
the required recovery and closure. Absence of a checkpoint alone does not prove
that no external effects occurred or make the request safe to discard.

Keep raw JWTs, seeds, credential bytes, Secret bodies and encoded backups out of
terminal/CI logs and public receipts. Report only the correlated finite stage,
safe error category, reviewed subject differences, hashes and resource identities.
`unexpected` or empty CLI stdout is not a root cause, even with exit code zero;
inspect bounded private stderr and signed permissions before deciding whether the
failure is authorization, parsing, build or another class. Track missing safe
stage/error reporting under `CI-A1-DEPLOY-DIAGNOSTICS`; that follow-up neither
waives a failed gate nor authorizes a blind retry.

Increasing the social duplicate window does not establish retroactive 24-hour
deduplication history. Fresh rollback captures current post-release records and
retains additive configuration. User was the initial backend acceptance target;
future Realtime/Space/Social releases still require their actual existing actor
rights and, for Social, the documented DB/outbox compatibility prerequisites.
Keep automatic deployment disabled until the populated backend transition and
fresh latest-operation rollback have both passed on the same selected PVC/PV.

## Historical V2 installation

The following V2 installation receipt is retained for provenance. Do not rerun
it on the already installed host or substitute its bytes for the V3 upgrade.

Build the Linux kernel from `../nats-known-baseline` with Go 1.26 and
`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o KERNEL .`.
The historical V2 builder used `python bundle.py KERNEL OUTPUT` to produce `rollout-bundle.tar` and the
checksum-bound `rollout-root.sh`. Review the complete code manifest and launcher
before copying these two files to `/home/pmd/voice-nats-rollout/` on pmdebook.
Both files must belong to pmd and must not be writable by another user/group.

The operator prepares `/root/voice-nats-rollout-policy.json`, owned by root with
mode 0600, using the existing staging configuration. `s3_signing_endpoint` is
required. Optional keys are `gateway_host`, `storage_host`, `livekit_host`,
`web_host`, `admin_host`, `developer_portal_host`, `gateway_tls_secret`,
`storage_tls_secret`, `image_pull_secret`, `apply_observability`, `minio_image`,
`minio_mc_image`, `minio_storage_class`, `minio_storage_size`, and `web_origin`.
These are existing endpoint/infra intentions, not application or NATS secrets.
Use the configured values; placeholder domains are not a valid deployment plan.

For the reviewed bundle SHA256
`94cd5ce566dacd5bca77f6131853f2047d6fe961c84b7cb8ecc06f339be094f3`
(9881600 bytes), run this exact command in the human root shell. It captures and
checks the launcher bytes before executing that captured content; it does not
execute the mutable pmd-owned launcher path.

```sh
/usr/bin/python3 -I -S - <<'PY'
import hashlib, json, os, pathlib, stat, subprocess, sys
if os.geteuid()!=0 or sys.platform!='linux':raise SystemExit('HUMAN_ROOT_LINUX_REQUIRED')
path=pathlib.Path('/home/pmd/voice-nats-rollout/rollout-root.sh')
for parent in (*path.parent.parents[::-1],path.parent):
    s=parent.lstat()
    if not stat.S_ISDIR(s.st_mode) or s.st_uid not in (0,1000) or s.st_mode&0o022:
        raise SystemExit('LAUNCHER_CUSTODY=BLOCKED')
fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
try:
    before=os.fstat(fd)
    if not stat.S_ISREG(before.st_mode) or before.st_uid!=1000 or before.st_mode&0o022 or before.st_nlink!=1 or before.st_size!=4349:
        raise SystemExit('LAUNCHER_CUSTODY=BLOCKED')
    raw=os.read(fd,4350);after=os.fstat(fd)
    fields=lambda s:(s.st_dev,s.st_ino,s.st_size,s.st_mtime_ns,s.st_ctime_ns)
    if fields(before)!=fields(after) or len(raw)!=4349 or hashlib.sha256(raw).hexdigest()!='3e085b498cb4bb59e89fdb8c3f135eac0edeb30c8e2f3eac34dcdf92d7ed72ae':
        raise SystemExit('LAUNCHER_SHA256=BLOCKED')
finally:
    os.close(fd)
policy=pathlib.Path('/root/voice-nats-rollout-policy.json')
expected={'s3_signing_endpoint':'https://voice.comrade.click',
    'gateway_host':'voice.comrade.click','gateway_tls_secret':'voice-gateway-tls',
    'livekit_host':'livekit.comrade.click','web_host':'app.comrade.click',
    'admin_host':'admin.comrade.click','developer_portal_host':'developers.comrade.click',
    'storage_host':'','web_origin':'https://app.comrade.click'}
try:
    fd=os.open(policy,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW|os.O_NONBLOCK,0o600)
except FileExistsError:
    fd=os.open(policy,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        s=os.fstat(fd)
        if not stat.S_ISREG(s.st_mode) or s.st_uid!=0 or s.st_mode&0o077 or s.st_nlink!=1 or not 1<=s.st_size<=65536:
            raise SystemExit('POLICY_CUSTODY=BLOCKED')
        if json.loads(os.read(fd,s.st_size+1))!=expected:raise SystemExit('POLICY_AUTHORITY=BLOCKED')
    finally:os.close(fd)
else:
    try:
        content=json.dumps(expected,sort_keys=True).encode()
        with os.fdopen(fd,'wb') as stream:stream.write(content);stream.flush();os.fsync(stream.fileno())
    except Exception:raise SystemExit('POLICY_CREATION=BLOCKED') from None
subprocess.run(['/usr/bin/bash','-s','--','--install','/root/voice-nats-rollout-policy.json'],
    input=raw,env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'},check=True,timeout=180)
PY
```

The public endpoint/domain values above were read from existing staging
Deployment/Ingress/MinIO configuration on 2026-10-05. The command creates a
missing policy as root-private mode 0600 and refuses conflicting existing
policy; it never overwrites it. If these public values change before installation,
review and bind the updated policy instead of accepting a mismatch. No Secret
values or application/NATS credentials are included.

The installer refuses an active operation or an existing installation. It
captures immutable code outside the overwritten release checkout, installs a
fixed systemd oneshot/path/timer, and creates a private RSA3072 recovery key and
matching certificate at
`/var/lib/voice-nats-preservation/installed/recovery/`. Existing keys are verified
and retained. Agents do not execute this root installation. There is no sudo
permission, arbitrary command endpoint or downloaded-code execution bridge.

### Interrupted installation

An existing `installed/` directory makes another `--install` fail deliberately.
Do not delete this directory, replace its recovery key, or retry deployment to
repair installation. In the human root shell, stop only the path and timer with
`systemctl stop voice-nats-preservation.path voice-nats-preservation.timer`.
If the service is still running, wait for its bounded completion and inspect its
operation status; do not terminate a live preservation operation to reinstall.

Before repair, require the live marker phase `active`, every existing rollout
checkpoint `PASS` or `ROLLED_BACK`, empty inbox/processing directories, and no
unresolved `STARTED` request journal. Otherwise use that operation's exact owned
recovery procedure and keep deployment disabled. Preserve the existing recovery
key and certificate through the private offline channel before changing any
installation files. If only one key/certificate file exists, or their public
keys do not match, stop: neither file may be regenerated or overwritten.

Compare the installed capture manifest and every code hash with the reviewed
bundle. Restore only missing installation files from that exact bundle, with
the installer-defined root ownership and permissions; conflicting code, policy,
or unit content requires review, not replacement. If code, policy, directories
and matching recovery key/certificate are complete and only unit creation or
activation failed, create only missing `.service`, `.path`, and `.timer` files
using the exact `SERVICE`, `PATH_UNIT`, and `TIMER` constants in the captured
`installer.py`, root ownership and mode 0644. Existing units must match those
bytes. Then run `systemctl daemon-reload` and
`systemctl enable --now voice-nats-preservation.path voice-nats-preservation.timer`.
Check the enabled units and an empty idle queue before allowing a release.
Earlier partial failures need a reviewed reconstruction of the missing files;
there is no automatic reinstall or key rotation path.

Back up `recovery-key.pem` and `recovery-cert.pem` through the operator's private
offline channel, outside this machine and CI artifacts. Do not paste them into
chat, logs or tracker comments. Losing both node and recovery key makes the
encrypted artifacts unusable. GitHub retention is 30 days; a longer disaster
recovery policy requires an independently retained ciphertext and private key.

## Release and rollback authority

The approved source is root-selected current master from the fixed Voice
repository, with completed successful `ci-gate` and `staging-stack-lock` jobs in
the fixed push CI workflow. Source bytes are checked against its complete Git
tree and blob hashes. Trusted CI tag-lock images are independently resolved to
linux/amd64 GHCR digests; the tag-only artifact does not itself attest digests.
This assumes authorized project package publishers. Source files are passive
compiler inputs; their scripts are never executed as root.

Successful gate jobs alone are insufficient: `source_authority.py` also checks
the parent source CI run and its complete observed attempt. An in-progress
parent is permitted after the required jobs pass; a completed parent must have
conclusion `success`, or satisfy the narrowly checked deployment-only `failure`
exception. A completed `cancelled` run is rejected even when its visible gate
and lock jobs succeeded. A successful web/images-only deployment does not prove
that an earlier backend change received a full rollout.

Before a standalone full rollout, check the exact source run's conclusion as
well as its gate and lock jobs. If it is cancelled, preserve the guard and wait
for an eligible source-triggered CI run from the next reviewed master change;
do not blindly retry deployment or manufacture approval from individual green
jobs. A generic `NATS_ROLLOUT_BRIDGE=BLOCKED` response does not identify the
private journal failure. Keep the failed run/job receipt and use the bounded
journal correlation procedure above before attributing a root cause.

CI concurrency groups include the event name: scheduled and manual checks must
not cancel the push workflow that authorizes a release. New pushes on the same
branch still supersede older push runs; deployment approval remains bound to
the exact source revision and attempt.

Reusable deployment retains the parent CI run ID. Standalone staging dispatch
has a separate execution authority bound to its fixed workflow/master/head and
attempt. The encrypted backup artifact belongs to the current execution run,
head, operation and fresh challenge, even when source approval came from a
different CI run. Root reads back every ciphertext byte and validates the ZIP's
single expected member; a runner-written hash or local copy is insufficient.

For an images-only rollback, dispatch staging with `rollback_operation` equal
to the latest successful operation. Root derives its changed workloads and
recorded actual old image digests from the protected receipt, keeps current live
configuration, and takes a fresh cut of the current PVC, including records
written after the previous release. It never replays the prior archive or
undoes database migrations. A latest successful FULL may be the origin of a
Story-only rollback: only its recorded reviewed Story application predecessor
is selected. Fresh current Story schema4 and the exact migration hashes are
required; current configuration, template, sidecar/init pins, other workloads
and HUB renderer remain. Generic FULL/app-only reversal is rejected.

Recovery of an interrupted images-only apply before business restart uses the
same ROOT CLOSED-seal producer after restoring its captured templates. The
original receipt deadline remains in force at every template, resume and release
effect. Completion checks the owned release marker, running selected storage,
and the UID, exact restored template, replica count and readiness of every
captured workload. A late failure re-fences only the owned objects; replacement
identities remain BLOCKED with unknown fence state rather than being taken over.

Nonce replay returns the persisted result; interrupted finish with an already
persisted PASS does not repeat restart or turn the operation into BLOCKED.
Unresolved interrupted mutations remain fail-closed and require exact owned
recovery rather than blind rerunning scripts. Status is read through the same
fixed installed client, without root authentication per deployment.

## Compatibility and bounds

Full/app-only plans cover the canonical 16 non-NATS requirement rows, including
Auth mail/key authority, principal/cursor/MinIO Secrets, ingress/CORS/signing,
frontend reconciliation, migrations, existing infra and database-init evidence.
Exact existing semantics may be preserved. Missing resources, changed
StatefulSets, enabled observability reconciliation and unsupported initialization
changes are explicitly rejected before the fence; they are not silently omitted.

The last confirmed staging cut had 42 durables while the reviewed source
bootstrap had 43. This is historical evidence, not a fresh live census. After
installation, root must capture the actual current marker/store/account and
compare the requested release contract before fencing. Any remaining mismatch
blocks backend releases including Gateway until the explicit NATS contract/ACL
migration prerequisite is resolved. Frontend-only
images-only targets retain every backend template/configuration and NATS intent;
root derives this scope from the actual target, not a caller's waiver flag.

Runtime data is not restricted to the reset fixture's 15 streams/42 durables.
Isolated monitoring enumerates the complete JWT account store, including extra
streams, durable and ephemeral consumers, ACK floor/delivery/pending/redelivery
state and full native records. Ephemeral inactive thresholds remain effective;
expiry or changed native state cannot be silently accepted as preservation.

Rollout archives stream bounded chunks and support PVC capacity up to 48 GiB and
262144 native file/directory entries; the reset fixture's 64 MiB/8192 limits retain
their defaults. Before fencing, capacity, existing native entries/types, free
space for archives/restored copies/encryption/readback and available inodes are
checked. Active-write growth may still exhaust capacity after this preflight;
such failures retain the fence with an explicit recovery status. Operation
checkpoints have a 128 MiB read/write bound; ancillary private JSON inputs retain
their 2 MiB bound, and monitoring remains bounded at 64 MiB. Supported capacities
are finite.

## Evidence

`*_test.py` covers canonical full/app-only first and repeated CLI compilation,
paused apply/CAS, exact image rollback, journaling, source/custody authority,
authenticated CMS roundtrip, installer/inbox custody and streaming large stores.
`../nats-rollout-preservation_test.sh` exercises all-mode shell authorization.
`rollout_census_integration.py` runs only an owned disposable remote Linux
fixture on pmdebook under pmd: 16 streams, 44 consumers, 5 known records, extra durable
redelivery/pending ACK and an ephemeral, complete native/census equality and
restored known payloads. This does not prove an actual staging Kubernetes
release. Actual rollout activation and before/after receipt are still required
after installation and review.

Every observed job is bound to the selected run/head/attempt and has a unique name.
Known nondeployment failures, cancellations, timeouts or unknown terminal results
reject source approval even while the overall CI run remains in progress; successful
`ci-gate` and `staging-stack-lock` do not override an already failed optional job.
Owned pending jobs and the exact deployment-only failure exception remain allowed.

A completed CI failure is eligible only when the exact `deploy-staging / deploy`
job is the sole failed job, every current-attempt job is terminal and bound to
the same run/head/attempt, and both required authority jobs succeeded. Only
fixed optional branch jobs may be skipped; missing, duplicate, cancelled,
timed-out, spoofed deployment names or any failed build reject approval. This
permits first installation recovery after a missing-helper deployment failure
without accepting a failed build as an approved source.

## Actual staging acceptance after the one-time installation

The human operator first runs the captured-byte installation command above;
agents do not authenticate as root. The installed bridge must be idle before
installation, and an existing recovery key is retained. Subsequent release and
rollback transactions use the normal workflow, without another root login.

Acceptance requires populated staging data. A zero-message cut is insufficient.
Use real records produced through the existing application flow, or a separately
reviewed NATS-only fixture whose existing grant and side effects are established
before publishing. Do not issue credentials, change ACLs, replay bootstrap,
create database identities, reset consumers or replace the selected PVC to make
the proof pass. A missing publish authority is a named prerequisite.

Record the approved source CI and execution run identities, operation/challenge,
selected PVC/PV UID, original running image digests, nonzero pre-release record
count, complete native/census hashes and ACK/pending/redelivery state. The release
must retain the same claim, prove isolated restore and encrypted off-node custody
for that operation, apply while every enrolled workload is paused, verify the
unchanged original store, then record the actual new running image digests and
sole verified restart PASS. Do not publish payloads, keys or private snapshots.

Before rollback, produce and confirm additional real records. Dispatch a new
execution run with the latest successful operation as `rollback_operation`.
Require a fresh cut containing those post-release records, a new encrypted
off-node artifact, the same claim and complete before/after state, and the
recorded original actual image digests after restart. The previous archive is
never restored over the current claim. Both root-owned successful receipts and
the active-release chain are required; a failed attempt is not a PASS receipt.

A frontend-only proof establishes that concrete version transition. It does not
close a remaining durable/ACL migration prerequisite or establish
acceptance of backend releases that the pre-fence compatibility gate rejects.

Selected Kubernetes store custody retains the root-safe ancestor and exact
PVC/PV/path checks. Its live leaf is UID65532, GID10000, mode2770, bound to the
captured HUB fsGroup10000/OnRootMismatch and broker UID/GID10000. Both stage and
native migration registration consume the same closed descriptor. This reflects
Kubernetes group ownership; the operator never chmods/chowns the live store.
Private proof stores and reset-baseline custody remain separate. A failed fenced
operation is not an ordinary upgrade or retry authority: continuation requires
an explicitly reviewed exact-operation adoption and preserved original evidence.

Native migration on that selected store runs as the descriptor-bound HUB broker
UID/GID10000, with no supplementary groups and all capabilities dropped. This
permits access to legitimate owner-only descendants created by the live HUB;
leaf ownership alone does not establish descendant access. The original private
server configuration stays root:G65532/0440. Root creates an exact-byte copy
inside an owned0700 directory, root:G10000/0440, for the selected broker mount.
Create and inspect bind its identity, configuration bytes and exact mounts to
the registered descriptor; generic selected-store launches are rejected.
Restored proof brokers, clients and reset stores retain their65532 isolation.

V7 adopts only paused operation `3340764a7d24`, its exact V6 checkpoint and
STARTED journal, captured marker/PVC/PV and named selected-custody failure.
Installation preserves original checkpoint bytes, V6 code, keys, policy and
all witness registry records. Unknown unfinished operations, partial backup
outputs and changed originals veto adoption; installation does not release the
fence or replay the old request.

The separately journaled `resume-cold-backup` workflow action uses a fresh full
execution nonce and approved current V7 helper source. The preserved Story
target remains historical approved source `73ca52699ddf6a9182e3407d5bbdedbc29c06dee`.
Root reconstructs its retained canonical Git files and exact successful CI
stack-lock artifact, recompiles retained parameters and compares compiled,
image-only and server-normalized targets with the original checkpoint. Current
helper approval does not replace historical target approval.

Paused canonical recompilation retains the original desired file/user signing
endpoint objects. The recovery-only producer first requires the exact owned
Deployment UID/resourceVersion and semantics, plus an original-to-owned change
limited to the recorded replica count1→0. It projects only the original desired
spec onto fresh current metadata for server dry-run; ordinary compiler reads
remain live. Exact compiled/image-only/normalized target equality is retained,
and the producer rechecks custody after compilation. Saved targets, snapshots
and live replicas are never refreshed or rewritten to make recompilation pass.

Before accepting a cut, a pinned isolated broker restores the closed native
archive; one actual account monitoring snapshot feeds both census and contract
compilation. Original plan, binding and selected scripts must match, and root
rechecks actor/credential, PostgreSQL, non-NATS, kernel, target and current
execution authority. Rejected startup or proof removes only its owned copy;
unverified cleanup cannot return an accepted cut. Interrupted capture or partial
outputs cannot be replayed. Completed encrypted output recovery requires the
same fresh execution nonce/run/head and operation binding. Ordinary encrypted
off-node readback, paused authorization, apply and release guards remain required.

The ciphertext-only export directory is explicitly root:pmd/0750 after creation,
including under the bridge service umask0077. Its ciphertext remains root:pmd/0440;
private plaintext, keys and checkpoint modes are not broadened. Upload failure
after AWAITING_OFF_NODE cannot authorize a new capture or refresh the saved cut.

## Preserved ciphertext upload recovery

Captured NATS client inputs use a root:65532 directory with explicit mode0750,
including under the bridge service umask0077. Credential files remain
root:65532/0440. This applies to future captures; it does not repair or resume
an existing fenced operation or change its stored authority and deadlines.

V9 provides a closed continuation for operation3340764a7d24 in its recorded
WAITING/AWAITING_OFF_NODE/VERIFIED state. The advanced-state installer preserves
checkpoint bytes and inode, the original V8 code, keys, policy, witness records,
ciphertext, cut and all original journals/responses. It cannot reset progressed
state or admit an unknown unfinished operation. Installation does not repair the
export directory or authorize deployment.

The `cipher_upload_operation` workflow input is separate from cold-backup and
rollback. Current helper source is admitted first. The historical73ca target
workspace is acquired as bounded data before starting the fresh upload window;
it is never substituted for current helper code or executed as an old helper.
`resume-cipher-upload` binds a fresh actual workflow run/attempt/head/nonce to
the immutable original ciphertext/run37900273078/cut. Under the global lock,
root verifies original checkpoint/journal/response and ciphertext custody,
repairs only the owned export directory through its verified FD to0750, and
creates an exclusive immutable upload record. It performs no capture, encryption,
target recompilation or checkpoint rewrite.

`authorize-preserved-upload` requires that explicit upload nonce and fresh
dispatcher authority. Actual GitHub artifact identity and the entire downloaded
ZIP ciphertext are compared with the original private ciphertext. The dedicated
route cannot fall back to ordinary artifact verification. It rechecks source,
actor and state authority after readback. Its deadline is the minimum of the
fresh600-second window and the original four-hour proof expiry; no timestamp or
original proof is renewed. Checks occur before authorization mutation, before
prepared state and before receipt creation; the resulting receipt is clamped
to that deadline. Ordinary authorization retains its existing deadline policy.
Before transaction mutation, rejection preserves the original checkpoint. After
mutation begins, failures retain truthful progressed BLOCKED/refenced state.
Expired original proof leaves the stage fenced and requires a separately reviewed
invariant-bound fresh-proof disposition, never a blind retry or recapture.

Scoped verification covers actualUID1000 ciphertext access under umask0077,
root-isolated immutable record/replay and full ZIP comparison, actual client to
Actions/dispatcher composition with explicit external adapters, workflow/guard
historical-data ordering, ordinary transaction controls and pre/post-mutation
deadline rejection. The captured V8-to-V9 installer fixture verifies preservation,
partial rename retry, unknown journal and original execution/cipher drift vetoes.
Synthetic data, mocked Kubernetes/API boundaries and fixture capabilities do not
establish live backup, apply, rollback or full staging acceptance.

## Exact expired native recovery (implementation in progress)

The dedicated334076 recovery binds a fresh complete private-copy full native/records/config/consumer ACK-pending proof and actual full off-node readback to separate short authority. Original cipher/cut/target/execution/expiry and BLOCKED history remain immutable. Closed selected bytes must equal the proved current observation immediately before first selected broker mount; unknown drift rejects. Durable prospective transaction entry precedes broker startup and intent/issued records; later failure retains truthful progress and refence rather than restoring the adopted checkpoint. Finish-only proof permits missing resume/release obligations without migration/application replay. A safety pause records a new resume obligation. Ordinary routes are unchanged. No live consumer accepts the new authority until the whole adoption/readers/proof/workflow/apply/finish outcome has final independent review and exact CI. Typed startup-rewrite disposition requires real populated pinned-image evidence, not semantic equality alone.

The V10 package includes the complete cold-route dependency closure and fixed
installer unit restrictions. The new ELF and rebuilt archive require independent
validation. Installation admits only the exact V9 BLOCKED/pre-intent adoption;
it preserves checkpoint bytes and does not repair inputs, start a broker, issue
authority, or replay a request. The fixed transferred archive path is
`/home/pmd/voice-nats-rollout-v10/rollout-bundle.tar`. Activation follows combined
implementation/operator review, normal CI/merge and complete remote readback.


The expired-recovery consumer checks its fresh independent capability after durable intent/issued journal work immediately before every NATS API, custody CAS and active-release publication. Finish observation accepts an exactly owned already-verified marker without replaying its CAS. Dedicated apply receipts use a closed authority-bound schema through the public guard/runner/apply consumers; ordinary receipts remain strict. Successful finish-proof request status is separate from the truthful blocked operation.

Ordinary full/app/images rollout and every new rollback keep HUB stopped from
the original cold cut through paused apply and release verification. Clients
stop first. ROOT captures the original complete live census and exact running
container, retains the Pod with one operation finalizer, scales HUB to zero,
and records its current matching container termination with exit0 and no OOM
before removing only its finalizer. Kubernetes1.27 or later and a Ready node are
required; absent or replaced termination evidence blocks capture.

The complete canonical original census is also written into the root-private
operation checkpoint at the shutdown-intent journal boundary. Its canonical
digest must match the intent event before the finalizer CAS or HUB scale can
proceed; checkpoint size limits remain fail-closed. A reconstructed new-format
operation validates the snapshot schema and digest before using it for the
pre-cut native inventory comparison. A historical checkpoint that contains
only `live_census_sha256` remains readable for phases whose cut and backup were
already proved, but it cannot continue a pre-cut capture that needs the missing
original census. Do not substitute a fresh observation or infer the lost census
from shutdown events or the restored store.

Before any copied broker starts, the closed archive enumerates every stream
metadata pair and consumer metadata/state triple, rejecting memory storage and
unknown layouts. Complete offline durable decoding includes ACK floor,
delivered sequences, pending timestamps and redelivery maps. The restored
broker census must enumerate the same native resources. Actual authenticated
CMS decryption verifies every promised member, native archive and isolated
payload/durable readback; complete remote ciphertext equality binds that proof
to the off-node artifact. Release records durable verified intent before normal
HUB startup and client resume. It requires no selected-HUB live seal or custom
namespace/firewall support. Natural timer/retention changes that cannot be
classified exactly block release; unavailable live callback telemetry is not
claimed.

Only the already-stopped historical operation3340764a7d24 uses separate fresh
current observation and authority without inventing original termination proof.
Its input repair creates an exact private runtime copy and verifies both copies
at every mount; original input directory custody and historical bytes remain
unchanged. A new rollback always captures current state and retains additive
contracts and migrations instead of replaying an earlier cut.

Interrupted cold finish obtains a fresh full current CLOSED-copy proof, both
complete ciphertext readbacks and a separate short finish-only capability.
Unknown loss, natural retention or ACK progress that cannot be exactly classified
blocks admission; the package does not claim universal resume after application
traffic has changed the store. Historical live-seal/postseal states explicitly
reject as unsupported before pause, copied broker startup or release. Their
source/evidence remains preserved outside the qualified package. Each admitted
finish checks resumed workloads and its deadline before release/publication;
failure retains truthful progress and fresh safety-pause obligations.
# Cold native proof and V11 continuity

Active cold deployment, rollback and expired recovery copy readbacks verify the
complete authoritative native archive, exact message-file inventory and hashes,
restored pinned-broker census/configuration, and complete decoded durable state
(including pending timestamps and redelivery maps). They do not require the live
bootstrap role to publish message GET requests and do not claim API-decoded
message semantics. Authenticated CMS decryption verifies every promised member
and repeats the isolated native restore/readback before release authority.

Complete consumer decoding uses authenticated private archive bytes. Archive
custody, manifest and hashes are checked before and after the read; live fence
checks remain at those phase boundaries and after all consumers are decoded.
Losing authority at the final boundary rejects the entire proof. Consumer count
does not multiply live cluster checks during this immutable-byte phase; every
consumer still receives complete native-state and admitted-decoder validation.

V11 preserves the V10 helper and original adoption, checkpoint, request journal,
response and recovery keys. Its separate upgrade record binds the replacement
code. The exact failed pre-proof recovery request is recorded without replay;
new recovery requires a fresh dispatcher and separately verified current proof.
Exact native continuity requires no inferred historical startup interval;
changed metadata remains subject to the pinned interval/checksum classifier.

V12 corrects the common state-reader routing after the V11 helper upgrade. The
reader selects the expired adoption verifier only for the exact current upgrade
receipt and then verifies the full preserved V11/V10/V9 code and adoption chain.
The historical V9 reader cannot be redirected by a newer receipt. Installation
preserves V11 code, its upgrade receipt and all original history; a separate V12
receipt binds the replacement and a separate disposition freezes the exact
failed pre-proof reader request. Neither record grants recovery permission.

V13 prefers an optional protected archive at
`installed/source-archives/<full-commit-sha>.tar.gz` in both source consumers.
Archives are passive data: each use still verifies the live approved commit,
complete Git tree, blob contents and executable modes, CI, image lock and image
identity. Only a missing archive uses the existing network download; an invalid
present archive blocks. The dispatcher filters CI runs by its exact validated
head SHA while retaining the existing response checks and deadline.

Installation preserves the installed V12 code and all earlier upgrade receipts,
adoption, checkpoint and request history. A separate V13 receipt binds the new
helper, and an immutable disposition records the exact failed pre-source
transport request without replay or recovery authority.

## Explicit retirement of the prepared historical operation

The root-only reviewed retirement action applies only to operation3340764a7d24
and the exact frozen prepared/unapplied checkpoint. The user's current-data
waiver permits loss of all current messages and consumers while retaining the
staging setup. Retirement restores the original twenty workloads with their
existing UIDs and templates, verifies configuration and selected storage, and
releases the prepared marker only after the selected setup checks. The original
strict action requires all workloads ready. The separately attested continuation
of the exact failed125-event attempt restores original desired replicas and
templates, requires HUB ready, and records actual readiness for all other apps;
it may close with unhealthy old apps so a normal FULL can repair them. It
preserves the complete historical checkpoint as an immutable snapshot and appends truthful retirement
events. The terminal state is ABORTED/OPERATOR_RETIRED/RELEASED, never a
deployment PASS or rollback source.

Normal idle admission requires the separately bound immutable plan, closure
receipt, exact historical event prefix, setup proof and named journal bytes.
An ABORTED status alone grants nothing. A root-only idle helper update accepts
the fixed installed V15 predecessor after that closure and preserves all old
operation code, keys, policy, receipts and request history. Unknown pending
requests still block. Neither action is exposed through the bridge or workflow.

The exact blocked pre-cut fence at operation `32151c4cb431` has a separate
root-only recovery path. Its checkpoint is `FENCE/BLOCKED/UNKNOWN` and contains
no cut or cipher, so restoring the captured workloads after the retained HUB
pod's orderly exit is an explicitly authorized disposable-staging recovery,
not a preservation PASS and not a rollback source. The recovery is pinned to
that checkpoint, journal/response hashes, namespace, selected PVC/PV, workload
UIDs/templates, marker, and the retained pod UID/container/finalizer. It removes
only that owned pod finalizer with UID/resourceVersion tests, restores the
captured workloads in the recorded dependency order, and releases the marker
only after readiness checks. A failure during restoration leaves the marker
fenced and a private progress step. If the marker-release patch succeeds but
closure persistence then fails, the marker can already be active; the private
progress step distinguishes that case. In either case, stop for root operator
inspection and reconciliation. Do not rerun the one-shot recovery or treat an
active marker as proof that closure/idle admission completed.

There is one narrowly bound continuation for the observed
`removing_owned_finalizer` progress step. It is not a retry of the Pod patch:
the continuation requires the exact unchanged pre-cut snapshot and progress
record, confirms the original Pod name and UID are absent, rechecks the marker,
storage identity, zero-replica workloads and absence of PVC writers, then resumes
at HUB scale. Its v2 closure explicitly records that the patch response metadata
was unavailable and that the exact Pod UID was observed absent; it leaves both
pre- and post-patch resourceVersions unknown. It cannot be used if the Pod remains,
the name is reused, any progress advanced beyond the finalizer step, or any
precondition changed. On any such mismatch, stop for manual reconciliation.
The normal patch path treats an omitted empty `metadata.finalizers` field as an
empty list but still requires the actual returned resourceVersion; no version is
inferred or fabricated.

The isolated operator container must present the same root-owned evidence
directory at both `/evidence` and `/var/lib/voice-nats-preservation`, because
the one-shot driver uses the former while the captured verifier uses its
canonical latter path. The paths must resolve to the same device and inode;
root:gid1000 mode `0750` is accepted, while group-write and all other-user access
are rejected. The captured manifest root contains `nats-known-baseline/` and
`nats-rollout-preservation/` directly beneath its `code/` directory.

The historical `STARTED` request rows are handled by a separate, root-owned
administrative abandonment receipt after the exact fence recovery closes. The
receipt binds the complete pending-row inventory and each unchanged journal,
response, request, operation checkpoint, and source-build state. It records the
explicit disposable-staging authorization; it does not claim those requests had
no effects or succeeded. Each row records whether a terminal checkpoint exists,
only a source-build directory exists, or the referenced checkpoint is absent;
absence is an observed filesystem state, never a no-effects claim. The request
and response operation IDs must agree, and checkpoint/build presence is
rechecked when the receipt is consumed. Journal and response files remain
immutable. A new, unknown, changed, or nonterminal row blocks admission, as does
any unreferenced nonterminal rollout directory. This disposition is
separate from the 321 fence closure and is required before the exact installed
`ceccb8a607957d9c436247fb183d8467d48ac33ece8b3e6c70850404c569e032` helper
predecessor can use `--upgrade-idle-ceccb-v1`; it does not broaden the existing
V15-only `--upgrade-idle` route. After that reviewed helper transition, any new
deployment must use the ordinary protected fresh-cut, encrypted backup and
readback, isolated restore, native preservation, and post-apply gates. The
fence-abort or abandonment receipts alone do not establish those gates.

The separately fixed source-capture rejection from run `38469787490` is not a
row in that abandonment receipt: `--upgrade-idle-source-capture-v1` checks its
exact request/journal/response hashes and the still-empty pre-capture workspace
directly. All other pending rows still require the full inventory disposition,
and all rollout, marker, key, policy, witness, and closure checks remain in
force.

Captured public code is hashed through an ordinary protected read-only file
descriptor so the unprivileged workflow can verify root-owned group-readable
source. Its complete manifest, ownership, mode, single-link, size, hash and
identity checks remain. Private state and native data retain their existing
O_NOATIME readers. Future deployments and new rollbacks use the normal protected
cold transaction and require fresh preservation and release evidence.


Actual owned Running Pods can identify a previous application image even when
the application is not Ready: a current container attempt and unique immutable
runtime imageID must match its owned ReplicaSet and current deployment template.
Missing or ambiguous attempts, foreign ownership and changed templates refuse
capture. HUB live census and graceful shutdown health requirements are unchanged.


Leaf readiness and monitoring are separate checks. The October 15 recovery
reached Ready for all 22 workloads but its leaf monitoring request failed because
the shared leaf configuration did not enable HTTP monitoring. Staging leaf
Deployment arguments now bind both the client and monitor to 127.0.0.1, with
monitor port 8222; the shared ConfigMap is preserved. The Realtime dual-leaf proof
uses client/monitor ports 4222/8222 and 4223/8223. Canonical target preflight rejects
missing, duplicate, public-host, or alternate leaf-monitor arguments and changed
config mounts before applying a target. An isolated pinned-image linked-Hub/two-
leaf proof verifies the actual listener addresses and the leafz response contract.

The fixed media_monitor helper identifies the Hub varz or allowlisted leaf leafz
request in command failure diagnostics. Command failures retain exit/timeout,
a fixed stderr reason, and the hash/length of bounded stderr, never its contents
in public output. Recovery consumers must retain that diagnostic with the current
failure and update the current checkpoint error; copied historical checkpoint
errors are not the cause of a newer failure. The ready-baseline-proof failure at
checkpoint cf937 must be read from its retained progress/failure records (the
historical error field still names the earlier Gateway timeout). Preserve those
bytes. Neither all-ready snapshots nor this monitoring correction authorizes
replaying an old runner or claims terminal deployment acceptance.


The exact cf937 monitoring-failure successor uses the separate
operator-media-target-monitor namespace and voice-media-target-monitor-closure-v1
receipt. Admission binds all 30 retained members, including the prior 21-member
history, cf937 bytes, gateway source/prepared/normalized/progress artifacts, the
202e runner and current command_failed record. It reconstructs the failed state
from the prior boundary plus progress, requires verified refence, all 46 resources
at the prior target with unchanged UIDs, and all 22 paused workload snapshots
including Hub separately. User bootstrap overrides, foreign rows, changed history,
old terminal namespaces and source/CI mismatch refuse admission. A fresh accepted
source capture and full normalized digest are required; only the new Deployment
templates enable monitoring. Existing ConfigMaps, storage, migrations and prior
runner/artifact bytes are retained. The new runner records current failure error
and safe monitor diagnostics without altering historical checkpoints. Independent
terminal readback and live Admin preservation remain mandatory before and after
helper installation; this closure still does not mean ordinary FULL or product
acceptance passed.

The subsequent ordinary FULL request `8d65ee47ce1b` failed before fencing: its
isolated Story writer called the real `CreateStory` RPC without a User privacy
provider and received `Unavailable`. HTTP health did not prove writer readiness.
The disposable probe now supplies `GetPrivacySettings` only for its generated
author and the `story` caller on loopback port 9091; the fixture sets
`USER_GRPC_ADDR` to that endpoint. The production Story privacy check remains
fail-closed. A successful writer must observe that RPC, preserve SQL/schema
assertions, and pass the independent wrong-count negative control (exit 17).
Rebuild `story-current4-probe` when changing this fixture; replacing Python alone
does not activate the dependency. Safe fixed writer failure labels and the
original command action are captured before Docker cleanup.

After reviewing the new bundle, `installer.py --upgrade-idle-story-fixture-v1`
accepts only installed binding `7f5b901d8429f2d560bb7f71481baabfd90ad2303cfd1b06c8569d94ff87920b`.
It requires the successful monitor checkpoint `bfa1426b` / receipt `f951ce87`
and the exact failed pre-fence request, source tree/custody, journal, response and
public report bytes. The request is classified `ABANDONED_PRECHECKPOINT` with
`BLOCKED` status; its STARTED journal and all retained evidence remain unchanged.
Any operation/build directory, changed source, request or terminal evidence
refuses admission. This upgrade authorizes no rollback or replay and makes no
ordinary FULL acceptance claim. Root must submit a fresh ordinary request and
complete deployment and product acceptance after installation.

Before swapping helpers, validate every installed journal together. The exact
Story disposition is excluded from the legacy abandonment census only after its
strict retained-evidence verifier succeeds; the legacy 29-entry receipt stays
byte-identical. An isolated new-request test misses this interaction: regression
coverage must include both inventories, corrupt Story evidence and an unknown
STARTED nonce, plus the complete installer admission path.

Check the reviewed ACL proof digest and generation before costly preparation.
When the source ACL changes, retain the reviewed delta and its signed-grant,
server-authentication and bootstrap API evidence before updating proof variables;
ACL compatibility does not establish business publication, ACK or product acceptance.
After full apply, preserved Services may have new resource versions from kubectl
bookkeeping. Finish still requires their original UID and complete desired
semantics, including labels, annotations, selectors and ports. Pre-apply version
checks and Secret preservation remain strict; this is not a general version waiver.

A failed finish request can remain STARTED after a separate finish completes the
same operation. Do not rewrite its journal, response or public report to make an
idle helper upgrade pass. Root may retain a private immutable
`terminal-finish-disposition-<failed-nonce>.json` with schema
`voice-terminal-finish-disposition-v1`. Its closed file map pins both finish
journals/responses/reports, terminal checkpoint and apply authorization; it binds
the operation and captured helper. Admission requires the same source, dispatcher
run, attempt and apply claim, a successful completion and released terminal marker.
The failed request remains BLOCKED. This grants no replay or new deployment authority.
Any retained Service RV reconciliation must include its original checkpoint and
receipt and reproduce the exact recorded metadata-only change. Both the legacy
pending census and per-journal admission verify this disposition, and the existing
idle upgrade transaction rechecks it after the code swap. Use that internal
transaction with reviewed exact predecessor/destination bindings and fresh immutable
receipt names; no new public upgrade command or failed-nonce exception is needed.

Historical source admission pins content and enforces safe live custody. Its
full metadata fingerprint is valid only within a read or across the locked
helper swap: `installed/sources` is shared, so a later operation legitimately
changes that parent's timestamps and link count. Do not persist that mutable
parent fingerprint as a permanent historical equality or repin it after each
operation. The Story boundary retains exact content, ownership/mode/link checks
and per-read race checks; the installer still refuses metadata changes during
its swap. Original per-entry metadata was not retained, so this does not claim
historical inode/timestamp equality.
