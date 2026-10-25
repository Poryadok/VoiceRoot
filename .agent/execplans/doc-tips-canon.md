# ExecPlan: make operational lessons available outside Slave Driver

## Purpose

Move verified reusable guidance into existing Voice documentation, with one
canonical recipe per topic and a traceable disposition for every source item.
This is documentation work; it authorizes no environment mutation or rollout.

## Context and scope

TT task `DOC-TIPS-CANON` names the installed
`slave-driver/references/tips-and-tricks.md` as its source. The original 12,720
bytes are preserved in TT attachment `be913120dddbca8f61d0b5e50c930ec0`, SHA-256
`2d9b380987e002e12b2b4841ec80fc7524f913f8ad58adff0ab8c0676d953907`.
Repository baseline: `17967d3a3c29c155a1a9c8e64ac47742ac432610`.

Sources of canon: `docs/RELEASE_EVIDENCE.md`, `docs/TESTING.md`,
`docs/CONTRIBUTING.md`, `docs/DEV_SERVER.md`, `.agent/AGENTS.md`,
`.agent/codex/README.md`, and current guarded operator sources/tests. External
tool facts use primary documentation linked from the destination. No historical
live incident is reproduced, and old observations do not establish current state.

## Milestones and detailed steps

- [x] Preserve source before editing; enumerate 39 list items/paragraphs and header.
- [x] Compare recommendations with existing canon, implementation and primary docs.
- [x] Consolidate missing guidance into existing documents; link existing guidance.
- [ ] Replace migrated source recipes with links; retain uncertain observations.
- [ ] Validate links/diff/source preservation, obtain independent recovery review,
  then commit, push and open a PR with exact evidence.

## Validation

Check every matrix row, local target and heading; run `git diff --check` and
applicable documentation CI. Pure language fixtures may confirm isolated tool
semantics; they are not deployment proof. No product build, private Secret query,
live failure injection, deployment, rotation or migration is needed.

## Disposition matrix

Rows follow source order; a multi-sentence bullet remains one item. `Confirmed`
means the bounded recommendation is supported, not that its historical incident
was rerun. `Retained` identifies missing external/runtime evidence. Destinations
below are canonical sections, not competing copies of the recipes.

| ID | Source claim / applicability and type | Basis / current assessment | Canonical destination and action |
| --- | --- | --- | --- |
| 00 | Append-only lesson file; run STATE routing; orchestrator-specific rule | Original preserved; the invoked parent skill owns coordination | Source index defers routing to active orchestrator; Swarm uses TT/tmp/swarm, Slave Driver retains its STATE route |
| 01 | Dirty main; Voice worktree rule | AGENTS + CONTRIBUTING; confirmed | CONTRIBUTING: managed worktrees; link |
| 02 | Acquire exact lease/path/branch; ownership rule | treehouse skill + AGENTS; confirmed | CONTRIBUTING: managed worktrees; consolidate |
| 03 | Inspect exact diff/base before integration; rule | RELEASE_EVIDENCE integration section; confirmed | CONTRIBUTING: managed worktrees; link |
| 04 | Return may kill Flutter; tool observation + custody rule | Manager implementation/version absent; kill behavior unverified | CONTRIBUTING: general process attribution; retain tool-specific observation |
| 05 | Bind CI run/SHA/jobs/terminal result; evidence rule | RELEASE_EVIDENCE source/scenario sections; confirmed | RELEASE_EVIDENCE: inspect CI jobs; consolidate |
| 06 | Firewall failure is environment evidence; conditional workaround | TESTING runtime restrictions; historical machine state unverified | DEV_SERVER: execution and shared probes; preserve condition |
| 07 | Separate infra/setup and assertion failures; triage rule | TESTING negative controls + RELEASE_EVIDENCE; confirmed | RELEASE_EVIDENCE: verify intended scenario; consolidate |
| 08 | Completed-job REST logs; tool recipe | GitHub workflow-jobs API documents per-job log redirect | RELEASE_EVIDENCE: inspect CI jobs; link primary source, bounded availability |
| 09 | Green gate must execute helper; test rule | TESTING + current ci-script-tests selector; confirmed | RELEASE_EVIDENCE: verify intended scenario; consolidate |
| 10 | Bash output local collision; language fact | Bash reference dynamic scope; isolated fixture or hosted assertion needed for exact wrapper | RELEASE_EVIDENCE: test executed producers; bounded rule |
| 11 | PR-only absent file is not upstream deletion; Git fact | CONTRIBUTING + ancestry comparison; confirmed | RELEASE_EVIDENCE: integrate work; consolidate |
| 12 | Thumbnail may hide controls; visual evidence rule | RELEASE_EVIDENCE visual scope; confirmed | RELEASE_EVIDENCE: bound visual claims; consolidate |
| 13 | Jobs REST default 30/max100; API fact | GitHub workflow-jobs API pagination; confirmed | RELEASE_EVIDENCE: inspect CI jobs; consolidate |
| 14 | Run status does not determine job status; API/evidence rule | CI job states independently recorded; confirmed | RELEASE_EVIDENCE: inspect CI jobs; consolidate |
| 15 | Windows bash may select WSL shim; conditional tool fact | Exact PATH is machine-specific; TESTING has explicit restrictions | DEV_SERVER: execution and shared probes; retain conditional warning |
| 16 | exec namespaces can create distinct exception classes; Python fact | Python exec namespace semantics; source-level identity fixture | RELEASE_EVIDENCE: test executed producers; consolidate |
| 17 | Freeze consumed diagnostic bytes; evidence rule | RELEASE_EVIDENCE immutable evidence + guarded journals; confirmed | RELEASE_EVIDENCE: resumable operations; consolidate |
| 18 | Assert final persisted/returned diagnostic; test rule | Existing diagnostic report pipeline + tests; confirmed | RELEASE_EVIDENCE: test executed producers; consolidate |
| 19 | Caller-scoped handle is not OS PID or stopped process; tool caveat | Specific runtime unverified; general ownership safety applies | CONTRIBUTING: managed worktrees; retain runtime caveat |
| 20 | Distinguish provenance B0 and live event base B; CI recipe | GitHub PR context + RELEASE_EVIDENCE source binding; confirmed conditional | RELEASE_EVIDENCE: inspect CI jobs; preserve equality context |
| 21 | Secret formatting errors may disclose payload; security rule/incident | DEV_SERVER no-payload rule + GIS redaction tests; exact incident not rerun | RELEASE_EVIDENCE: Secret metadata; consolidate safety, preserve historical uncertainty |
| 22 | Buildx artifact can establish missing producer digest; recipe | gis_dev_first_install.py parser + gis_first_install_test.py | RELEASE_EVIDENCE: inspect CI jobs; conditional fallback |
| 23 | Secret projections include symlinks; runtime fixture rule | media_projection_test.py + Kubernetes AtomicWriter | RELEASE_EVIDENCE: test executed producers; preserve containment |
| 24 | Real process proves startup ordering; test rule | TESTING integration principles; unit parser cannot prove process order | RELEASE_EVIDENCE: test executed producers; consolidate |
| 25 | Test installed selector on synthetic dry-run, one metadata snapshot; recipe | GIS selector/redaction tests; no real Secret probe performed | RELEASE_EVIDENCE: Secret metadata; consolidate |
| 26 | Preserve Opaque type; type top-level; schema rule | Kubernetes Secret API + current issuers and strict schema tests | RELEASE_EVIDENCE: Secret metadata; consolidate |
| 27 | Resume journal incl. older fields; recovery rule | DEV runbook + guarded operation tests | RELEASE_EVIDENCE: resumable operations; consolidate without weakening pins |
| 28 | Same author owns source/tests; workflow rule | canonical TDD ordinary mode + Swarm; confirmed | TESTING/TDD canon already owns rule; link, no scheduler duplication |
| 29 | Hold shared DEV only for real mutation/probes; resource rule | DEV_SERVER explicit namespace/operation guard + Swarm | DEV_SERVER: execution and shared probes; bounded rule |
| 30 | Reuse normal private QA session; fixture rule | TESTING private fixture schema; no live credential check | TESTING existing DEV fixture section; link |
| 31 | WS handshake/history is not delivered message; scenario rule | TESTING request-id smoke + DM semantics | RELEASE_EVIDENCE: verify intended scenario; consolidate |
| 32 | Check branch requirements, not only visible green check; integration rule | CONTRIBUTING approve/CI + RELEASE_EVIDENCE freshness | RELEASE_EVIDENCE: inspect CI jobs; consolidate |
| 33 | UI keyboard/control bounds/incomplete fixture; UI evidence rule | TESTING a11y/fixture principles + visual canon | RELEASE_EVIDENCE: bound visual claims; consolidate |
| 34 | Test container persists beyond session; resource observation/rule | Tool/runtime dependent; ownership safety confirmed | CONTRIBUTING: managed worktrees; retain observation as conditional |
| 35 | Failed event checkout/blob beats blame; attribution rule | Git ancestry/source binding; confirmed | RELEASE_EVIDENCE: inspect CI jobs; consolidate |
| 36 | Save QA fixture before auth and stages atomically; fixture rule | TESTING private fixture schema; source-preserving recovery | TESTING existing DEV fixture section; consolidate |
| 37 | Post-completion RV can differ without template drift; recovery rule | Kubernetes resourceVersion + guarded post-operation comparisons | RELEASE_EVIDENCE: resumable operations; explicitly no in-progress relaxation |
| 38 | Actual CLI selector/defaulted Job differs from mock/submitted spec; producer/recovery rule | gis_first_install_test.py hostile output + guarded Job reconciliation | RELEASE_EVIDENCE: test executed producers / resumable operations; consolidate |
| 39 | Hardened issuer image needs writable temp mount; runtime rule/incident | Current issuer TemporaryDirectory + reviewed runbook mount contract; original incident receipt absent | RELEASE_EVIDENCE: test executed producers; retain historical claim as unverified |

## Decisions

- Existing RELEASE_EVIDENCE is already the shared source for many lessons and
  linked from TESTING, OPERATIONS, CONTRIBUTING and DEV_SERVER. Extend its focused
  sections rather than reproducing recipes across all entrypoints.
- Do not change installed project skills independently. Their repository source
  is `.agent/codex/skills`; `.agent/codex/install-local.ps1` is the normal full
  installer. This task does not run it because it replaces unrelated local skills.
  The installed-only tips reference is separately authorized by this task.
- Coordination routing belongs to the currently invoked parent skill. Swarm uses
  TT and `tmp/swarm/`; Slave Driver keeps its own `tmp/slave-driver/STATE.md` route.
  The reference defers to those instructions rather than duplicating or replacing
  either workflow. Preserve the original header in the archived source.

## Progress and risks

Source preserved and task claimed. No private payloads read and no live operation
executed. Exact Treehouse termination behavior, tool session scoping and original
incident receipts remain unverified; retain them as observations in the source
reference. Independent review must check recovery advice and non-Git source edits
before final delivery. See TT for current ownership, review and PR evidence.

Pure Python fixture executed on the author host confirms that separately executed
class definitions with the same name have different exception identities. No Bash,
Kubernetes or issuer runtime is executed for this documentation migration.
Local checks: 83 relative link paths and the three new canonical entry anchors
resolve; matrix IDs 00–39 are unique and complete; `git diff --check` passes.
