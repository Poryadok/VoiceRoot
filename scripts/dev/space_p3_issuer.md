# Missing Space identities in DEV

This bounded phase creates only `voice-space-principal-signing` and
`voice-space-media-server-tls`, and only after both are absent. It does not
rotate or create either CA, adopt existing Secrets, activate a workload, or
authorize a rollout. An unknown response or partially completed operation
requires reconciliation under the retained common lock; do not retry it.

The central operator supplies an existing Journal, Fence, client, lock and
namespace UID. The immutable `space_p3_create` intent contains `source_repo`,
the current candidate `source_sha`, and `roots`, mapping each of
`voice-principal-ca` and `voice-media-ca` to its exact `uid`,
`resourceVersion` and `der_sha256` captured from DEV. The phase re-reads those
public certificates and source on the active fence; preparation metadata alone
is not an operation claim. It checks that the current Space Deployment has no
reference to either missing Secret and retains its UID, generation and spec
through creation. Later adoption belongs to the guarded configuration phase.

`space_p3_issuer_create.Session(...).prepare()` reserves the one-shot handoff
and returns the PID, process start ticks, and socket identity. Human root must
independently authenticate the captured source bundle and live operator claim.
`space_p3_issuer_bundle.build_git(repo, candidate_sha, output)` builds a
root-captured launcher from the exact reviewed issuer and runtime source. Its
source tar must be placed at `/home/pmd/voice-space-p3/p3-source.tar` for the
launcher. Root separately needs the existing root-owned
`/home/voice-dev/issuance/generation1/media-ca.crt` and `media-ca.key` and a
private copy of the captured DEV media CA public certificate. No source or
public receipt establishes that this private key actually exists.

The launcher invocation takes `--operation-id` and `--media-public`; its
`--export` invocation additionally takes `--source-sha` equal to the reviewed
issuer SHA from `space_p3_issuer_source.ISSUER_SHA`, plus the exact consumer
PID and start ticks. Root source and operator authentication are independent
of those command-line identifiers. The issuer checks the existing CA's exact
DER and private/public key match, writes a private one-shot packet, and sends
it once through a root-authenticated sealed memfd. `finish()` validates the
packet, dry-runs both fixed objects, then records a pending operation before
each create and verifies UID/RV and full content after each response.

`verify_completed(session)` rechecks the operation journal, live Secret
contents, CA identities and issued cryptography, and returns only public JWKS,
TLS fingerprint and identities. The operator then captures the now-existing
Space public TLS certificate with the existing three-public-input path before
attempting the independent Realtime seven-object P3 handoff. A successful
receipt does not prove that Space is running or serving the JWKS; that is a
separate post-adoption health check.
