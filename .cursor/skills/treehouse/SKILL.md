---
name: treehouse
description: >-
  Treehouse worktree pool for parallel agent work in Voice: get/return/lease,
  status, prune, in-project pool (root=.), Cursor workspace alignment. Use when
  treehouse, worktree pool, parallel agents, isolated checkout, treehouse get,
  or running multiple tasks on the same repo without branch conflicts.
---

# Treehouse (Voice)

[Treehouse](https://github.com/kunchenguid/treehouse) — CLI, который держит **пул переиспользуемых git worktree** per repo. Даёт изоляцию для параллельных агентов без `git clone` на каждую задачу и без ручного `git worktree add/remove`.

**Git внутри worktree** — skill `voice-git-workflow` (ветки, commit, PR, **без rebase/amend/force-push**).

---

## Когда treehouse vs Cursor `/worktree`

| Ситуация | Инструмент |
|----------|------------|
| Терминальный агент, долгоживущий пул, кэш deps | **treehouse** |
| Быстрый one-off в Cursor IDE, `/worktree`, best-of-n | **Cursor worktrees** ([docs](https://cursor.com/docs/configuration/worktrees)) |
| Один агент, одна задача, основной checkout | Ни то ни другое — работай в основном clone |

**Не смешивать** два механизма на одну задачу. Выбери один и держи workspace/терминал в **одном** path.

---

## Voice defaults

### In-project pool (`root = "."`)

По умолчанию treehouse кладёт пул в `~/.treehouse` (часто диск `C:`). Для Voice **предпочтителен in-project pool** — worktree рядом с репо на том же диске:

```toml
# treehouse.toml в корне репозитория
max_trees = 16
root = "."
```

Создать конфиг: `treehouse init`, затем добавить `root = "."`. Пул: `<repo>/.treehouse/` (gitignore добавляется автоматически).

Разово: `treehouse get --root .`

### Cursor workspace = path worktree

**Критично:** если Cursor workspace открыт на `D:\Git\Voice`, а терминал в worktree на `C:\Users\...\.treehouse\...`, агент правит **не те файлы**.

Перед работой:

1. Узнать path: вывод `treehouse get` / `treehouse status` / `treehouse get --lease --json`
2. **Workspace / agent root** = этот path (`move_agent_to_root` или открыть папку worktree в Cursor)

Внутри worktree действуют те же `.cursor/rules/`, `docs/`, TDD — это полная копия репо.

---

## Жизненный цикл задачи

```text
repo root → treehouse get [--root .]
    → cd в worktree (или subshell уже там)
    → feature/fix branch, работа, commit, push
    → PR → merge в master (voice-git-workflow)
    → exit subshell / treehouse return
    → (опционально) treehouse prune --yes для stale слотов
```

### Interactive (subshell)

```powershell
cd D:\Git\Voice
treehouse get --root .
# subshell в worktree; exit — вернуть слот в пул
```

### Automation (lease, без subshell)

```powershell
$path = treehouse get --lease --root .
# stdout = только path; баннеры в stderr
# ... agent работает в $path ...
treehouse return --force $path
```

`--lease`: слот не отдаётся другим, пока не `return`. Для retry-safe automation: `--if-lease-id` / `--if-lease-holder`.

---

## Команды (шпаргалка)

| Команда | Назначение |
|---------|------------|
| `treehouse` / `treehouse get` | Взять worktree + subshell |
| `treehouse get --root .` | In-project pool |
| `treehouse get --lease` | Lease без subshell; stdout = path |
| `treehouse get --lease --json` | Path + lease_id для automation |
| `treehouse status` | Пул: idle / in-use / leased |
| `treehouse return [path]` | Вернуть слот (clean + reset) |
| `treehouse enter <n>` | Subshell в существующий слот |
| `treehouse prune` | Dry-run удаления stale idle |
| `treehouse prune --yes` | Удалить безопасные stale |
| `treehouse destroy <path>` | Dry-run удаления одного слота |

**Безопасность по умолчанию:** `prune` и `destroy` — dry-run; `--yes` только после просмотра preview. **Не** использовать `--include-unlanded --yes` без явной просьбы пользователя (потеря незакоммиченной работы).

---

## Bootstrap monorepo (deps, env)

Treehouse **переиспользует** worktree с уже установленными deps — это главный выигрыш. Первый заход в новый слот может потребовать:

| Область | Типичная команда (из корня worktree) |
|---------|--------------------------------------|
| Go backend | `make build-all` или `go test` в нужном модуле |
| Flutter | `cd src/frontend ; flutter pub get` |
| Node (admin/portal) | `npm ci` в соответствующем пакете |
| `.env` | Скопировать из main checkout вручную (не symlink) |

Опционально: user-level hooks в `~/.config/treehouse/config.toml`:

```toml
[hooks]
post_create = ["powershell -File D:/Git/Voice/scripts/worktree-bootstrap.ps1"]
```

**Repo-level `[hooks]` в `treehouse.toml` игнорируются** (безопасность). Hooks выполняют shell-команды — только доверенный user config.

Если нужен полноценный audit + скрипт bootstrap — см. [imsai-sh/git-worktree-setup](https://github.com/imsai-sh/git-worktree-setup) (отдельная задача, не часть treehouse CLI).

---

## Правила для агентов

1. **Один агент ↔ один worktree path** — не делить слот.
2. **Commit в worktree**, интеграция через **PR + merge commit** на `master` — не `cp` в main checkout.
3. **Не** `git worktree add/remove` вручную, если в проекте принят treehouse.
4. **Не** `treehouse destroy --include-unlanded --yes` без явного запроса.
5. Временные файлы — `tmp/<session>/` (как в CONTRIBUTING).
6. Sync с master: `git fetch origin ; git merge origin/master` — **не rebase** (`voice-git-workflow`).

---

## Anti-patterns

| ✗ | ✓ |
|---|---|
| Workspace на main, терминал в treehouse WT | Один path для IDE и shell |
| `cp` изменений в основной checkout | commit → push → PR |
| Repo-level hooks в `treehouse.toml` | User hooks в `~/.config/treehouse/config.toml` |
| Забытый leased слот | `treehouse status` → `return` или `destroy` |
| Два параллельных механизма WT | treehouse **или** Cursor `/worktree` |

---

## Установка

Windows (если нет в PATH):

```powershell
# бинарник: %LOCALAPPDATA%\treehouse\treehouse.exe
treehouse --version
```

См. [treehouse README](https://github.com/kunchenguid/treehouse) — Go, Nix, install script.

---

## Fleet captain

Параллельные crew в Multitask → skill `.cursor/skills/voice-fleet-captain/SKILL.md` (backlog, brief, spawn). Treehouse выдаёт path; captain записывает в `tasks/T-*.meta`.

---

## Связанные документы

| Тема | Файл / skill |
|------|----------------|
| Fleet captain / backlog | `.cursor/skills/voice-fleet-captain/SKILL.md` |
| Git, PR, no rewrite | `.cursor/skills/voice-git-workflow/SKILL.md` |
| Git safety | `.cursor/rules/git-safety.mdc` |
| Machine setup + hooks | `docs/DEV_SETUP.md` |
| Cursor worktrees | https://cursor.com/docs/configuration/worktrees |
| Upstream treehouse | https://github.com/kunchenguid/treehouse |
