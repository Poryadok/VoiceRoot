# ExecPlan: Prepare Voice for GPT-6 Astra

## Purpose

Make GPT-6 Astra the portable Codex default for Voice and remove conflicting
agent guidance that causes unnecessary pauses or verification work.

## Context

- User request: prepare this project for GPT-6 Astra using official OpenAI guidance.
- Sources: `AGENTS.md`, `.agent/AGENTS.md`, `.agent/PLANS.md`, `docs/PLAN.md`,
  `docs/TESTING.md`, `docs/CONTRIBUTING.md`, `.agent/codex/README.md`.
- Official guidance retrieved 2026-09-05:
  [Astra migration and prompting](https://developers.openai.com/api/docs/guides/latest-model?model=gpt-6-astra),
  [Codex project configuration](https://learn.chatgpt.com/docs/config-file/config-basic).
- Initial checkout: clean `master`, synchronized with `origin/master` at `7fa2fba6`.
- No OpenAI API call sites found in `src/`, `scripts/`, or `.github/`.
- Local Codex already selects `gpt-6-astra`; there is no project `.codex/config.toml`.
- The portable TDD copy predates the canonical strict-mode section and both
  versions demand a pause for every user/docs conflict.

## Scope

- In: project model default, shared instruction precedence/autonomy, useful
  delegation, proportional verification, TDD scope and portable-copy consistency,
  existing Codex setup documentation, affected installed project skill copies.
- Out: product behavior, service code, API integration, credentials, permissions,
  provider settings, pricing, global model/effort changes, commits and publication.
- Preserve architecture, git safety, explicit strict TDD, and required CI checks.

## Milestones

- [x] Fetch official guidance and audit configuration/instructions independently.
- [x] Update project configuration and instruction surfaces.
- [x] Validate configuration, skill metadata and links; independently review diff.

## Detailed Steps

1. Add `.codex/config.toml` containing only the explicit model target; inherit effort.
2. Add shared rules in `.agent/AGENTS.md`, referenced from root `AGENTS.md`.
3. Align TDD scope with `docs/TESTING.md`; preserve strict explicit invocation,
   accept explicit user changes with matching docs updates, ask only about
   unresolved product decisions, and avoid repeating successful checks.
4. Synchronize the portable TDD copy from its canonical source. Apply any other
   narrow instruction corrections supported by the audit, recording them below.
5. Update `.agent/codex/README.md` with activation, verification and official links.
6. Validate TOML with Python, inspect the current installed Codex model catalog,
   validate changed skills with `quick_validate.py`, inspect links and diff.
7. Refresh only affected installed Voice skill files after checking for local drift.
8. Obtain an independent review and record actual evidence.

## Validation

- Parse `.codex/config.toml`; it must set `model = "gpt-6-astra"` without
  overriding reasoning, permissions or authentication.
- Use a read-only Codex diagnostic to confirm the current catalog exposes Astra.
  Project configuration activation is checked in a new task's model selector.
- Run the skill validator and compare canonical/portable TDD content.
- Check newly added local Markdown targets and `git diff --check`.
- Review behavior for an ordinary docs edit, explicit strict TDD, an explicitly
  changed requirement, and a genuinely unresolved product decision.
- Service test suites are outside this configuration/documentation-only change.

## Progress

- [x] Official guide and Codex config docs opened and read.
- [x] Independent instruction audit completed.
- [x] Main checkout fetched and merged with `origin/master` (already current).
- [x] Changes and installed-copy synchronization complete.
- [x] Validation and independent review complete.

Evidence:

- Python `tomllib` parsed the project config and confirmed its only key is
  `model = "gpt-6-astra"`.
- `codex debug models` succeeded on CLI `0.153.0-alpha.5`; the refreshed catalog
  exposes Astra and the existing local `ultra` effort, so effort was preserved.
- `quick_validate.py` passed for the canonical TDD skill and all three changed
  portable skills. SHA-256 checks confirmed installed copies matched before
  refresh; only those three `SKILL.md` files were copied, without directory deletion.
- Canonical, portable, and installed TDD files are byte-identical. Installed
  fleet/verification skills match their project sources.
- All four local Markdown link targets in setup documentation exist.
- `git grep` over tracked `src`, `scripts`, `.github` confirmed no OpenAI API/model
  references. `git diff --check` passed after normalizing touched Markdown to LF.
- Independent review passed all five scenarios: docs typo, explicit strict TDD,
  explicit requirement change, unresolved product choice, same-service audit.

## Decisions

- Keep effort inherited: migration guidance preserves a compatible effective
  effort; API documentation does not define all local Codex effort presets.
- Keep existing crew `model: inherit` settings rather than pinning every worker.
- Narrow full-verification skill discovery to whole-repo/cross-cutting checks;
  clarify internal subagents versus user-visible tasks in fleet skill and roster.
- Edit the clean main checkout for this single-writer documentation/configuration
  task; delegated agents only read files. Leave changes available for review.

## Risks And Follow-Ups

- Project config requires a trusted project and does not override an explicit
  model selection. An existing task may retain its selected model.
- Configuration validation does not prove paid API access or benchmark model
  quality; no product API integration or model evaluation is part of this task.
- Diagnostic adjustments: this CLI rejects `--strict-config` with `debug`, and
  its bundled catalog predates Astra. The supported refreshed `debug models`
  command succeeded and is documented. `debug prompt-input` timed out after 45s;
  prompt assembly/new-task activation was not verified by that command.
