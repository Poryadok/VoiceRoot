# ExecPlan: BE-023 File account-erasure participant

## Purpose

Release exactly the erased account's profile-media references after Auth's
irreversible P30D boundary, preserving independent references to shared bytes.
This is a continuation plan with missing upstream source inputs, not delivered
participant behavior or an activation/acceptance claim.

## Context

Sources: `docs/features/auth-and-contacts.md` account deletion/GDPR;
`docs/microservices/file-service.md` File reference/GC authority;
`docs/DATA_MODEL.md`, `docs/DATA_STORES.md`,
`docs/ARCHITECTURE_REQUIREMENTS.md`, `docs/CONTRACT_MATRIX.md`;
`docs/TESTING.md`, `docs/CONTRIBUTING.md`; PLAN A4. The accepted implementation
decision is TT BE-020 c4-c8, particularly c7 paragraphs 18-23. Consumer is
BE-020/User, with BE-019/Auth supplying terminal authority.

Examined base: `f40a7ddf5ad1f9d22441635773f7abdf2e7baaf2`.
`protos/voice/file/v1/file.proto` and
`src/backend/file/internal/grpcsvc/reference_lifecycle.go` admit only
Space/Chat/Messaging reference producers. The Auth wire lacks the explicit
ACCOUNT_ERASURE purpose. `src/backend/user/internal/grpcsvc/user_avatar.go`
lines 57-70 directly presign an R2 key and return public URL/object key without
a File reference. CodeGraph inspected these paths; Graphify broad query completed
but added no account-erasure authority.

## Scope

File-owned protected participant, deterministic USER_PROFILE plans, exact durable
release/capability revocation, minimized retry state and existing blob GC.
Excluded: Auth/User/Role/Mod source, P3 Space lifecycle changes, deployment,
staging, acceptance and any global account-erasure completion claim.
No substitute terminal boolean, age-based raw event purge or URL sweep.

## Milestones

- [x] Read owning contracts and inspect actual authority/enrollment seams.
- [x] Record the precise upstream gaps in File documentation and backend TODO.
- [ ] Upstream delivers explicit protected File handle/proof contract and User
  exact owner-bound media enrollment/backfill with writer closure.
- [ ] Write first focused failing Linux integration proof before source edits.
- [ ] Implement File participant and independent source review.

## Detailed Steps

1. BE-019/Auth and BE-020/User must name concrete protected methods/DTOs,
   workload ACLs and immutable provenance for handle enrollment, snapshots,
   journal/current-cycle reconciliation and terminal proof. Reuse actual Auth
   handle ownership; do not create a second lifetime authority.
2. User owner must provide exact references before publication/replacement,
   all historical/archived media inventory and late-writer/backfill closure.
   Existing direct R2 media needs owner-verified File import or a User-owned
   exact-object journal. Ambiguous ownership stays incomplete.
3. Update owning schema/ACL/data-classification contracts for the concrete wire.
   Choose a fresh migration after rereading current File migrations; none is
   reserved by this plan. Do not widen P3 manifest producer semantics.
4. First RED: actual protected authority schedules cycle A, restores just before
   deadline with delayed delivery, then File's due job cannot release from its
   stale schedule. After an actual terminal cycle B proof, only the registered
   exact references may release atomically.
5. Implement durable per-handle state, immutable deterministic plans, local due
   jobs and proof acquisition outside locks; recheck exact binding/revision,
   terminal dominance and fresh DB time within the transaction.
6. Under existing reference/blob serialization, revoke affected capabilities,
   release saved reference IDs, minimize raw owner/retention tuple state and
   persist receipt/GC intent atomically. Shed raw manifest tuples at P30D;
   retain only classified pseudonymous IDs/outcome/retry state.
7. Prove changed-body conflicts, stale/foreign-cycle refusal, repeat/restart,
   zero-plan inventory, rollback, terminal late-acquire denial, outage overdue
   behavior and surviving Story/message/other-profile shared-reference access.
   Physical object cleanup needs separate retry/absence and write-fence proof.
8. Run focused Linux integration tests plus File short tests, vet and pinned
   lint from `docs/TESTING.md`, then `graphify update .` after code changes.
   Preserve raw proof and route frozen source to a separate reviewer.

## Validation

- Documentation-only checkpoint: `git diff --check` and local Markdown link
  targets. No behavior tests or artificial RED are warranted for these edits.
- Future source proof: owned disposable Linux PostgreSQL, real protected
  Auth/User contract, actual File reference/GC paths, no skipped decisive cases.
- Exact commands for a new participant test depend on the upstream delivered
  API and test names; do not advertise a nonexistent executable test as proof.

## Progress

- [x] No production, migration or test source edited at gap checkpoint.
- [x] No runtime fixture, storage, shared environment or hosted CI acquired.
- [ ] First RED and implementation await the listed source inputs.

## Decisions

- Preserve source boundary: missing producer/owner authority cannot be replaced
  by a mocked trusted interface and described as a completed participant.
- Historical actor-label correction is append-only in TT: this actual child is
  Dimas-24; its initial Dimas-21 label collided with another worker. The same
  clean worktree/lease remains with the same real agent, not a second writer.
- File receipt is exact-release/GC-intent evidence; physical blob completion and
  whole account-erasure acceptance remain separate.

## Risks And Follow-Ups

Missing Auth proof or owner inventory prevents destructive source integration.
P30D cannot be claimed under an authority partition: fence affected scopes,
record overdue cleanup and reconcile. User raw erasure cannot be delayed merely
to preserve File's raw profile roster. Backup rollback requires independently
current terminal/canceled-cycle coverage before affected serving resumes.
