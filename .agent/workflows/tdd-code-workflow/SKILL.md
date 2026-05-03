---
name: tdd-code-workflow
description: Documentation-first, plan-first, test-driven coding workflow for any implementation or bugfix task. Use when a coding agent (Cursor Agent, OpenAI Codex, Claude Code, or similar) must write, change, refactor, or repair code through documented requirements, a detailed plan, tests written before implementation, delegated review loops where available, incremental implementation, and final verification against documentation and plan.
---

# TDD Code Workflow

## Multi-tool layout (canonical path)

- **This file is canonical:** `.agent/workflows/tdd-code-workflow/SKILL.md`
- **Cursor Agent Skills:** project stub at `.cursor/skills/tdd-code-workflow/SKILL.md` points here — see [Agent Skills](https://cursor.com/docs/skills). Related customization: [Skills (help)](https://cursor.com/help/customization/skills).
- **OpenAI Codex (plugin metadata):** `.agent/workflows/tdd-code-workflow/agents/openai.yaml`
- **Repo-wide agent defaults:** `.agent/AGENTS.md`

Throughout this document, **delegation** means any supported mechanism: Cursor Task / parallel agents, Codex subagents, separate chat sessions, or other tool-specific parallel runs — not a single product.

## Non-Negotiable Rules

Use this workflow for every coding task unless the user explicitly asks for analysis only, review only, or no code changes.

### Strict mode when the user invokes this workflow

If the user **explicitly** asks to follow this skill/workflow (by name, by path `.agent/workflows/tdd-code-workflow/SKILL.md`, or the Cursor stub under `.cursor/skills/tdd-code-workflow/`), treat the rest of this document as **mandatory**, not a loose guideline:

- Produce a **written plan** (in chat, ExecPlan per `.agent/PLANS.md`, or another repo-agreed artifact) **before** editing production code: sources read, acceptance criteria, files to touch, verification commands, red–green sequence.
- **Do not** skip delegation for convenience: when Task / parallel agents / separate sessions are available, use them for **test authoring**, **test review**, **implementation**, and **implementation review** as described below; if a tool cannot delegate, still execute the same steps **in order** in one session.
- Keep **real** red–green–refactor: one documented behavior → failing test → minimal fix → green → refactor.
- Finish with the **final checklist** in this file; do not treat the workflow as satisfied by tests-only or implementation-only shortcuts.

Do not write production code before a plan exists and test work has started.

Use real TDD, not only "tests somewhere before final verification": drive implementation through repeated red-green-refactor cycles. First write a failing test for one documented behavior, then write the smallest production change that makes it pass, then refactor only while tests stay green.

Do not create the plan from memory alone. First read the project documentation that governs the requested behavior. Prefer repository docs, specs, ADRs, README files, API contracts, issue text, protocol definitions, comments that are treated as source-of-truth, and existing test descriptions. If documentation location is unclear, search the repository for likely docs before planning.

If the user request conflicts with documentation, stop before coding. Tell the user exactly which documented expectation conflicts with the request and ask whether the documentation should be changed. Do not silently prefer the user request over the documentation.

If documentation is missing or too vague to define expected behavior, state the gap in the plan and use the safest existing-code inference only for exploration. Ask the user before making product or contract decisions that documentation should own.

Write tests from documentation first. Delegate initial test writing to a separate agent or session whenever delegation is available. Give that delegate precise scope, documentation excerpts, target files, commands, and a warning not to touch unrelated code.

Review tests before implementation. Use fresh review passes (another agent, Task, or session) in a loop until there are no critical findings about documentation coverage, assertion quality, false positives, fixture realism, or accidental production-code changes.

Do not weaken, delete, or rewrite accepted tests merely to make implementation pass. Change accepted tests only when documentation or the plan was wrong, and record the reason.

Implement code only after tests are accepted. Give implementation delegates precise instructions, constraints, documentation excerpts, test expectations, ownership boundaries, and required test checkpoints.

Review implementation in a loop with separate reviewers when delegation exists. Continue until critical findings about correctness, documentation alignment, plan completion, maintainability, or test pass status are resolved.

Finish by explicitly checking that the plan, documentation expectations, tests, and implemented behavior all agree.

## Workflow

### 1. Read Documentation

Find and read the documentation before writing a plan. Use fast repository search first:

- Search for product requirements, feature docs, API docs, README sections, ADRs, protocol definitions, OpenAPI/protobuf schemas, comments referenced by docs, and existing tests that encode documented behavior.
- Read the smallest complete set of docs needed to understand the requested behavior.
- Record the specific files and sections used as the basis for the plan.
- Distinguish documented facts from inferences.

Stop if the requested change contradicts documentation. Report:

- The user-requested behavior.
- The documented behavior.
- The documentation source.
- The decision needed from the user, usually whether to update documentation first.

### 2. Analyze Requirements

Translate the documentation and user request into explicit expectations:

- External behavior and user-visible outcomes.
- Inputs, outputs, errors, status codes, events, persistence effects, logs, metrics, permissions, and edge cases.
- Compatibility constraints and invariants that must not change.
- Existing code paths likely involved.
- Testable acceptance criteria.
- Unknowns or assumptions.

If assumptions affect product behavior, ask before coding. If assumptions are purely technical and low risk, state them and proceed.

### 3. Create A Detailed Plan

Create a detailed plan before any code edit. Do not conserve tokens in the plan; include every relevant implementation and verification step needed to avoid ambiguity.

The plan must include:

- Documentation sources read.
- Confirmation that the request matches documentation, or the exact mismatch and stop decision.
- Requirements and acceptance criteria.
- Files/modules likely to change.
- Test strategy, including unit, integration, contract, regression, and negative tests as appropriate.
- The smallest useful sequence of red-green-refactor cycles.
- Test data and fixtures.
- Delegation assignments and ownership boundaries (who or which agent owns which files).
- Implementation strategy.
- Development order.
- Exact moments when tests must be run during implementation.
- Commands expected to verify the work.
- Risks and rollback/containment notes.
- Final completion checklist.

Use the task planning tool when available (e.g. Cursor todo list). Keep exactly one plan item in progress at a time and update it as work advances.

### 4. Write Tests First Via Delegation When Possible

Before production implementation, launch a fresh test-writing delegate when delegation is available and runtime policy allows it.

Give the test delegate:

- The user request.
- Documentation files and brief excerpts that define expected behavior.
- Acceptance criteria from the plan.
- The target test files or allowed test directories.
- Existing test conventions to follow.
- Commands for running relevant tests.
- A clear instruction not to modify production code unless adding test-only fixtures requires it.
- A clear instruction not to touch unrelated files or revert other work.
- Expected final output: changed files, what behavior each test covers, commands run, and failures observed.

If delegation is unavailable, write the tests locally but preserve the same discipline and scope.

Tests should fail for the right reason before implementation whenever feasible. If a test cannot be run yet because infrastructure is missing, record why and keep the assertion as precise as possible.

Prefer tests that describe externally observable behavior. Use unit tests for fast feedback around pure or isolated logic, contract/API tests for documented interfaces, integration tests for cross-component behavior, and regression tests for reported defects. Avoid excessive mocking that proves only the mock setup.

For bug fixes, first write a regression test that reproduces the documented or reported failure. Confirm it fails before fixing the bug whenever feasible.

### 5. Review Tests In A Loop

After initial tests are written, launch a fresh review delegate to inspect tests against documentation and the plan.

Ask the reviewer to check:

- Whether every documented acceptance criterion has test coverage.
- Whether tests assert behavior rather than implementation details.
- Whether at least one new or changed test fails for the expected reason before implementation, unless infeasible and explained.
- Whether negative, edge, and regression cases are included where risk warrants them.
- Whether tests can pass falsely.
- Whether fixtures are realistic and minimal.
- Whether test names describe documented behavior.
- Whether the test delegate touched unrelated or production files unnecessarily.
- Whether commands are adequate to prove the tests.

Treat critical findings as blockers. For every critical finding, run another test-writing pass, preferably with a new delegate and a narrower correction prompt. Then review again with a fresh reviewer. Continue until there are no critical findings.

Non-critical suggestions may be accepted, deferred, or documented with rationale.

### 6. Plan Implementation After Tests

After tests are accepted, expand or update the plan with implementation details:

- The concrete code changes needed for each failing test or acceptance criterion.
- The technology, framework APIs, libraries, and local helper patterns to use.
- The order of development.
- The red-green-refactor cycle for each behavior.
- The smallest useful test command to run after each step.
- The broader verification command to run at integration points.
- How to avoid accumulating errors across steps.

Use incremental TDD implementation:

1. Red: run the focused test and confirm it fails for the expected reason.
2. Green: make the smallest production change that satisfies that behavior.
3. Run the focused test again and confirm it passes.
4. Refactor: improve structure, names, duplication, boundaries, or performance without changing behavior.
5. Run the focused test after refactoring.
6. Move to the next documented behavior.
7. Run broader tests after related behaviors are complete.

Keep each cycle small. If a cycle needs many unrelated edits, split it into smaller documented behaviors or revise the plan.

### 7. Implement With Bounded Delegates

Launch implementation delegates when the work can be split safely, delegation is available, and runtime policy allows it. Use one or more delegates only when their ownership boundaries are clear.

Tell each implementation delegate:

- They are not alone in the codebase.
- They must not revert or overwrite unrelated edits.
- Their exact file/module ownership.
- The documentation excerpts and acceptance criteria relevant to their slice.
- The tests they are expected to make pass.
- The framework/library/local patterns to use.
- The order of work.
- The red-green-refactor cycle they must follow.
- Required checkpoints and commands after each step.
- Expected final output: files changed, behavior implemented, tests run, remaining failures, and risks.

Do not assign overlapping write scopes to parallel delegates. If scopes overlap, serialize the work or keep it local.

If implementing locally, follow the same bounded ownership and checkpoint rules.

### 8. Review Code And Behavior In A Loop

After implementation, launch separate review/verification delegates when available and policy allows. Use fresh reviewers for independent checks.

Ask reviewers to verify:

- Code behavior matches documentation.
- Code behavior satisfies the detailed plan and acceptance criteria.
- Tests cover the implemented logic fully enough for the risk.
- All new tests pass for the right reason.
- The implementation was driven by small red-green-refactor cycles rather than a large unverified change.
- Accepted tests were not weakened to fit the implementation.
- Existing behavior and compatibility are preserved.
- Error handling, concurrency, persistence, security, logging, and performance are adequate for the changed surface.
- The code follows existing project patterns and does not introduce unnecessary abstractions.
- No unrelated files or user changes were reverted.

Treat critical findings as blockers. For each critical finding:

1. Update the plan with the corrective action.
2. Assign a bounded fix to an implementation delegate or handle it locally.
3. Run the relevant tests.
4. Review again with a fresh reviewer.

Continue until no critical findings remain.

### 9. Final Verification

Before final response, perform a last local check:

- Re-read the plan.
- Confirm every plan item is complete or explicitly explained.
- Confirm every documented expectation is covered by tests or a justified verification method.
- Confirm each documented behavior went through an explicit red-green-refactor cycle, or explain why it could not.
- Run the agreed narrow and broad test commands where feasible.
- Confirm tests pass, or report exact failures and blockers.
- Inspect the final diff for unrelated edits.
- Confirm implementation, tests, documentation, and user request are consistent.

If documentation needed updates and the user approved them, verify documentation was updated together with tests and code.

## Subagent Prompt Templates

Use these shapes for **delegated** sessions (wording works across tools). Replace `...` with task-specific content.

### Test Writer

```text
Write tests first for this documented behavior. Do not implement production code.

User request:
...

Documentation sources and excerpts:
...

Acceptance criteria:
...

Allowed write scope:
...

Existing test patterns:
...

Commands to run:
...

Constraints:
- Do not touch unrelated files.
- Do not revert other changes.
- Prefer precise behavior assertions from documentation.
- Prefer externally observable behavior over implementation details.
- Include negative and edge cases where documented or high risk.
- For bug fixes, add a regression test that fails before the fix whenever feasible.

Final response:
- Files changed.
- Tests added and what each covers.
- Commands run and results.
- Which tests fail before implementation and why.
- Any gaps or blockers.
```

### Test Reviewer

```text
Review only the tests for documentation coverage and quality. Do not modify files unless explicitly asked.

Documentation sources and excerpts:
...

Plan and acceptance criteria:
...

Test files to inspect:
...

Check for:
- Missing documented behavior.
- Weak assertions or false positives.
- No demonstrated red phase for new behavior.
- Missing negative/edge/regression cases.
- Unrelated changes.
- Test commands and observed results.

Return findings by severity. Mark critical findings clearly.
```

### Implementation Worker

```text
Implement the documented behavior needed to satisfy the accepted tests.

You are not alone in the codebase. Do not revert or overwrite unrelated edits.

Documentation sources and excerpts:
...

Accepted tests and acceptance criteria:
...

Allowed write scope:
...

Implementation order:
...

Required red-green-refactor cycles:
...

Required checkpoints:
...

Technology and project patterns to use:
...

Final response:
- Files changed.
- Behavior implemented.
- Red-green-refactor cycles completed.
- Tests run and results.
- Remaining risks or failures.
```

### Code Reviewer

```text
Review the implementation against documentation, accepted tests, and the plan. Do not modify files unless explicitly asked.

Documentation sources and excerpts:
...

Plan:
...

Changed files:
...

Commands/results:
...

Check correctness, maintainability, test coverage, project patterns, unrelated edits, and remaining risk.
Check that accepted tests were not weakened and that implementation proceeded through small red-green-refactor cycles.
Return findings by severity and mark critical blockers clearly.
```

## Final Response

In the final user response, state:

- What was implemented.
- What tests were added.
- What verification commands ran and their results.
- Whether red-green-refactor cycles were completed or where they were not feasible.
- Any documentation mismatch, approved documentation change, skipped verification, or residual risk.

Keep the final response concise, but do not omit failing tests or unverified expectations.
