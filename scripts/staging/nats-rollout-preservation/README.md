# Per-rollout NATS preservation

Normal staging deployment uses a root-owned captured bridge installed once by
the operator. Every subsequent release creates a new operation and cold cut;
an earlier reset baseline or successful backup cannot authorize another release.
The selected PVC/PV UID and physical path remain bound throughout the operation.

The workflow prepares the cut, proves an isolated restored copy, uploads only
an authenticated encrypted CMS backup to GitHub, independently reads the entire
artifact back through root, authorizes one paused apply, proves the original
native bytes and complete account census again, and then resumes applications.
No bootstrap Job runs against the retained store. NATS ConfigMaps, Secrets,
credentials, streams and consumers are retained. A failed proof retains the
owned fence and reports BLOCKED; it does not restart clients or overwrite data.

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

The hub renderer is a separately bound root-only init image transition; the
runner cannot apply arbitrary hub changes. Exact captured inputs and private
old/new output equality precede startup. Actual completed init identity,
generated configuration and unchanged broker are checked after hub readiness
and before applications resume. Failure refences; native migration/CAS still
require closed writers. Renderer retry validates the recorded completed DB
state rather than rerunning migrations or assuming the original version.

The first compatible populated backend witness is the reviewed Story image
pair, whose four schema UP files are identical and whose old binary has passed
the documented text-write probe on current clean4. Root independently binds
the actual mounted Story actor, application DSN, PostgreSQL authority, clean4
and exact image/source witness; the disposable probe is not a live receipt.
Unknown old Story content and unproved backend rollback selections, including
historical Bot/Search, fail closed. Ordinary frontend rollback remains separate.

A rebuilt current Story component requires its own actual-binary/current4
witness even when its source has not changed. Compatibility admission must bind
the exact child manifest, config, application binary, component build source,
four migration hashes, actual fixture receipt and successful publisher evidence.
Historical caller/schema fixture inputs remain separate from the new image's
build source. Current source CI approval remains a separate deployment gate.

The V6 root-only witness promotion pins reviewed immutable evidence bytes;
the runner cannot register a witness or supply a compatibility boolean. Both
captured Story preflight and fresh rollback use the same selected root authority.
Missing, revoked, corrupt or drifted authority vetoes before mutation. The
captured witness digest is rechecked after custody and before authorization;
schema, PostgreSQL, mounted actor and target-contract checks remain mandatory.
The root-private registry retains at most 128 approved child/config pairs, so a
new current pair does not erase the prior compatible rollback pair. Every
selection binds the complete monotonic registry revision. Any registry change
during an operation vetoes revalidation; revocation cannot silently re-enable
the same record. Conflicting evidence for an existing pair is rejected.

Delivery has two phases. First review, merge, publish and install the generic
V6 helper using its exact immutable assets and fixed V5 predecessor. After the
final master image is published, run and review the actual image/current4
fixture and its publisher/component/schema evidence, then publish that evidence
as separately pinned public data. This evidence contains no credentials and is
not compiled into the helper. An explicit human-root command promotes the
reviewed file; neither pmd nor the workflow can approve it:

```sh
/usr/bin/python3 -I -S /var/lib/voice-nats-preservation/installed/code/nats-rollout-preservation/story_witness.py --promote /root/reviewed-story-witness.json REVIEWED_WITNESS_SHA256
```

The supplied file must be root-owned, private, regular, single-link and read
through trusted ancestors. Root promotion takes the existing global operation
lock, checks installed code and idle operations, and rereads the unchanged
active marker before its atomic selector write. Use `--revoke` with the same
record path/hash for explicit revocation. Code upgrades preserve every registry
and immutable record byte; unknown partial files or corruption veto the upgrade.
The V5 operator has no intake, so newly rebuilt content remains blocked until
V6 and the separately reviewed evidence are installed. Promotion is scoped
binary/schema compatibility authority, not current CI or live rollout proof.

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
seed retain all existing claims and deny rules; only the four documented fixed
publish permissions are added. A create-only immutable new bootstrap Secret is
authenticated with its existing reply prefix in a pinned network-none scratch
broker before an atomic root enrollment. The old Secret and Windows signer
original remain intact. Never paste seed, JWT or credentials into a receipt.

Normal backend prepare binds that enrollment, the approved source, pinned
server and complete current contract. The authoritative closed native cut and
encrypted full off-node readback precede the two fixed stream UPDATEs and one
new-policy durable CREATE. Preserve every old record, existing consumer
config/ACK/pending/redelivery state and dynamic resource. The post-config cold
ledger is the app-apply baseline; do not compare changed config bytes to the
pre-migration ledger or restore that old archive. Only proved pinned recovery
timestamp observations are classified; durable and untouched-stream creation
identity remains strict. The public script ConfigMaps and root active-contract
receipt advance only after the complete proof, without replaying bootstrap Jobs.

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
undoes database migrations. Full/app-only rollback needs a separate explicit
database compatibility plan and is rejected by this images-only endpoint.

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
such failures retain the fence with an explicit recovery status. Checkpoints
are bounded 128 MiB and monitoring 64 MiB; supported capacities are finite.

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

Before accepting a cut, a pinned isolated broker restores the closed native
archive; one actual account monitoring snapshot feeds both census and contract
compilation. Original plan, binding and selected scripts must match, and root
rechecks actor/credential, PostgreSQL, non-NATS, kernel, target and current
execution authority. Rejected startup or proof removes only its owned copy;
unverified cleanup cannot return an accepted cut. Interrupted capture or partial
outputs cannot be replayed. Completed encrypted output recovery requires the
same fresh execution nonce/run/head and operation binding. Ordinary encrypted
off-node readback, paused authorization, apply and release guards remain required.
