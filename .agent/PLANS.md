# ExecPlan Standard

Source: adapted for this repository from the OpenAI Cookbook article
"Codex Exec Plans": https://developers.openai.com/cookbook/articles/codex_exec_plans

Use this file whenever work is substantial, ambiguous, cross-service, risky, or
expected to span more than one focused edit. A good plan is a living design and
execution document, not a TODO list.

## Goals

An ExecPlan must let a future agent or developer complete the work from the
plan alone. It should explain:

- What user-visible or repository-visible outcome will exist when the work is done.
- Which project documents define the required behavior.
- What code, tests, and docs need to change.
- How to verify the result.
- What decisions were made while working and why.

For Voice, product and feature behavior must come from this repository only.
Before writing a plan, read the relevant sources listed in `AGENTS.md`.

## Required Sections

Each ExecPlan should include these sections unless the task is tiny enough that a
short in-chat checklist is clearly sufficient.

### Purpose

Describe the concrete outcome in user terms. Avoid vague goals like "improve the
service"; say what will work, where, and how it will be observed.

### Context

List the repository docs and code paths that define the task. Include current
state, constraints, and architectural boundaries. If a term is project-specific,
link it back to `docs/GLOSSARY.md` or the relevant feature/service document.

### Scope

Name what is in scope and what is deliberately out of scope. If the docs are
missing behavior, record the gap and either ask the user or point to
`docs/TODO.md`.

### Milestones

Break the work into observable milestones. Each milestone should produce a
repo-visible change or a verification result, not just "investigate" or
"implement".

### Detailed Steps

Write specific steps that can be executed in order. Include expected files,
commands, tests, migrations, generated artifacts, and any dependency between
steps. Keep the steps concrete enough that another agent can resume after an
interruption.

### Validation

State the exact checks that prove completion: unit tests, integration tests,
linting, generated code checks, manual flows, or docs consistency checks. If a
check cannot be run locally, say why and what evidence remains.

### Progress

Maintain this while working. Use timestamp-free checklist items unless the user
needs dates. Mark completed items promptly and add newly discovered necessary
work instead of hiding it.

### Decisions

Record important choices and their reasons. Tie decisions to project docs,
existing code patterns, test evidence, or explicit user direction. Do not invent
product behavior to fill a documentation gap.

### Risks And Follow-Ups

Capture unresolved risks, missing docs, migrations, rollout concerns,
compatibility issues, and follow-up work that should not be smuggled into the
current change.

## Quality Rules

- Keep the plan self-contained. A reader should not need hidden chat context.
- Keep it current. Update it after meaningful discoveries, completed milestones,
  failed assumptions, and changed scope.
- Prefer concrete evidence over intent: file paths, test names, commands, and
  observed results.
- Make every step resumable. If work stops halfway through, the plan should show
  what is done, what is next, and what is blocked.
- Preserve Voice architecture boundaries from `AGENTS.md` and `docs/`.
- Follow TDD for documented behavior: tests from docs, minimal implementation,
  green relevant checks, then refactor.
- Do not weaken tests to match incorrect code. Compare disputed expectations to
  the docs first.
- Do not expand product behavior beyond repository-backed specifications.

## Template

```md
# ExecPlan: <task name>

## Purpose

<Concrete outcome and why it matters.>

## Context

- Docs:
- Code:
- Current state:
- Constraints:

## Scope

- In:
- Out:
- Documentation gaps:

## Milestones

- [ ] <Observable milestone>
- [ ] <Observable milestone>

## Detailed Steps

1. <Read/confirm specific docs or code.>
2. <Add or update tests.>
3. <Implement the smallest change that satisfies the documented behavior.>
4. <Run checks and record results.>

## Validation

- [ ] `<command>` proves <expected evidence>

## Progress

- [ ] <Current work item>

## Decisions

- <Decision>: <reason/source>

## Risks And Follow-Ups

- <Risk or follow-up>
```
