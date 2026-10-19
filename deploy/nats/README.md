# NATS JWT credential contract

This directory defines the Kubernetes JWT activation contract. It does not
change `docker-compose.yml`; Compose remains outside this rollout.

`secret-contract.example.yaml` is a placeholder-only Kubernetes interface for
both `voice-staging` and `voice-prod`: render `__VOICE_NAMESPACE__` to the
target namespace in the secret manager, never in source control. Values use
`stringData` and are UTF-8 text; Kubernetes encodes them to `data` on write.
The exact required keys, TLS SAN, and staging delivery bundle are specified in
[`docs/DEPLOYMENT.md`](../../docs/DEPLOYMENT.md#nats-jwt-and-leaf-activation).

The non-root renderer init container receives `voice-nats-operator/operator.jwt`,
`account.jwt`, `system-account.jwt`, `account.public`, and
`system-account.public`; after signature/claim validation it writes the distinct
`APP` and `SYS` MEMORY resolver preload to a memory-only config file. The main
hub mounts only `operator.jwt` at `/etc/nats/jwt/`, the rendered config, and the
TLS Secret `voice-nats-hub-tls` (`tls.crt`, `tls.key`, `ca.crt`). Its leaf
listener is Service port 7422. It must mount one
`voice-nats-service-credentials/<service>.creds` entry into each matching pod
at `/var/run/nats/creds/<service>.creds`, read-only with `defaultMode: 0400`.
No pod receives the shared Secret wholesale and no app pod receives an operator
or account seed. The account and operator signing seeds are secret-manager-only
rotation material, deliberately absent from this manifest.

For disposable Compose or staging rehearsal, generate fresh fixtures from the
pkg Go module:

```powershell
cd src/backend/pkg
go run ./cmd/nats-jwt-fixture C:\temp\voice-nats-acl.yaml C:\temp\voice-nats-fixture
```

The reviewed policy is [`acl-intent.yaml`](acl-intent.yaml). It enumerates all
19 services and the Job-only bootstrap identity with exact event, consumer,
pull and ACK permissions. Service users cannot create, update or delete
JetStream state. The bootstrap identity has only the scoped stream create/info
permissions it needs, plus exact in-place update permissions for the
`chat_events`/`voice_events` subject additions and `social_events` duplicate window; it cannot delete streams. Reply inbox subscriptions use scoped service prefixes, never
`_INBOX.>` or `$JS.API.>`. The disposable fixture command above is for proof
only; its signing seeds are discarded.

For staging or production issuance, use the separate Linux-only issuer with a
fresh absolute destination and external TLS certificate/key/CA PEM files. The
certificate must chain to that CA and contain an exact `voice-nats` DNS SAN.
Run this on an isolated trusted Linux host/runner with a protected parent
directory. The shared staging `pmd` host is not an issuance location because
its UID also holds SSH and Kubernetes credentials; do not copy output or
signing seeds into `/home/pmd`. Prefer an ephemeral private filesystem,
transfer the `secrets.json` restore bundle through the approved secret-manager
stdin path, and separately back up the three signing seeds there:

```bash
cd src/backend/pkg
go run ./cmd/nats-jwt-issuer --namespace voice-staging \
  --tls-cert /secure/voice-nats.crt --tls-key /secure/voice-nats.key \
  --tls-ca /secure/ca.crt ../../../deploy/nats/acl-intent.yaml \
  /secure/fresh-nats-issue
```

The destination must not exist. Output files are mode `0600`, directories
`0700`. `secrets.json` is the staging restore contract: a Kubernetes `List`
with exactly four Opaque Secrets in the target namespace, using base64 `data`
and the keys listed in `secret-contract.example.yaml`. Individual Secret YAML
manifests are also emitted. `signing-seeds/{operator,app-account,system-account}.seed`
must be backed up separately in the secret manager for rotation; they are not
part of the Kubernetes List. Never commit or print the destination. The issuer
fails if the destination exists or if Linux permissions, ACL, or TLS validation
fail. Staging restore transports only gzip+base64 of `secrets.json` via secret
stdin; the hosted proof reports its byte count against the 48 KiB limit.

`bootstrap.creds` is a separate, short-lived provisioning-Job credential in
`voice-nats-bootstrap-credentials`. It
must be stored and mounted separately from `voice-nats-service-credentials`;
no application pod or leaf sidecar may receive it.

## Existing account additive rollout prerequisite

The retained staging generation `r20260930a4` has an immutable bootstrap
credential Secret. The October 6 signed-claim projection confirmed that its
existing user lacks the two exact stream UPDATE permissions and the new
`social_events/rt_realtime1_friend_removed` CREATE/INFO permissions. Current
source policy includes those four subjects; this does not update deployed JWTs.
The existing reply prefix remains `_INBOX.voice.bootstrap.reply`, with no
unrestricted inbox grant.

Renew only that same bootstrap user under the same APP account and existing
user seed, using the protected existing account signer. Preserve every other
claim, deny list, expiry, response policy and connection limit. The general
fresh issuer above creates new identities and must not be used for this upgrade.
A scoped signer that cannot grant the exact policy remains a preflight veto;
never change signer/account JWT to evade its scope.

Create a new immutable bootstrap-only Secret and enroll its exact identity and
credential hash under root custody for the active generation. Preserve the old
immutable Secret, all operator/account/service identities, hub/PVC, and completed
bootstrap Jobs. Enrollment requires isolated authentication/INFO with the
existing reply prefix and atomic root-owned generation/UID/RV bindings. No
implicit issuance, Secret replacement, bootstrap replay or migration is allowed
through an ordinary deployment. The bounded renewal/enrollment and additive
migration operator require their separate reviewed implementation before this
stage's backend rollout can proceed. Application-user policy changes remain
independent prerequisites for each affected target service.

### BE211 ordinary Auth observations: activation prerequisite

The reviewed fresh-generation policy grants Auth exactly `user.registered`,
`user.logged_in`, `user.logged_out`, and `user.2fa_enabled`. The central
`user_events` selector and realtime/search/analytics-chat bootstrap contracts
capture those subjects. Auth has no stream administration or wildcard event
grant. Canonical JWT/TLS hosted proof requires four `user_events` PubAcks and
denies neighboring events, stream UPDATE and foreign inbox operations. The Java
binary-delivery fixture uses the actual Auth allowlists and central selectors.

This source change does not renew retained Auth JWTs or mutate retained streams.
An older generation missing grants or selectors is an activation veto:
observation drops are allowed degradation, not delivery proof. All three
existing-stream bootstrap checks still reject selector drift. Do not replay
bootstrap, widen to `user.>`, delete/recreate streams, or reset consumers.

Retained activation requires a separately reviewed protected extension of the
existing same-user service-renewal and additive stream-migration operators;
current operators deliberately do not admit Auth renewal or `user_events`
updates. The extension must pin the existing Auth user/APP account and policy,
preserve every other claim/deny/expiry/limit, add only four exact publish
subjects, and reject insufficient signer scope. It must validate the complete
current stream config against a frozen predecessor, add only four selectors in
place, preserve all other settings and message/consumer state, and journal
before/after hashes, census and readback for resumable execution. Only a
separately authorized operator may obtain scoped UPDATE capability and enroll a
new immutable Auth-only credential generation. Existing credentials/records
remain retained. Until that implementation and execution evidence exist, fresh
disposable proof is valid but retained rollout remains open. BE-211 source
review does not authorize rotation or deployment.

### Bounded Role, Space, Voice and Messaging grant transition

BE-102 adds exactly `voice.participant_joined` and `voice.participant_left` to the
Voice publish policy and `$JS.API.STREAM.UPDATE.voice_events` to bootstrap.
Central bootstrap accepts only the complete legacy Voice subject set or the
complete new set; it adds the two subjects in place while preserving every other
stream setting, stored message/sequence and consumer identity/ACK state. Drift
or an update/readback error fails closed; no stream or consumer is recreated.
The executable preservation fixture is required in the Linux bootstrap contract
job, and the real disposable JWT/TLS broker proof remains a delivery gate.

These source permissions do not change retained deployed JWTs or authorize
issuance, Secret replacement or bootstrap replay. Existing-generation rollout
must first prove the exact new Voice/bootstrap grants under its separately
authorized renewal/enrollment procedure. Missing grants remain a preflight veto.

`nats-jwt-issuer --renew-existing-service PRIVATE_PIPE_FD` is an explicit
Linux-root operator mode, separate from fresh issuance and bootstrap renewal.
Its stdin is one bounded JSON document with schema
`voice-nats-existing-service-renewal-v1`, `old_creds`, `account_jwt`,
`operator_jwt`, `signer_seed`, `actor_sha256`, `account_sha256`, `service`,
`publish`, `subscribe`, and `no_response`. The target grant must come from the
reviewed `acl-intent.yaml`; private credentials travel only through stdin and
the inherited result pipe. Arguments, environment, stdout and stderr never
carry private material. Ordinary deployment does not invoke this mode.

The only permitted additions are `role.voice_policy_invalidated` for Role,
`space.voice_room_access_invalidated` for Space, the exact INFO and ACK subjects
for `chat_events/voice_space_media_chat` and
`role_events/voice_space_media_role` plus the two BE-102 participant publish
subjects for Voice, and
`$JS.API.CONSUMER.INFO.file_events.messaging_music_metadata` plus
`$JS.ACK.file_events.messaging_music_metadata.>` for Messaging. The target
publish set must equal the original set plus the service-specific additions,
and subscription authority must match unchanged. Messaging retains its existing
owned `_INBOX.voice.messaging.>` subscription, which covers the Music inbox;
renewal adds no inbox grant. Messaging receives no consumer CREATE permission,
arbitrary Music publication, or `file.processed` publish permission. Concrete
source inboxes already covered by the same existing owned
`_INBOX.voice.<service>.>` tail do not require extra JWT entries; the original
subscription claims remain intact. Missing inbox coverage or any subscription
deny overlap vetoes renewal. A concrete deny inside an ACK tail vetoes renewal,
as does a wildcard deny. Other services, wrong actor names, signer changes,
scoped signers, unsupported effective claims, expired/revoked actors and
unrelated grant changes are refused. Existing identity, user seed, account,
deny lists, expiry and all other signed fields remain unchanged; only publish
additions and signing issuance metadata may change. An already-current
credential is returned intact.

For retained staging generation `r20260930a4`, the separately reviewed human-root
operator must capture the immutable service Secret and operator/marker bindings,
back up the original Secret privately, and verify all 19 signed actor chains.
Verify prospective exact ACL compatibility and isolated broker authentication
before mutation. Under the existing operation lock with no active rollout,
stop each affected service (Role, Space, Voice and/or Messaging) whose signed
grant is stale, and observe its Pods gone. Replace only the exact generation
Secret using Kubernetes delete UID/resourceVersion preconditions, then create it
under the same name with all original metadata and every unaffected credential
byte unchanged. Verify readback and fresh signed/broker proofs before restoring
the affected replica counts. The read-only existing-service actor preflight
binds each selected `voice-<service>` deployment, credential Secret key and
mounted identity to this ACL before preservation; it never issues credentials
or writes a Secret. A missing or stale grant remains a hard preflight failure
until the human-root issuer renewal and guarded Secret update are completed. A
concurrent object/spec change is a veto; an ambiguous failure requires
private-journal recovery, never a blind retry. Keep the original backup until
the new FULL deployment and current-service proof complete. No operator/account
JWT, hub, PVC or bootstrap Secret changes are part of this transition.

## Per-service leaf topology

[`leaf-sidecar.template.yaml`](leaf-sidecar.template.yaml) documents the
selected rendering contract used by staging and production. Compose remains
outside this rollout. Each rendered workload
adds one `nats-leaf` sidecar with an app listener restricted to `127.0.0.1:4222`;
the matching application is configured only for that loopback endpoint and has
no route or credential that can reach the hub directly.

The sidecar mounts exactly one `<service>.creds` Secret key at mode `0400` and
uses it for an outbound TLS leaf connection. Its CA and expected hub server name
are explicit. Operator/account JWTs mount only in the central NATS workload,
under `/etc/nats/jwt/`, consistent with the Secret contract above; no app or
sidecar gets them, and no seed is represented in the template.

The hub alone owns fixed, centrally pre-provisioned JetStream consumers. The
four bootstrap Jobs own 40 durables. A preexisting durable with a different
filter, delivery subject, policy, group, or push/pull mode fails bootstrap and
requires an explicit migration before the application is started. Auth's old
random-target `auth_subscription_tier` cannot overlap the new no-group push
binding during a rolling start. Staging migration is an additional hard gate:
the existing anonymous `$G` account was observed with 566 consumers, while
the issued APP account limit is 512. Inventory and classify the legacy
dynamic consumers and prove a state-preserving migration of the existing
eight stream messages before account cutover. Do not silently raise the cap,
discard consumer state, or recreate the hub on empty storage. Rotate a service
credential by replacing its external Secret key and restarting only that
service's sidecar; rotate
account/operator material through its separate rehearsed broker rollout.

### NATS 2.12.12 non-JetStream leaf compatibility

The staging per-service `$G` leaf config sets an empty `default_js_domain`
mapping as a temporary NATS 2.12.12 compatibility bridge. This prevents that
version's automatic `$JS.API.>` deny from blocking ordinary JetStream API
request/reply traffic across a non-JetStream leaf. The isolated NATS 2.12.12
two-server regression test verifies the map is required on the leaf; adding it
to the APP hub alone does not restore the route. This does not enable JetStream
or storage on the leaf and does not widen the Search user JWT permissions; the
exact API publish and scoped inbox subscribe ACL remains the authorization
boundary. Staging hub and leaf images pin the official multi-platform
`nats:2.12.12-alpine` manifest digest
`sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb`.

NATS marks `default_js_domain` as temporary backwards-compatibility behavior.
Before upgrading the NATS server, migrate to a named JetStream domain and the
matching `$JS.<domain>.API` request subjects, with ACLs and consumer clients
updated and tested together. Do not remove the leaf mapping or enable leaf
JetStream as a workaround. This bridge is limited to staging's
currently verified NATS 2.12.12 runtime; production configuration is unchanged.

Auth's fixed subscription-tier durable uses a five-delivery budget with an
increasing server backoff. Persistent processing or unsupported-payload errors
are copied with their original bytes, SHA-256, source sequence, and reason to
the centrally provisioned `subscription_auth_quarantine` stream (P400D) before
the source delivery is ACKed. If quarantine publication fails, Auth NAKs the
source and logs an operator-visible error; the source remains retained for
recovery and JetStream emits its max-deliver advisory when the retry budget is
exhausted.
