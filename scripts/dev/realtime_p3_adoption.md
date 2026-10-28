# DEV Realtime startup planning contract

`realtime_p3_adoption.plan(original_deployment, admitted_image, assets_receipts)`
is a pure renderer for the central DEV operator. It neither reads the cluster nor
creates credentials, runs migrations, installs peers, applies patches, or grants
rollout authority. It returns a JSON Patch testing the Deployment UID,
resourceVersion and entire original spec before replacing it; the target spec;
and SHA-256 of the canonical JSON receipt (sorted keys, compact separators).
The operator must journal the original spec, patch, target, admitted image and
receipt digest under its existing fence/claim and recovery protocol. A target
match alone is not proof of ownership. Read back all identities under that claim
and stop on any change; Deployment CAS does not protect other objects.

The receipt has exactly `assets`, `schema`, `public_hashes`, `service`, and
`transport_evidence_sha256`. `assets` maps each fixed name in `ASSETS` to
`kind`, `namespace` (`voice-dev`), `uid`, `resourceVersion`, and `keys` (a list
of key names, never values); `voice-p3-principal-public` additionally requires
`immutable: true`. `public_hashes` maps every name in `PUBLIC_HASHES` to the
lowercase SHA-256 of the corresponding public document, certificate or trust
root bytes. `schema` contains `database: realtime_db`,
`migration: 000001_admission`, and `evidence_sha256`. The latter and
`transport_evidence_sha256` identify independently verified evidence; their
presence and syntax alone prove nothing. Do not include credentials in receipts,
logs, or journal output.

`service` contains the name, namespace, UID, resourceVersion, type, clusterIP,
selector and ports from the
`voice-dev/voice-realtime` Service. Its selector must match the target Pod labels
and its `realtime-lifecycle` TCP port must reach 9091. The operator must verify
the Service identity and that its selector excludes unrelated Pods when collecting and rereading this
receipt. An absent or mismatched Service holds adoption. For an existing Service
without the lifecycle port, `plan_service(original_service, expected_pod_labels)`
returns a separate UID/RV/full-spec CAS and target adding only TCP 9091. It
rejects collisions and does not create or apply anything. The central operator
must fence, journal, apply and reread that transition before preparing the
Deployment receipt; an already matching target is not proof of ownership.

Before constructing a receipt, the operator must independently establish a
restorable backup and actual migration state, and bind their evidence to the
database identity and exact source/migration bytes. Verify current/next distinct
PKCS#8 RSA signer keys match the immutable public JWKS; TLS key/certificate pairs,
SANs, EKUs, chain and scoped roots; Redis replay connectivity and authentication;
protected Chat listener and Space HTTPS JWKS availability and identities; and
the independent Realtime HTTPS publisher consumed by Chat. The DEV DNS values in
the renderer follow the canonical staging template with the DEV namespace and
must be verified against real peer resources and certificates. Existing
identities must be preserved. Missing or unverified peers/assets mean HOLD.
Asset UID/RV/hash readback and evidence validation are external to this planner.
The operator also validates the image's release policy/profile, source-bound
snapshot and mounted files and its alpha presence dependency before startup;
this renderer does not qualify those independent requirements. It rejects the
optional Space JWKS client certificate environment variables because their
identity and mounts are outside this fixed contract.

This is a startup prerequisite, not public P3 activation. Realtime documentation
requires additional writer coverage, eleven-participant release coverage and a
qualified recovery/rollback package for public activation. See
`docs/DEPLOYMENT.md`, `docs/microservices/realtime-service.md`, and
`deploy/templates/migrate-realtime-db-job.yaml`.
