# Full application target composition

## Purpose and contract
For the source-bound full application request described in docs/DEPLOYMENT.md
(canonical full application build), compile all 23 application images into one
protected final target. Existing frontend environment, mounts, ingress and data
configuration stay captured; only their selected image pins change. GIS first
installation remains create-only and separately qualified, but does not truncate
the final target to GIS. Legacy subset and images-only behavior stays unchanged.

## Ownership and design
WT29 owns compiler.py/source_plan.py and the new composition test. WT25 owns
bridge/normalization/transaction consumer wiring and exact request-path allowance
in guard. WT34 owns Session and terminal23 verification. No overlapping edits.
The discriminator is derived from exact captured full-build request bytes and
complete application image inventory, never an operator boolean. Its hash is
part of target source_hashes. Authority still comes from protected source/Session.

## Acceptance and risk
High-risk cross-service composition: real compiler and source planner against
actual repository manifests; synthetic external Kubernetes only. Assert old
frontend config survives new image pins; unchanged frontend still remains in
the complete target; cold GIS is excluded only from generic apply; all existing
deployment checks remain. Reject malformed request, partial scope and extra
images. Bind the actual compiled target through Session consumer tests. Independent
review is required; no live activation follows these tests.

## Progress
- Inspected compiler, source planner, normalization and bridge gates. Existing
  GIS isolation and omitted preserved frontends explain the incomplete target.
- Coordinated fixed full_application_scope/full_application_target interfaces
  with Stage; consumer changes remain their exclusive scope.
- Meaningful red: actual compiler reduced cold GIS to one image and discarded
  captured frontend environment/annotations. Both cases now green. Full and
  app-only preserve existing frontend non-Deployment objects and include no-op
  frontend templates in the complete target.
- Four focused Linux tests pass, including actual RootJournal/Session and
  historical_binding with synthetic external TT/Kube. The canonical portal
  container is `voice-developer-portal/portal`, not `.../developer-portal`.
- Legacy compiler/source-plan/GIS suite: 44/44 PASS, 43.835s, Linux Python3.12
  retained isolated runner, source archive 4f6f64190b6d plus this working delta.
  Command: `python -m unittest full_application_compiler_test compiler_test
  source_plan_test gis_compiler_test`. Actual tool transcript retained in chat;
  no separate raw log was captured for this run. Stage owns normalization/apply integration, so this
  checkpoint alone is not complete rollout acceptance.
- Stage consumer 14e395ee84c3 normally merged. Actual compiler/normalizer/paused
  apply, RootJournal/Session, terminal consumer and preservation tests pass
  39/39 in 24.627s (synthetic external Kube/TT). Fixed smoke source import closure
  is qualified before the cut, without executing credentials or HTTP. Missing
  captured dependency is rejected even after module caching; final focused test
  passes. Existing source/helper authority remains independently required.
- Historical Space completion omission reproduced through the actual historical
  loader and fixed in 6f0eb7edaa21; source-owned migration selection mandates the
  nonrecursive native validator after full prior PASS ancestry. No-Space rejects
  unexpected native records, events and dangling directory symlinks.
- Independent review requested from release_history; no overall live sign-off.

## Validation and remaining work
Run focused new tests red/green, then compiler/source-plan/GIS legacy tests and
the joined Session/normalizer consumer test. Publish immutable source and evidence.
No whole-repository or live readiness claim. Root owns the shared AST update.
