---
name: voice-fleet-captain
description: 'Voice fleet captain playbook for Codex: delegate parallel work using Codex task/thread or multi-agent tools, maintain ignored tmp/fleet backlog, use treehouse worktrees for isolated code, and coordinate verification.'
---

# Voice Fleet Captain

Use this skill when work should be split across multiple services, Flutter plus backend, design plus code, or long verification. Live fleet state belongs under `tmp/fleet/` and must not be committed. Tracked `.agent/fleet/` stores only README/templates; Codex crew profiles are summarized in `.agent/codex/crew-profiles.md`.

## Principles

1. One user-facing captain keeps the user updated and integrates outcomes.
2. Delegate when scopes are independent or the work spans 2+ services.
3. Repository docs define behavior; read `AGENTS.md`, `.agent/AGENTS.md`, and the relevant `docs/*` first.
4. Write durable coordination state to `tmp/fleet/backlog.md` and `tmp/fleet/tasks/T-*.meta` when running a real fleet.
5. Do not create or update a tracked backlog with live PR/branch/CI statuses; those snapshots are local operator state and go stale quickly.
6. Use `treehouse` for parallel code isolation when multiple agents modify the repo.
7. Use `voice-git-workflow` for branches, commits, PRs, and no-history-rewrite rules.

## When To Delegate

| Situation | Action |
| --- | --- |
| One file, one service, or read-only question | Captain can handle directly |
| Gateway plus service, or 2+ backend services | Split by service/profile |
| Flutter plus backend | Separate Flutter and backend tasks |
| Protos plus implementation | Protos first or parallel with explicit contracts |
| Auth | Use the Java Auth profile only |
| Penpot/tokens/design | Use the designer/Penpot profile |
| Sign-off | Use the verify profile or verification skills |

## Crew Profiles

Read `.agent/codex/crew-profiles.md` and use the matching profile in the prompt when delegating through Codex tools. Cursor's `.cursor/agents/voice-*.md` files are the legacy adapter layer and may be used as extra reference, but Codex should prefer `.agent/codex/crew-profiles.md`.

## Task Lifecycle

1. Pick a short id (`T-001`, `T-002`, ...), outcome, scope, docs, validation, and owner profile.
2. If using a real fleet, create/update `tmp/fleet/tasks/T-<id>.meta` from `.agent/fleet/tasks/TEMPLATE.meta` and add the item to `tmp/fleet/backlog.md`.
3. For parallel code, lease a treehouse worktree and ensure the delegated task's workspace and shell both use that path.
4. Send a bounded prompt with outcome, scope, docs, worktree path, constraints, verification, and return format.
5. Integrate results, run or request verification, update backlog/meta, and report concise outcomes to the user.
6. After merge or abandon, return the treehouse lease.

## Delegation Prompt Shape

```markdown
## Fleet task T-<id>

**Outcome:** <what must be true>
**Scope:** <paths and service>
**Docs:** <specific docs to read>
**Worktree:** <main clone or treehouse path>
**Constraints:** docs-only behavior; no rebase/amend/force-push; no unrelated edits
**Verification:** <commands from docs/TESTING.md>
**Return:** Changed / Checked / Risk / PR / Blockers
```

## Anti-Patterns

- One agent owns overlapping write scopes with another.
- Workspace and shell point to different checkouts.
- File-copying changes from one checkout into another instead of using git.
- Leaving leased treehouse worktrees unreturned.
- Keeping fleet state only in chat during long or parallel work.
