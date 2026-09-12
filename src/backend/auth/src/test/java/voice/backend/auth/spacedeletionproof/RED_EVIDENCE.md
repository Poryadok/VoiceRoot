# R23 Cycle 2 Auth deletion proof — RED plan and evidence

## Purpose

Freeze the Auth-owned tests for the distinct `space_delete` proof family before
any production implementation. The accepted packet must prove that Space can
recover one immutable deletion receipt after response loss without persisting
the opaque proof, while factor changes, expiry, changed bindings and untrusted
callers fail closed.

## Sources and base

- Exact base: `9757f7c25a8b6ed736006624a5392a5849f28614`.
- `docs/microservices/auth-service.md`, section “Space deletion proof and recovery”.
- `docs/ARCHITECTURE_REQUIREMENTS.md`, Phase-0 service principals and P3 durable
  lifecycle wire/hash rules.
- `protos/voice/auth/v1/auth.proto`, the four R23 Auth RPCs and deterministic
  request/receipt messages merged by contract PR #283.
- `docs/features/auth-and-contacts.md`, account security and erasure rules.
- `docs/DATA_MODEL.md`, `docs/DATA_STORES.md`, `docs/PLAN.md`,
  `docs/TESTING.md`, and `docs/CONTRIBUTING.md`.
- `tmp/slave-driver/run/R23-DELETION-RETIREMENT-PLAN.md`, Cycle 2 and Auth
  future write scope.
- Existing ownership-proof tests and implementation are test-pattern precedent
  only; deletion proof purpose, rows, tokens and receipts remain separate.

## Acceptance criteria frozen by this packet

1. Issue binds the verified account/profile/positive epoch, canonical Space and
   operation UUIDs, and exact UTF-8 confirmation name. Password is always
   required. Enabled 2FA accepts exactly one valid TOTP or one unused backup
   code and rejects none or both.
2. Auth returns the opaque proof once, stores only the SHA-256 proof digest and
   SHA-256 exact-name digest, and expires it at exactly five minutes; equality
   with `expires_at` is expired.
3. The family is purpose-bound to `PROOF_PURPOSE_SPACE_DELETE` and cannot share
   ownership-transfer rows or accept an ownership-transfer proof.
4. Every account/backup-code security revision change permanently revokes an
   unconsumed proof, including change-and-revert. Issue and consume serialize on
   the account row, with the existing account-before-backup-code lock order.
5. Consume is atomic and single-use. Concurrent exact consumers return one
   byte-identical durable receipt. Exact replay remains the same grant; any
   changed binding, exact name, proof or purpose is denied without mutation.
   The purpose-separation proof uses another syntactically valid 43-character
   opaque proof backed by an actual ownership-transfer row; malformed appended
   text is not used as evidence.
6. Lookup validates account/profile/epoch/Space/operation plus both 32-byte
   digests, creates no grant, does not observe an uncommitted consume (including
   an ambient transaction which later rolls back), and can recover the immutable
   receipt after response loss and pre-ack account erasure. Erasure removes raw
   identity/binding bytes and replaces them with a versioned, purpose-separated
   HMAC lookup index; an unavailable retained key fails closed.
7. Acknowledge accepts only the exact receipt ID/Space/operation/receipt digest,
   is idempotent, and establishes the documented retention floor. A consumed,
   unacknowledged receipt has no time-based deletion. An acknowledged receipt is
   deleted only at `max(consumed_at + P30D, acknowledged_at + PT24H)`; changed
   acknowledgement evidence cannot replace the accepted evidence.
   Lookup changes account, profile, epoch, Space, operation, name digest and
   proof digest one field at a time; acknowledgement changes receipt ID, Space,
   operation and digest one field at a time. The initial unconsumed lookup and
   every denied consume/name/proof variant compare the complete row set before
   and after. Acknowledgement denials do the same both before and after a valid
   acknowledgement.
8. Issue is an ordinary/public RPC and resolves the user from the ordinary
   access credential. Consume, lookup and acknowledge are absent from that
   listener and require verified `service:space` on the protected listener;
   issue is absent from the protected listener. Each protected method enforces
   exact RPC, deterministic request hash, audience, request ID, credential
   lifetime and replay. Every authority request rejects unknown fields before
   reaching the domain.
9. Malformed requests map to `INVALID_ARGUMENT`; absent credentials to
   `UNAUTHENTICATED`; wrong caller and missing/unconsumed/expired/revoked/
   mismatched proof states to the same coarse `PERMISSION_DENIED`; storage or
   key/config failures to redacted `UNAVAILABLE`.
10. Flyway `V13` and golang-migrate `000014` are equivalent, additive and
    hash-only. Executable DOWN obtains `ACCESS EXCLUSIVE`, waits for an in-flight
    consume commit and then refuses with SQLSTATE `55000` while receipt evidence
    exists, preserving schema and data. Issue/consume serialize against security
    writers, and concurrent issue cannot spend one backup code twice.
11. The Auth receipt-index key family rotates at `P90D`; destruction waits for
    both zero dependent retained rows and expiry of the `P30D` maximum
    restorable-backup window. The fixed HMAC vector covers the exact purpose,
    NUL separator and deterministic canonical binding bytes: for key bytes
    `00..1f`, `HMAC-SHA256(key, ASCII("voice-auth-space-delete-receipt-v1") ||
    0x00 || binding_bytes)` is
    `61f3d6dfaabe3f99502d0d94db34c40b822d33d5b4b359b6bd6103c5b502519b`.
    The frozen vector uses canonical UUIDs, positive epoch, two valid 32-byte
    digests, factors and purpose rather than the contract suite's intentionally
    minimal wire fixture.
12. PostgreSQL metadata exposes exactly the allowed deletion-proof column
    inventory. Every SHA-256/HMAC column is `bytea` with a 32-byte check, while
    no plaintext or raw/base64/encoded proof or confirmation-name column exists.

## Planned test files

- `SpaceDeletionProofMigrationContractTest`: dual-catalog presence/parity,
  separate hash-only schema, constraints and guarded DOWN text contract.
- `SpaceDeletionProofServiceTest`: factor matrix, exact UTF-8 binding, TTL,
  purpose separation, replay/change matrix, canonical binding domain hash in
  the receipt, lookup/ack and coarse domain denial.
- `SpaceDeletionReceiptErasurePolicyTest`: fixed purpose-separated HMAC vector,
  missing-key contract, `P90D` rotation and `P30D` backup/destruction boundary.
- `SpaceDeletionProofJdbcIntegrationTest`: real PostgreSQL concurrency,
  ambient rollback, uncommitted visibility, issue/consume versus security-writer
  locks, concurrent backup-code issue, response-loss/pre-ack HMAC recovery,
  exact retention boundaries, same-template backup-code races, purpose
  separation against an ownership row, exact metadata and executable guarded
  DOWN.
- `AuthSpaceDeletionProofAdapterTest`: ordinary credential path for issue,
  protected Space credentials for consume/lookup/ack, mutually exclusive
  listener surfaces, unknown-field rejection and exact request-hash enforcement.

## Red-green order and commands

1. Run migration contract alone before production files exist; expect a real
   assertion failure naming missing `V13`/`000014`.
2. Add the remaining accepted tests, then run the focused packet; expect test
   compilation to fail only because `voice.backend.auth.spacedeletionproof`
   production types and new Auth adapter/configuration hooks do not exist.
3. Stop for independent review. Freeze reviewed test hashes before any GREEN.
4. After driver handoff, add the minimal migration/package/config/adapter in
   small cycles: migration → domain → JDBC/recovery/ack → transport/security.
5. For each cycle run the focused test, then the packet; finish with
   `mvn -B test`, required Testcontainers report audit and `git diff --check`.

Focused commands from `src/backend/auth`:

```text
mvn -B -Dtest=SpaceDeletionProofMigrationContractTest test
mvn -B "-Dtest=SpaceDeletionProofMigrationContractTest,SpaceDeletionProofServiceTest,SpaceDeletionReceiptErasurePolicyTest,SpaceDeletionProofJdbcIntegrationTest,AuthSpaceDeletionProofAdapterTest" test
mvn -B test
```

The PostgreSQL class owns a task-labelled Testcontainers container and never
uses shared Compose data. No Docker/Testcontainers resource is started during
the compile RED.

## Ownership and exclusions

Writer owns only this leased Auth worktree and future isolated Testcontainers
resources. No nested agents. No generated output, Space, Role, Gateway, contract
proto, shared plan/TODO, commit, push or PR changes are allowed in this phase.

## Progress

- [x] Required instructions, canonical docs, exact contract and existing Auth
  proof patterns read.
- [x] Dedicated Treehouse lease acquired and exact base verified clean.
- [x] Assertion RED captured for absent migration: from `src/backend/auth`,
  `mvn -B -Dtest=SpaceDeletionProofMigrationContractTest test` exited `1` with
  `tests=1, failures=1, errors=0, skipped=0`; the expected assertion names
  missing `V13__space_deletion_proofs.sql` at test line 22.
- [x] Initial packet was independently reviewed and rejected for seven concrete
  coverage gaps; no production code was started.
- [x] Test/manifest-only correction packet now covers pre-ack HMAC erasure and
  key lifecycle, exact receipt retention, executable transaction/DOWN behavior,
  security-writer and backup-code races, canonical receipt binding hash,
  unknown-field rejection, and the corrected ordinary/protected listener seam.
- [x] Second test-only review correction replaces the minimal HMAC fixture with
  a valid canonical binding/vector, gives every concurrent service instance one
  DataSource/template for both proof and backup-code access, uses a valid
  alternate proof plus ownership row, expands exact negative matrices with
  before/after row equality, and checks the live PostgreSQL catalog inventory,
  binary types and digest-length constraints.
- [x] Final oracle correction adds the lookup operation-ID mismatch and asserts
  complete row-set equality for the initial unconsumed lookup denial and every
  changed-binding, confirmation-name and proof consume denial.
- [x] GREEN fixture correction removes one surplus encoded zero from the fixed
  UUID wire vector and isolates each PostgreSQL test with an empty deletion-proof
  table; the independently recomputed canonical HMAC is `61f3d6df...2519b`.
  It also binds the network credential and header to the same request ID and
  tests PostgreSQL retention one microsecond before equality, matching the
  store's durable timestamp precision.
- [x] Corrected focused packet re-run from `src/backend/auth`: `mvn -B
  "-Dtest=SpaceDeletionProofMigrationContractTest,SpaceDeletionProofServiceTest,
  SpaceDeletionReceiptErasurePolicyTest,SpaceDeletionProofJdbcIntegrationTest,
  AuthSpaceDeletionProofAdapterTest" test` exited `1` in `testCompile`. Leading
  errors are the expected missing deletion-proof package/API, ordinary actor
  resolver and protected deletion RPC constants. No test ran and no
  Docker/Testcontainers resource started.
- [x] Independent review PASS for the corrected frozen RED packet; the driver
  explicitly authorized bounded GREEN in this retained lease.
- [x] Production GREEN adds Flyway `V13`/golang-migrate `000014`, the
  `spacedeletionproof` HMAC/keyring/service/JDBC store, ordinary actor
  resolution, protected listener adapters and the exact Auth service document.
- [x] Source review exposed an ordinary-listener actor-boundary regression.
  `mvn -B -Dtest=AuthSpaceDeletionProofAdapterTest test` exited `1` with
  `tests=10`, `failures=1`, `errors=0`, `skipped=0`: after a prior login had
  populated another actor's remembered access token, an issue call without the
  current request's `Authorization` metadata returned `UNAVAILABLE` instead of
  the required `UNAUTHENTICATED`.
- [x] The boundary GREEN derives the issue actor only from
  `AuthorizationServerInterceptor.AUTHORIZATION` for the current request and
  never reaches the process-global remembered-token fallback. The same focused
  adapter command exited `0` with `tests=10`, `failures=0`, `errors=0`,
  `skipped=0`, preserving the valid current-credential path.
- [x] Focused GREEN from `src/backend/auth`: `mvn -B
  "-Dtest=SpaceDeletionProofMigrationContractTest,SpaceDeletionProofServiceTest,
  SpaceDeletionReceiptErasurePolicyTest,SpaceDeletionProofJdbcIntegrationTest,
  AuthSpaceDeletionProofAdapterTest" test` exited `0` with `tests=46`,
  `failures=0`, `errors=0`, `skipped=0`.
- [x] Relevant principal/JWT/Auth regression command exited `0` with
  `tests=69`, `failures=0`, `errors=0`, `skipped=0`.
- [x] Full Auth verification from `src/backend/auth`: `mvn -B test` exited
  `0` with `tests=511`, `failures=0`, `errors=0`, `skipped=0` after the
  actor-boundary fix.
  `scripts/ci/check-auth-testcontainers-reports.sh
  src/backend/auth/target/surefire-reports` also exited `0`.

## Risks and containment

- The new package API in tests is reviewable intent, not accepted production
  architecture until independent review. Tests prefer wire-visible behavior and
  durable SQL evidence; helper seams are kept as small as existing Auth proof
  patterns permit.
- Account erasure HMAC/key rotation is the accepted U5-A policy with `K=P90D`
  and `B=P30D`. Its exact runtime/key-store seam must fail closed; no test may
  silently replace it with a raw UUID fallback or an Analytics/general-purpose
  key.
