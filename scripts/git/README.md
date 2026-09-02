# Git safety hooks (Voice)

Enforces **no history rewrite** and **no hook bypass** for humans and Cursor agents.

**Machine setup checklist:** [docs/DEV_SETUP.md](../../docs/DEV_SETUP.md) § «Git: политика истории и хуки».

## Install

From repo root:

```powershell
.\scripts\git\install-hooks.ps1
```

```bash
chmod +x scripts/git/install-hooks.sh scripts/git/hooks/*
./scripts/git/install-hooks.sh
```

This installs:

| Layer | Files | Blocks |
|-------|-------|--------|
| **Git** | `.git/hooks/pre-rebase`, `.git/hooks/pre-push` | rebase; non-fast-forward push |
| **Cursor** | `~/.cursor/hooks/block-history-rewrite.js` + `hooks.json` entry | force-push, rebase, amend, `--no-verify`, `reset --hard`, filter-branch |

Git hooks apply only to this clone. Cursor hook applies globally to all projects (user-level).

## Self-test (Cursor hook)

```bash
node scripts/git/selftest-history-rewrite.js
```

## Agent rule

`.cursor/rules/git-safety.mdc` (`alwaysApply: true`) — same policy for agents.

## Policy source

`docs/CONTRIBUTING.md` — section «История Git и хуки».
