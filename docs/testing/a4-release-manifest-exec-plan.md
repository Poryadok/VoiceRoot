# ExecPlan: exact-source alpha capability evidence

## Purpose

Make the A4 release gate reject a capability claim unless it names an exact
source revision, explicitly constrains every required alpha capability, excludes
Federation from all release inventories, and supplies an actual negative result
for every disabled capability on UI, REST and WS.

## Context

PLAN A4 capability/admission DoD and G1 define this boundary. DEPLOYMENT defines
immutable source/image provenance; TESTING requires discriminating negative
checks. No versioned capability manifest currently exists. The existing A4
recovery harness accepts evidence files but does not validate capability claims.

## Scope

Implement a dependency-free offline manifest compiler and evidence verifier,
an explicit alpha policy and generated negative-suite contract. The compiler
does not toggle runtime features, execute probes, deploy, appoint a tester or
claim that the current application satisfies alpha. Consumers are the A4/G1
operator qualification gate and its UI/REST/WS scenario runners. A missing runner
result is a failed gate, never an omitted or skipped success.

## Milestones

- [x] Test source/config/inventory/negative-evidence rejection and valid closure.
- [x] Implement deterministic suite generation and exact-source evidence checks.
- [x] Document operator contract and remaining runtime adapter requirement.
- [ ] Run isolated Linux tests, independent review and sole-121 delivery.

## Detailed Steps

1. Use ordinary TDD: add pure manifest tests and observe meaningful RED.
2. Compile an operator-supplied exact-SHA manifest against the committed alpha
   policy. Reject unknown/missing capabilities and ambiguous JSON duplicate keys.
3. Generate one required case per disabled capability and surface; bind case ID,
   source SHA and canonical manifest digest. Require complete matching PASS
   observations, no duplicates/extras/skips and a nonempty portable evidence URI.
4. Reject development configuration and Federation in images, rollout, readiness
   and alert inventories. No endpoint or UI selector is invented by this tool;
   runners bind their concrete probe to the stable capability/surface contract.
5. Verify in an owned no-network Linux container, inspect diff and request review.

## Validation

`python3 -m unittest discover -s scripts/release -p '*_test.py'` in an owned
Linux container proves compiler/evidence-gate behavior. No UI/REST/WS runtime
claim follows from these unit tests. Full negative qualification requires actual
runner observations and immutable build/deployed-source evidence.

## Progress

- [x] Atomic TT PL-A4-02 claim; actual develop 8e4a462867df1b5efcada4bd72d922fa333337c5.
- [x] Owned WT43 lease ce794ee487328dea2c013dce6adf8f77.
- [x] Read PLAN A4/G1, TESTING, CONTRIBUTING, DEPLOYMENT and OPERATIONS.
- [x] Linux RED: 8 tests, 21 assertion failures and one downstream empty-suite error.
- [x] Linux initial GREEN: 8 tests; expanded CLI/schema checks follow.
- [x] Expanded Linux GREEN: 11 tests, zero skips, including expected-source CLI
  rejection and no sensitive-input diagnostic disclosure; diff check passed.

## Decisions

- Keep manifest input explicit rather than deriving capability readiness from
  task status or an image's presence. PLAN distinguishes implementation and
  release activation.
- Require UI/REST/WS observations even when a transport lacks the feature; its
  runner must prove unreachability/absence instead of claiming not applicable.
- Canonical digest excludes evidence so the same planned suite survives retries.

## Risks And Follow-Ups

Runtime adapters and disabled guards remain required and cannot be inferred from
an offline manifest. Source/build provenance is supplied by the protected release
pipeline; this offline format does not cryptographically attest external reports.
No live activation, stand use or full PL-A4-02 acceptance is claimed.
