# ExecPlan: A2 capture environment delivery

## Purpose

Publish the existing User custody image target, then qualify and deliver the
same application artifacts to DEV and STG. Actual HTTP Space → User → Ban,
persisted User/Space effects and refusal under lost qualification are the
acceptance evidence. Production is excluded.

## Context

Sources: `docs/DEPLOYMENT.md`, `docs/TESTING.md`, User/Space service contracts,
and TTenrec `A2-USER-CAPTURE-ENV-SECURITY`,
`A2-USER-CAPTURE-SOURCE-ADOPTION`, and
`A2-USER-CAPTURE-DEV-STG-QUALIFICATION`.

The User Dockerfile already has a `custody` target containing `/sourcecustody`
and reviewed migrations. Its default target contains only `/service`. The normal
Go publisher therefore cannot supply the independently running operator and
supervisor. Both must consume the same pinned custody image, with different
credentials and arguments. Image publication is not qualification or acceptance.

## Scope

Add custody-target PR verification and master publication to the existing
User-selected Go jobs, retaining their source/event selection. Publish a separate
`user-custody` repository and immutable digest receipt. Keep the normal User
image and release selection intact. Environment configuration follows source
review and real CI; no source proof, guard activation or signed decision is
created by image publication.

## Milestones

- [x] Confirm existing custody target and missing publisher.
- [x] Add target builds, source binding and digest receipt.
- [x] Reconcile the compiled-source contract and negative cases.
- [ ] Independent review, final AST refresh, commit/push and required CI.
- [ ] Merge CI/source prerequisites and publish exact source-bound artifacts.
- [ ] Under DEV resource custody, provision/configure and verify DEV.
- [ ] Under STAGING custody, preserve data and promote DEV-passed artifacts.
- [ ] Independent environment acceptance and tracker evidence.

## Detailed steps

1. The selected User PR job builds `target: custody`; the existing master User
   publisher builds/pushes that same target with `VOICE_SOURCE_SHA=github.sha`
   and OCI source/revision labels. Retain the actual digest, source SHA and
   GitHub run/attempt in `user-custody-image.json`. Never deploy a mutable tag.
2. Obtain a current environment claim before mutations. Reconcile actual
   namespace, updater/preservation locks, image IDs, quota, storage and node
   capacity. Do not treat a tracker card as evidence that Kubernetes is safe.
3. Provision independently identified control/source PostgreSQL with verified
   TLS and separate credentials, isolated writer privileges and separately held
   custodian/supervisor signing keys. Configure dedicated capture mTLS, Auth
   parent transport, HTTPS JWKS and replay dependencies. No startup auto-opens
   guards or manufactures source/restore/writer/Auth evidence.
4. DEV may use a new empty source under the user's explicit reset authorization.
   For STG prefer a verified logical backup/restore of User data into a dedicated
   source cluster, retaining the old source for reviewed rollback. Quiesce and
   fence all old User writers before the cut. Verify row/identity preservation
   and restore before adopting the exact pristine v22 domain. This is a new
   independently evidenced adoption, never a claim of historical creation or
   migration. Do not change shared PostgreSQL or other service data merely to
   obtain User isolation.
5. Review complete participating writers and real custody evidence, prepare one
   fresh signed opening decision, enroll the separate supervisor, and open only
   through the reviewed operator. Unknown acknowledgments, incomplete recovery,
   stale credentials or failed supervision remain CLOSED/HOLD.
6. Deploy the reviewed application images through the documented protected
   environment workflow. Verify actual authenticated HTTP Ban and resulting
   persisted Space/User state, rejection cases and recovery. Only then promote
   identical application digests to STG and repeat. Evaluate the separate GIS
   lease-boundary task if the selected delivery depends on those changes.

## Validation

The workflow must parse and the existing compiled-source contract must pass
with negative source-argument/target mutations. Required hosted CI and actual
custody image builds remain distinct from local static checks. Source runtime
and integration evidence belongs to the source PRs. Final DEV/STG verification
must bind actual commit/digests, resource custody, configuration/source identity
and persisted HTTP results in TTenrec.

## Progress and decisions

Local static validation initially failed because the existing compiled-source
test counted build arguments across the rest of the entire job. It now checks
the exact normal build step and independently checks the custody build's target,
context, source argument and selection. All three contract tests pass.

Read-only DEV observation showed only 256 MiB of memory-limit quota headroom.
Quota sizing must use fresh host capacity under the claim. No live configuration,
secret, quota, database or deployment has been changed by this work.

## Risks and follow-ups

The data-preserving STG transfer is a procedure to qualify, not completed
evidence. Backup/restore, writer inventory, custody, signatures and final HTTP
acceptance cannot be replaced by nonzero digests or healthy processes. The
source-adoption and supervised-opening prerequisites must be integrated before
any environment opening.
