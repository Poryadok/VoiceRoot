# BE-041: verified identity for the audit read vertical

## Purpose

The existing audit-log REST route must carry the verified delegated actor through
Space's complete read/disclosure/permission/cursor chain without manufacturing
`x-voice-*` identity metadata. This is the first activated BE-041 consumer, not
completion of the remaining thirteen legacy `authctx` packages.

## Context

The accepted decision is BE041-VERIFIED-IDENTITY-v1 (TT BE-041 c4). Sources are
ARCHITECTURE_REQUIREMENTS Phase-0, DATA_MODEL account/profile/session epoch,
api-gateway audit REST and space-service audit ACL contracts. Base is
`e3dc9ba836100707a9c4313d83bea1872f1f9016`. Gateway already signs the exact
request and uses a dedicated mTLS client. Space currently verifies it and then
projects raw identity for its audit handler and public disclosure wrapper.

## Scope

Add a bound reader of already verified delegated identity in `pkg/principal`;
remove projection only for GetAuditLog; pass typed profile through audit handler,
public disclosure wrapper and extracted permission/owner cores. Keep all other
method migration behavior and all existing ACL, membership, freeze, filters and
viewer-bound cursor rules. No schema/proto/producer changes. Space-to-Role's
ordinary decision transport is an explicit BE-181 residual, not protected S2S
proof. Other thirteen authctx adoption chains remain BE-041 work.

## Milestones and detailed steps

1. [ ] Demonstrate existing audit projection and raw-handler admission violate
   the accepted contract using focused Linux RED tests.
2. [ ] Implement `UserIdentity`/`RequireDelegatedUser` with canonical non-nil
   account/profile UUIDs, positive epoch, exact request/audience/RPC binding,
   gateway delegated kind and no mixed bot capability. Read verified context
   only; preserve explicit account type, including empty compatibility value.
3. [ ] Migrate interceptor, ordinary handler and public wrapper together. Extract
   profile-explicit ACL cores while preserving legacy wrapper behavior.
4. [ ] Migrate test-only ledger fixtures to explicit bound verified context;
   these fixtures are not network-authentication evidence.
5. [ ] Prove real REST Gateway route/issuer to Space mTLS listener/handler using
   HTTPS JWKS, Redis floor/replay and PostgreSQL audit data inside owned Linux
   isolation; exercise positive membership/Role/cursor and denial/disclosure.
6. [ ] Run affected Linux package/component tests, vet/lint; obtain independent
   ACL review, freeze portable source/evidence and offer delivery to ant121.

## Validation

Shared tests reject missing/service/bot/mismatched/invalid identity and bindings,
including nil/noncanonical UUID, nonpositive epoch and mutated request. Interceptor
tests reject raw/duplicate/tampered/revoked/replayed credentials before handler and
assert zero synthetic identity on valid audit. Real isolated route proof covers
owner/member, Role deny/outage, nonmember, frozen owner/member, viewer-bound cursor,
ordinary listener, wrong TLS and floor/replay failures. Existing principal/JWKS
negative suites retain temporal/cache verification. No Windows test executables
or host listeners; no shared DEV/STAGING claim or deployment is part of this work.

## Progress

- [x] Reconciled actual task/base and accepted decision; own WT31 lease acquired.
- [x] Authors155/144 confirmed no overlapping target edits.
- [ ] RED/GREEN and complete Linux route proof.

## Decisions

- Shared helper is a reader, not another verifier or metadata parser.
- PublicLifecycleSpace.GetAuditLog is part of the complete ACL/disclosure chain
  and must migrate with SpaceGRPC.GetAuditLog.
- Existing ledger-only fixtures may inject verified context explicitly; the new
  route test must use the real network verifier and no synthetic raw identity.

## Risks and follow-ups

Independent ACL review and exact-head hosted CI remain mandatory. Other Space
lifecycle projections and thirteen legacy authctx packages remain open. No global
claim of protected Role S2S, production PKI rollout or final acceptance is made.
