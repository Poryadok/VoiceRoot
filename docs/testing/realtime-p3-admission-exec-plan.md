# Realtime P3 durable admission execution plan

Task: BE-A2-REALTIME-P3-PARTICIPANT. Owner: Рудеус-172, developer,
gpt-6.1-sol/medium. Base: d4abb829622e9bd91b26842b61522e6840840867.
Consumer: existing Space BE005 coordinator; additive participant11. Decisions:
BE-REALTIME-P3-DURABLE-ADMISSION-v1 (TT c3-c5),
BE-REALTIME-P3-JWKS-BOOTSTRAP-v1 (independent Astra c9-c10).

Sources: AGENTS, PLAN A2, realtime-service.md, space-service.md, DATA_MODEL,
DATA_STORES, ARCHITECTURE_REQUIREMENTS, CONTRACT_MATRIX, TESTING, CONTRIBUTING.

## Safety and completion boundary

Realtime owns metadata-only PostgreSQL state. Every chat-origin local enqueue
requires a committed admission tied to a never-reused process incarnation.
PENDING survives backend loss, timeout, cancellation and process replacement.
Finite synchronous scopes reject captured late enqueues; settlement follows
every possible sink. Already queued frames are outside this boundary.

Protected native Chat enrollment supplies structure, never recipient ACL.
Unknown/unavailable state denies every recipient, including sender fallback.
Imported restrictive gates dominate delayed LIVE hydration. Exact immutable
Chat pages close gates before import receipts; strong FROZEN requires sealed
coverage and no unresolved admission, transactionally with saved receipt.
PURGED is permanent. Restore requires the allowed newer generation and prior
completed freeze. Public evidence and the original replay horizon use one fresh
database clock instant. Full bytes expire after30days; compact hashes, pending
tickets and restrictive gates survive. Expired replay cannot renew its window.

Space imports every page into participant11 and waits all11 FROZEN and
PURGE_DECIDED receipts before scheduling/destructive work. Migration34 widens
the participant/compact ledgers and terminal predicate without changing native
membership33 or RoomClose hooks. IDs1-10 retain their meaning.

## Completed source and proof

- Metadata migration/store, synchronous executor, exact manifest import/seal,
  fence/purge receipts and bounded compaction are implemented.
- Real connReg/hub, profile/Redis/message/chat/ephemeral paths carry typed origin
  and admission. Unknown Chat type no longer grants sender delivery.
- Separate Chat native resolver and Realtime lifecycle service use mTLS, exact
  principal/RPC/operation/body binding, current+next HTTPS JWKS and replay Redis.
- Native Chat LIVE0 nullable operation mapping is verified and corrected.
- Runtime requires its migrated DB and canonical exactly2 PKCS#8 signer keys.
  Public keys must match independent publication. Chat health observes its
  native schema/listener/verifier/replay; Realtime health additionally requires
  the protected Chat TLS connection and every actually used metadata column.
  Readiness performs no synthetic domain mutation. These are advisory checks,
  not proof a particular chat is LIVE or public P3 is globally active.
- Independent local public JWKS bootstrap has no app health cycle. CA/SAN and
  both documents are checked. Static generation supports a prepublished next
  key switch; mismatch/partial old volumes fail closed. Phase0 consumes its
  actual distinct fixture Space issuer. Staging/prod assets are explicit
  external secret/public-document prerequisites, not deployed credentials.
- Compose/DEV/staging migration catalogs and deployment templates are wired.
  Canonical store/contract/operation/testing docs describe the boundaries.

Owned Linux Go1.26/PostgreSQL16 `-race` runtime evidence:
10 Realtime cases (actual hub drain/late scope, protected RPC composition,
backend loss, exact replay, restart/permanent purge, unknown scope, multipage
seal, compaction, schema readiness) plus1 native Chat full-catalog case and1 Space34 full-catalog
hard-delete case passed with zero required skips. Additional protected transport
proof covers issuer/audience/body/replay/mTLS denials, JWKS outage before/after
hard expiry and recovery, and replay Redis loss. Receipt database timestamp
equality is included in the final PG rerun. The initial unrelated-idle-backend
kill fixture was superseded by MaxConns1/acquire-after-Begin and independent
killer, targeting the backend that actually committed admission.

Scoped short checks: Realtime456 passed; Space197 passed/152 explicit legacy PG
skips; principal63 passed; Chat package/root checks passed, with legacy skips
reported separately. Realtime/Chat/Space vet passed.
Buf lint, full local Go+Dart generation and Go parity passed; buf breaking
against a local archive of the exact base passed. Initial Git URL clone timed
out; the source archive avoids that clone. Migration runner contracts passed
with Node24, DEV migration unit tests10 passed, image-catalog/YAML/Compose
checks passed. Initial runners lacked Node/jq; corrected owned Linux runners
passed, so their environmental failures are not product failures.

## Remaining delivery work

- Collect the final active checks, maintain exact source/evidence bindings and
  preserve a normal immutable commit plus portable bundle/restore evidence.
- Update the AST graph with the owned `graphify update . --no-cluster` process.
- Obtain ordinary independent security/concurrency review of the exact head;
  Astra bootstrap disposition is not sourceCLEAR. Address actual findings.
- Sole SSH publisher/CI owner121 publishes and observes hosted checks. Source
  owner172 retains CI/task/delivery custody until explicit accepted real-ant
  transfer. No partial-source integration or fake acceptance.

## Resource and activation boundaries

WT41 lease f8e38cf028e75e95fa9ac18395adee7d; Docker network rudeus172-p3,
PG rudeus172-p3-pg, public JWKS rudeus172-p3-jwks, build volume
rudeus172-p3-build and uniquely named finite runners. No host ports, Windows
test binaries/listeners, firewall changes or shared stand operations. Current
TT resource protocol is mandatory; DEV/STAGING were not claimed or touched.
Account every owned command before returning a lease or removing owned fixtures.

Cooperative incarnation close is supported. Dead-incarnation recovery without
qualified exact termination proof is explicitly unsupported and remains pending;
no TTL reaper, unchecked operator boolean or identity reuse is introduced.
Public P3 activation still requires migrated/restorable metadata, complete old
and new writer coverage/drainage, all11 release coverage and qualified recovery
and rollback. A healthy binary, environment flag or this source/proof package
does not establish these gates. No global catch-up, payload history or event
generation binding after restored LIVE is claimed by this terminal scope.
