# ExecPlan: shared outbound gRPC breaker with active Gateway adoption

## Purpose

Prevent an unavailable upstream RPC from repeatedly consuming Gateway work while
preserving independent healthy RPCs, existing authorization and ordinary error
responses. This first coherent slice supplies a shared Go unary interceptor and
activates it on Gateway's ordinary upstream clients. It must not claim that all
Go/Java clients or protected/streaming transports have been migrated.

## Context

- Task: `BE-040`; author `Рудеус-153`, integration consumer `Рудеус-121`.
- Immutable starting develop: `ab6aa8e224f31f2ec55e93b923a43d9b7867ec76`.
- Owning requirements: `docs/MICROSERVICES.md` resilience section requires
  circuit breakers on interservice gRPC. Its retry requirement covers NATS and
  external APIs; it does not authorize replaying every gRPC mutation.
- `docs/OPERATIONS.md` defines Tier 0/1/2 isolation and degraded behavior.
  `docs/ARCHITECTURE_REQUIREMENTS.md` and the Gateway service document preserve
  current identity, authorization, idempotency and error-disclosure boundaries.
- `src/backend/pkg/grpcclient/` currently provides DNS targets and startup wait;
  importing it does not install a breaker. Gateway `transcode.go` owns an
  ordinary dial closure used by 17 configured namespaces. Four protected Gateway TLS
  client constructors are separate. Other Go main/runtime constructors and Java
  Auth remain explicit later adoption work.
- Runtime checks execute only in an owned Linux Docker runner. No host test
  executables/listeners, shared stand operations or independent GitHub probes.

## Scope

In scope: accepted shared unary breaker semantics, concurrency-safe state,
observable invoker admission/rejection, active ordinary Gateway dial wiring,
negative/race/real-gRPC tests and accurate adoption documentation.

Out of scope: generic application retries, cached/stale authority fallbacks,
new product degradation values, TLS/principal changes, streaming, Java and
unrelated service adoption, NATS DLQ, rollout or final acceptance.

The exact classification, isolation key, defaults and half-open transitions
must first be resolved by the bounded Astra decision recorded in task comment
`c1`. No dependent production behavior is written before that decision.

## Milestones

- [x] Claim task, inspect current shared helpers/callers and acquire exclusive
  Treehouse46 at current local develop.
- [x] Record accepted decision and owning canon before implementation.
- [x] Demonstrate meaningful RED admission/recovery tests, then implement and
  demonstrate GREEN for the shared helper.
- [x] Activate the helper in Gateway's ordinary dial closure and prove real
  upstream failure isolation and no replay through that production constructor.
- [ ] Complete affected checks and independent concurrency/security review.
- [ ] Preserve exact source, proof and portable restore; offer the coherent
  source to integrator121 and retain custody until explicit accepted transfer.

## Detailed Steps

1. Resolve `BE-040 c1` with Astra165 under the user's delegated ordinary decision
   mandate. Record source-backed semantics, defaults and unresolved adoption
   coverage in the task and owning docs.
2. Add focused shared-helper tests using a controlled invoker and clock. Verify
   actual invoker call counts and returned status, independent targets/methods,
   cancellation/domain-error nontripping, cooldown, one half-open probe,
   concurrent admission and generation-safe stale completion. Do not assert
   undocumented numeric defaults until accepted.
3. Run RED in Linux Docker. A missing-symbol compile failure is not behavioral
   RED: introduce only an inert API seam if necessary, then observe the old
   pass-through behavior violate the accepted admission contract.
4. Implement the smallest mutex-protected state machine. Never hold its lock
   across network work. Do not add retries or alter forwarded context/options.
5. Add Gateway integration coverage through `grpcClientsFromEnv`, a real local
   gRPC server inside Linux and typed User calls. Prove rejected calls do not
   reach the server, a healthy distinct method remains admitted and a mutation
   invocation is never replayed. Preserve existing HTTP status mapping.
6. Run focused tests and race checks, shared package suite, Gateway short suite,
   affected vet/lint and exact diff check. Reuse existing valid evidence; no
   repository-wide or unrelated PostgreSQL run is required by this slice.
7. Obtain one independent review of implementation, tests and canon together.
   Resolve findings before delivery. Commit normally, preserve bundle/SHA and
   restore evidence, then deliver exact head/base/dependencies/proof to121.

## Validation

- Shared `grpcclient` focused and race tests discriminate OPEN rejection,
  isolated RPCs, bounded half-open probes and stale generation completions.
- Gateway constructor integration tests execute real gRPC inside Linux and
  discriminate backend call counts from client-side status alone.
- `pkg` affected suite, Gateway short suite, affected vet/lint and
  `git diff --check` verify the coherent source.
- Independent source review is required for concurrency and authorization
  boundary risk. Hosted exact-head CI and assembled develop checks remain
  integrator gates; local checks are not deployment or acceptance.

## Progress

- Astra165 published accepted autonomous engineering decision
  `BE040-GATEWAY-UNARY-v1` in `BE-040 c2`; no routine user gate remains.
- Baseline existing `grpcclient` tests passed in an isolated Linux Go1.26
  runner. The first admission test and inert public API seam are prepared for
  meaningful RED; a missing-symbol compilation failure will not count as RED.
- Meaningful RED observed invoker3 instead of2 at OPEN; first shared GREEN
  passed34 named checks without skips. Extended concurrency matrix passed44
  checks under Linux race detector before the final copy-assertion refinement.
- Actual Gateway RED reached backend6 instead of5 through HTTP503 route, and
  a resolver-supplied retry policy invoked a received mutation4 times instead
  of1. The ordinary dial now installs the shared breaker and disables configured
  retry; the same immutable negative tests passed GREEN (two tests, no skips).
- Shared helper and Gateway golangci-lint report zero issues. Final shared race
  passed44 named checks without skips; Gateway short/race passed802 with129
  explicit opt-in/live skips. Both affected vet checks passed. A broader pkg
  attempt first lacked repository fixture paths; the faithful-context repeat
  passed other packages but two unrelated issuer tests refused root execution.
  Those runner failures are preserved and are not reported as product PASS.
- Independent review of417b4f3 found P2: sampling time before the mutex could
  apply an old observation after a newer state transition. Both admission and
  completion now sample under the state mutex. A deterministic clock seam
  models the delayed pre-lock observation and checks expired failure windows,
  full cooldown from transition time, and eligible probe admission. All three
  scenarios failed before the fix; the complete helper race suite then passed.

## Decisions

- Ordinary same-author TDD applies; one independent final expert review follows
  the high concurrency/cross-service risk. No strict workflow was requested.
- A helper-only disabled foundation is insufficient: the first delivery must
  include the named active Gateway consumer and observable integration proof.
- Existing dedicated transport/principal policy is preserved; this task does
  not infer authorization to retry a signed request or return cached authority.
- Accepted defaults: threshold5 within30s first-failure window; OPEN10s;
  HALF_OPEN one caller; cap1024 methods; CLOSED idle eviction10min. Config is
  immutable with positive validated caps100/5min/1min/4096/1h respectively.
- Only live-context Unavailable/DeadlineExceeded count as failures. Nil live
  success resets CLOSED count/window and recovers HALF_OPEN. All caller context
  failures and other statuses/errors are neutral. Neutral HALF_OPEN reopens.
- Per-connection exact-method state includes entry identity, generation and
  outstanding count; stale completion cannot change a newer generation. Panic
  unwinding releases accounting/probe as neutral and propagates the panic.
- Health Check/List bypass business state. New-method capacity exhaustion fails
  locally closed. Rejected calls cannot extend OPEN or retention.
- The interceptor invokes at most once. `WithDisableRetry` rejects configured
  application retry but leaves grpc-go transparent retries; it cannot promise
  exactly one wire attempt. Pinned grpc-go is1.70; no upgrade is included.

## Risks And Follow-Ups

- Classification and scope are not implicit from RPC names. Astra must settle
  the minimum contract before tests prescribe new production behavior.
- In-flight completions must not change a newer breaker generation; half-open
  admission must be race-safe and bounded.
- Circuit state cannot mix unrelated dependencies or RPC methods.
- The broad all-service/Java/streaming requirement stays open after this first
  coherent Gateway slice; adoption cannot be inferred from importing a helper.
- Current GitHub transport and final graph refresh may remain delivery gaps;
  preserve actual terminal resources and exact continuation without polling.
