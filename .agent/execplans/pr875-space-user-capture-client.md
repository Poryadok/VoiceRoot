# ExecPlan: purpose-bound Space User capture client

## Purpose

Provide Native's existing nonvariadic AccountCaptures interface with a typed request-bound mTLS User client component. Native/integrator2 is the activation consumer. This change does not install a production provider or activate B15.

## Context

Task PR875-SPACE-USER-CAPTURE-CLIENT is genuinely owned by Isaac-17, source author Sol6.1/medium. Native7 and integrator2 approved four new internal/usercapture files plus this plan. Canonical User API305679 and Native799 are now present in normal merged source. Initial accepted base7b4af43947a2061b67a86cef3c0bfe2dd8d954c9 was advanced by ordinary fast-forward to integrator2's refreshed exact base9a025c0f57fe96d0095ff101cc03efe7f62dfc09. The four intervening files are unrelated F13 tests/plan and Auth POM; the package dependency closure is unchanged. No source overlay or synthetic compiler cohort is used.

Requirements come from docs/microservices/user-service.md canonical identity association and Space capture, accepted SPACE-ACCOUNT-BAN-COVERAGE-v1 and producer/consumer ABI, shared pkg/spaceaccountcapture validators, User's protected capture guard, and Isaac11 intent-recheck invariant4. The prior immutable matrix and proposal are retained under tmp/swarm/isaac-17-user-native-abi. Repository TESTING, CONTRIBUTING and .agent/PLANS.md govern verification and delivery.

## Scope

Only client.go, client_test.go, transport.go and transport_test.go under src/backend/space/internal/usercapture, plus this plan. No store/SQL/main, User, SDK/proto/module/config/deployment changes. Actual source task claim/readback and fresh WT31 lease e329079fab9af27778aaa0a26aa41219 preceded source writes. No other owner's worktree is used.

## Milestones And Progress

- [x] Exact SDK/validator base and nonoverlapping allowlist accepted.
- [x] Genuine task claim, fresh worktree and source binding recorded.
- [x] Behavioral signing/body/outcomes test RED against compiling unavailable adapter, then GREEN.
- [x] Fail-closed input/result/deadline cases added; existing implementation satisfies them.
- [x] Typed mTLS constructor behavioral RED, then GREEN with unchanged acceptance assertions.
- [x] Final package race and vet pass on exact9a plus authored files.
- [ ] Independent source/component-proof review.
- [ ] Normal source delivery and explicit supported task handoff to integrator2.

## Detailed Steps And Decisions

The client validates/clones the full request, preserves immutable operation/Space/purpose/cohort/expectations, signs the deterministic full-body hash for the canonical generated User RPC, and creates fresh transport request ID/JTI. Outgoing inherited authorization is replaced with a pure service credential. The call uses the earlier caller deadline or two seconds, performs one application invocation without fallback, and rejects cancelled/error/late/mutated/mismatched results. Shared validation retains ASSOCIATED, TERMINAL_PROFILE and INCOMPLETE distinctly; Native still decides which can qualify an operation. Returned messages are cloned.

The constructor consumes explicit supplied endpoint, server identity, trusted roots and client certificate/private key, checks their basic consistency, snapshots certificate bytes/trust, and constructs only an mTLS generated client. It exposes no insecure dial options. Connection shutdown belongs to the caller. The unexported test dialer substitutes only the byte transport; TLS handshake, generated RPC and principal verification still execute. Deployment key acquisition, environment names and grants are deliberately not invented.

## Validation

Evidence is retained under tmp/swarm/isaac-17-usercapture with immutable source archives, SHA256/entry manifests, raw logs and terminal container state/commands.

- red1: compiling unavailable adapter fails TestClientBindsPureServiceCredentialAndPreservesOutcomes in0.044s, exit1. Identical test passes green1 in0.062s, exit0, validating real RSA signer/verifier, exact claims/hash, fresh IDs, request copies, timeout and all three outcomes.
- red2 was an unused-import compile error and is explicitly not behavioral RED. Corrected red2b compiles; constructor is still unavailable and protected transport tests fail in0.876s, exit1. green2 passes all seven top-level tests in1.119s, exit0.
- final: exact normal9a base, four authored files, 427-entry source archive SHA256a5365267d8092972314e2572c9198e7c2c1db13eea0a0b2fa5b5803e65c87e88. `go test -race -count=1 -v ./internal/usercapture && go vet ./internal/usercapture` completes exit0, nine top-level tests, zero skips, package3.136s. New final negatives use fresh credentials so replay cannot mask an audience/method/body failure; missing client certificate also fails actual TLS.

Each Linux execution uses a new empty source directory, network none, <=2CPU/3GiB, no host ports, privileged mode or Docker socket. Only immutable archive and read-only host module-download cache are mounted; existing image supplies toolchain/cache, never candidate source. No Windows test binary or shared environment is started.

Mandatory own CodeGraph returned Transport closed once; fresh helper22 succeeded4.2s without requested root anchors. Immutable source remains authority. Own Graphify update extracted254/254 files then did not complete; own session86935 was interrupted and exited1. Graph freshness is not claimed.

## Risks And Follow-Ups

JWT/cross-service security requires the separate independent review. The in-memory TLS server is explicitly a contract receiver using real principal verification/replay callback and shared wire validation, not actual User socialprincipal registration or a database owner provider. Its structural result fixtures do not assert User receipt/lineage authority. Real User main/listener registration, retained keyset/source lineage, complete writers, Auth ParentAuthority/birth, actual Space AccountCaptures main injection, coupled runtime, hosted CI and full Native/B15 acceptance remain separate owned prerequisites. No production readiness follows from this component proof.
