# ExecPlan: usercapture CI lint repair

## Purpose And Context

Repair CI444's ST1005 and SA1019 findings without weakening protected transport. Exact base444f0f7e6d8fefb1aa0f1a360e63ec087a748002/treedd39fd21785889eb93eeca9b8d5e2eba58fea8e7; saved job120152805980 log70843B SHA25685180f7b0258bafa1ec0b3e26e4582a808227b1a9245a5acae30b5d62d58b039. Prior client source eedac was independently reviewed; this follow-up is a new linked task PR875-USERCAPTURE-LINT owned by actualIsaac17, Sol6.1/medium, consumer integrator2. Fresh WT35 lease2de2bfc05af0077c347b80b6d1128308.

## Scope And Selected Contract

Only internal/usercapture/client.go, transport.go, transport_test.go and this plan. No other Native findings, main/provider/SDK/config changes or lint suppression. Error text becomes lower-case while preserving the sentinel/errors.Is/category.

Integrator2 explicitly selected the following narrow input policy BEFORE edits: Roots must be explicitly supplied; nil or empty explicit pools are rejected; a caller-supplied nonempty SystemCertPool is admissible. The constructor never creates or falls back to SystemCertPool. Use supported CertPool.Equal(empty) instead of deprecated Subjects. This is a deliberate contract clarification/change: on platform-backed pools, Subjects omits system roots while Equal includes the systemPool flag. It is not byte-for-byte equivalent for every prior input. Real TLS CA, server identity, client keypair/certificate and receiver principal checks remain unchanged.

Exact source has no production capture config builder/caller, nor any in-scope PEM parser. Tests use explicit pools. Separate Auth/Role config allows SystemCertPool, while Bot/lifecycle config uses explicit PEM; those do not determine capture policy. No invalid-PEM parser/test seam is invented. Canonical capture contract continues to require mTLS, exact Space credentials and fail-closed binding; production activation is separate.

## Milestones And Detailed Steps

- [x] Inspect exact CI/source and surface the SystemCertPool policy difference to integrator2.
- [x] Receive integrator2's explicit semantic scope/base ACK; create/claim linked task and fresh worktree.
- [x] Add empty explicit pool and caller-provided system pool characterization tests; retain existing real mTLS positive/wrong CA/server/missing client-certificate negatives.
- [x] Capture scoped staticcheck RED for ST1005/SA1019 using CI's honnef.co/go/tools v0.6.1; then minimal two-line production fix.
- [x] Run focused staticcheck GREEN and affected package/race/vet in private Linux, bind exact source/commands/terminal results.
- [ ] Freeze normal commit/push and independent review, then explicit integrator2 offer/ACK/supported handoff/readback.

## Validation

The saved exact CI findings supply observed lint RED; repeat only scoped ST1005/SA1019 when locally feasible. Package tests prove nil/empty explicit roots rejection, explicit system pool constructor acceptance, unchanged valid mutual TLS, wrong CA/server identity rejection and missing client certificate denial. Linux SystemCertPool may expose roots as ordinary certificates, so its acceptance is characterization rather than a claimed reproduction of the old Windows/macOS platform-root rejection. No synthetic systemPool flag, unsafe reflection or Windows test executable is used.

Use a completed/hash-bound immutable archive extracted into a fresh empty private Linux directory; <=2CPU/3GiB, network none, no host ports/socket/privileged/shared services. Existing toolchain/cache may be reused but no old candidate source. No full Native/CI rerun is claimed. New provider or production trust acquisition is not introduced.

Actual evidence: baseline package with new characterization tests passes11 top-level/zero skips in2.054s. Initial offline analyzer build lacks tool dependencies and is explicitly not lint RED. The separately built pinned staticcheck2025.1.1(v0.6.1), matching CI golangci2.6.1's analyzer dependency, reproduces EXACT ST1005/client17 and SA1019/transport34 on baseline archive, exit1. Green427-entry archive SHA256717242d6c5c68e4b991a81ddde9cc51ff9a602c64f570da487b28da9bc1c7a67 then passes the same two static checks, followed by `go test -race -count=1 -v ./internal/usercapture && go vet ./internal/usercapture`,11 PASS/zero skips/package4.010s/chained exit0. All test runs and checks are network none. Only the isolated tool-build container uses bridge networking to download checksum-verified Go tool dependencies into its private cache; no app source/runtime is in that container. No host module cache is modified.

## Decisions, Risks And Follow-Ups

Real cryptographic verification is retained; Equal is a supported emptiness check, not a substitute for TLS verification. Constructor acceptance of supplied platform roots was separately authorized, not silently inferred. Independent review is required for that narrow policy and proof. Production User provider/registration/lineage, Space main injection, B15 and hosted CI remain separate gates. Mandatory own CodeGraph first returned Transport closed; immutable Git/docs/toolchain sources supply exact evidence.

Own Graphify update attempt session98925 did not emit completion and was interrupted after its bounded observation; exit1, no graph freshness PASS. No unrelated source or caller was changed.
