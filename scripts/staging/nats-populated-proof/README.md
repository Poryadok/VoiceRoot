# Populated NATS proof: isolated-deployed-config-clone-v1

This separate, versioned scenario proves current credential authorization and
populated delivery behavior in an owned isolated clone. It never publishes,
subscribes, acknowledges, creates or deletes anything on staging. Existing
rotation behavior, its empty-stream live guard, staging proof hash variables and
the `NATS_LIVE_ACL_PROOF` gate are unchanged. Its output is **not** that gate's PASS.

Production execution is deliberately disabled until an independent custody
authority is enrolled. Development uses disposable test identities only.
Historical retention/migration remains a separate open prerequisite for A1.

## Exact coverage

| Required claim | Evidence in this scenario | Limit |
| --- | --- | --- |
| Current ACL intent | Exact signed publish/subscribe/deny sets and no response/bearer bypass for all 19 services and bootstrap | Static grant validation, not delivery tests of every product event |
| Operator/APP/SYS trust | Signature chain, actual account identities, claim validity, APP JetStream enabled and SYS disabled; user seeds match signed user IDs; revocations and expiry checked | Direct account issuer only; delegated or external authorization fails closed |
| Account limits/config fidelity | Original signed account JWT bytes, including limits, are copied unchanged; witnessed mounted config bytes and pinned actual image must match supported configuration | Does not increase limits or migrate historical consumers |
| Mounted deployed identity | Independent signature covers immutable Secret UID/resourceVersion/key, opaque file SHA256, active generation, cluster/pod UID, capture time, release/ACL identity and image digest | Requires independently trusted capture; a source hash cannot establish live provenance |
| Positive current grants | Bootstrap creates exact same-name social stream and three durables, populates it, and successfully updates the duplicate window; Social PUB receives server PubAck; Realtime receives correct payload and sends actual server-issued ACK | Synthetic clone data only |
| ACK durability | Exact consumer ACK floors survive hub restart, both for direct JWT clients and authenticated service leaf routes | Does not prove live scheduling/connectivity |
| Negative authorization | Explicit server permission errors for cross-profile SUB/ACK, Realtime consumer CREATE, and bootstrap business PUB | Timeout, disconnected broker or missing message never counts as denial |
| Six newly required grants | Omit each grant individually and the full input contract fails | Old deployed credentials cannot qualify by rehashing old evidence |
| Populated state preservation | Independent source fixture payload/config/stream/consumer ACK snapshots remain identical across success, exercise failure and ambiguous cleanup failure | Does not prove historical eight-message/566-consumer preservation |
| Cleanup/isolation | Owned resource deletion and empty owner inventories required; uncertainty vetoes success | Docker daemon/custody host are trusted infrastructure |

The runtime exercises Social, Realtime and bootstrap identities. Other service
profiles receive complete signed-intent validation; this scenario makes no claim
of their product-level runtime delivery. Candidate credentials are explicitly
labelled `candidate`; they cannot prove that staging mounted those credentials.
Disposable fixtures are labelled `fixture` and can only emit `FIXTURE_PASS`.

## Source and runtime isolation

The host controller may consume an approved protected opaque bundle, but does not
have a Kubernetes client, SSH operation, secret issuance or rollout operation.
The proof containers receive owned copies only. The hub mounts only its rendered
config, operator JWT and TLS files, plus an owned Docker volume. A separate owned
diagnostic holder mounts only Bootstrap/Social/Realtime keys for direct probes;
it receives no operator/account material or storage. Leaves mount only their
exact credential key, CA and config, at the same
paths used by staging. User authentication seeds necessarily accompany user
credentials; **operator/account signing seeds never enter inputs or containers**.
There are no business services, live payload copies, source PVC mounts, Docker
socket, kubeconfig, ServiceAccount token, privileged containers or published
ports. The read-only helper container filesystem drops all capabilities and
uses `no-new-privileges`.

The only network is a newly owned internal bridge. IPv6 is disabled and
`com.docker.network.bridge.inhibit_ipv4=true` prevents assigning a host gateway
address to the bridge. Network and container settings, every mount and actual
content-addressed image identity are inspected before exercise. This follows
[Docker's bridge option contract](https://docs.docker.com/engine/network/drivers/bridge/).
The controller creates containers without starting them, verifies actual local
volume name, owner label, mountpoint, local driver and empty driver options,
and requires every expected mount exactly once before starting a broker.
The helper only uses the hub IP captured from that inspected owned network, or
its own `127.0.0.1:4222`. A caller-set environment string is insufficient: a fixed
readonly phase context must match the actual Docker hostname/container-ID mounts,
role, interface/IP, no-external-route and capability boundaries, readonly owned
mount coordinates and copied file hashes before credentials are read or dialing
starts. The context describes parent-inspected isolation; it is not an independent
authority for deployed identity. The controller still requires the separately
enrolled witness policy. Mounted config
must match the current staging hub/leaf grammar, including its non-JetStream
`default_js_domain` compatibility mapping; includes, alternate remote endpoints,
imports, exports, subject mappings and external account authorization fail closed.

Docker-managed baseline mounts are limited to kernel pseudo-filesystems and
the matching container's hostname/hosts/resolver files. Readonly cgroup v1
controllers and the pinned Alpine `/var/run`→`/run` alias are covered explicitly;
writable cgroup controllers and source filesystems under those paths fail.

The allowed clone differences are Docker placement/ownership labels and owned
storage, rather than Kubernetes scheduling/storage. Config, credential identities,
grant subjects, stream names, durable names, TLS trust and image remain unchanged.
Direct JWT probe clients are additional owned diagnostics; real leaf delivery is
checked separately. The clone does not attest Kubernetes security policy behavior.

Cleanup attempts every owned container, volume and network, deletes owned opaque
copies, and checks all owner inventories, including stopped containers. Every
failure yields a stable sanitized code. Raw Docker/NATS diagnostics and protected
input bytes are never emitted. Failure before or during exercise cannot emit PASS.

## Protected input contract

Linux custody execution requires mode `0700` bundle directories and regular
non-symlink mode `0400`/`0600` files. `manifest.json` is strict JSON; its exact bytes
are signed with the independent witness's Ed25519 key. `manifest.sig` contains
only a base64 detached signature. Required files are:

- `operator.jwt`, `account.jwt`, `system-account.jwt`, `account.public`,
  `system-account.public`;
- `tls.crt`, `tls.key`, `ca.crt`, rendered mounted `hub.conf`, mounted `leaf.conf`;
- `bootstrap.creds` and the 19 `<service>.creds` keys listed in `acl-intent.yaml`.

The manifest has `schema`, `kind` (`deployed`/`candidate`/`fixture`),
`namespace` (`voice-staging`), `generation`, `captured_at`, `release_sha`, `acl_sha`,
`cluster_uid`, `hub_pod_uid`, `hub_image`, `leaf_image`, `files` and `mounts`.
Each files entry is SHA256 of its opaque bytes. Each corresponding mounts entry
has `resource`, `uid`, `resource_version`, `key`, `immutable`. Credential/JWT/key
sources must be immutable. Capture must be at most 15 minutes old; credentials
must remain valid for at least 10 minutes. Missing inputs or an unsupported
configuration block execution rather than silently substituting fixtures.

The independent trust policy lives outside the bundle. Its strict JSON fields
are `public_key` (base64 Ed25519 public key), `cluster_uid`, `hub_pod_uid`,
`generation`, `release_sha`, `acl_sha`. Its digest and source are reviewed
independently of the witness bundle. Same-bundle key material and a caller's
self-created policy cannot establish authority.

## Finite custody enrollment and approval path

The infrastructure/secret-manager owner owns capture and attestation; the A1
reviewer approves the public policy and actual mounted-input provenance. Neither
the agent nor the bundle producer can approve their own evidence. Enrollment is
not part of this PR and currently `approvedTrustPolicySHA` is empty.

1. For development staging, the user selects the existing `pmdebook` Linux node;
   no separate Linux machine is required. Separately approve a root-owned private
   temporary custody directory there, the secret-manager witness identity, and
   read-only mounted-input capture. Production identity/ACL requirements remain
   unchanged. The authority's public
   key identity must be delivered through an existing independently approved
   secret-manager/operator channel. Keep its private key and all credential
   bytes outside Git, chats, agent logs and artifacts. No signing key is
   provisioned by this tooling.
2. The approved operator binds the exact current generation/cluster/pod/ACL and
   release identity to a public trust policy. Independent review verifies its
   origin and digest, the actual hub/leaf mounted configuration and actual image
   ID, and the credential Secret UID/resourceVersion/key mapping. A producer's
   self-signed attestation is insufficient.
3. A separate narrowly reviewed source change sets **only** the public approved
   policy SHA256 in `main.go` and records its approved provenance. Required checks
   are the same unit/Docker targets plus positive approval and wrong-key,
   wrong-policy, generation/SHA, stale-witness and fixture-kind negative cases.
   That reviewed artifact is the authority; a CLI argument cannot replace it.
4. Only after separate user approval may the operator export opaque mounted
   inputs and sign their fresh manifest into approved custody. Run the reviewed
   Linux binary with `--inputs /secure/bundle --acl /reviewed/acl-intent.yaml
   --approved-trust /secure/approved-policy.json`. Do not pipe bytes into chat.
5. If current service credentials lack any grant, stop. A separately approved
   service-credential update may be required. That later plan must preserve the
   current operator/account, all broker storage/messages/consumers/ACK state and
   immutable resources, and cannot assume root rotation or delete/recreate.
   Candidate proof remains candidate until mounted provenance is approved.
   Future issuance can run as an approved protected root operation on the same
   `pmdebook` node. It needs the existing APP account signing seed (or an already
   authorized account signing key) through approved custody, plus TLS and the
   reviewed policy. No operator/root identity rotation is implied. If that seed
   is unavailable, restore its existing protected backup or record a missing
   decision; do not invent a new account/operator. No key is requested or issued
   during this development slice.
6. Independent review must accept this versioned proof scope before any proposed
   gate integration. Historical census/classification, retention and replay/
   idempotency evidence and live read-only deployment health are additional
   inputs. Existing gate variables stay unchanged in this tooling.

Without that separate approval/enrollment, production execution returns
`custody_trust_anchor_not_enrolled`; no old LIVE PASS or new A1 acceptance is claimed.

## Verification

From this directory:

```text
go test -count=1 -timeout=90s ./...
go test -tags=dockerproof -run TestDockerPopulatedProof -count=1 -timeout=4m ./...
```

The explicit Docker target is mandatory for this scenario. It builds a disposable
Linux helper, uses the pinned NATS 2.12.12 image, exercises the full clone and a
separate populated source broker, injects exercise/cleanup failures, and removes
all owned containers/networks/volumes and opaque fixture files. It does not skip
when Docker is missing. Ordinary tests pin the same embedded NATS server version.
`.github/workflows/nats-populated-proof.yml` runs both scoped targets on Linux
with read-only repository permissions and pinned checkout/setup actions.
