---
name: voice-git-workflow
description: >-
  Git workflow for the Voice monorepo: branches (master, feature/*, fix/*),
  commits, sync, PR, merge policy — with strict no history rewrite and no hook
  bypass. Use when branching, committing, syncing, opening or merging PRs,
  resolving conflicts, or any git operation in Voice. Не rebase, не amend, не
  force-push — см. CONTRIBUTING и git-safety.
---

# Voice Git Workflow

Git-операции в Voice следуют **`docs/CONTRIBUTING.md`** и **`AGENTS.md`, `.agent/AGENTS.md`, and `docs/CONTRIBUTING.md`**. Этот skill — практический чеклист для агентов; при конфликте приоритет у CONTRIBUTING.

**Worktree / параллельные агенты** — отдельный skill: `the `treehouse` skill` (treehouse) or Codex worktree/thread tooling when explicitly chosen. Здесь — git **внутри** checkout/worktree.

---

## Когда применять

- Создание ветки, коммиты, push, PR, merge, sync с `origin/master`
- Любая git-команда в репозитории Voice
- После падения pre-commit / pre-push / local shell/git hook

**Не применять** для lifecycle worktree-пула — см. skill `treehouse`.

---

## Ветки и naming

| Ветка | Назначение |
|-------|------------|
| `master` | Default; deployable-кандидат после CI |
| `feature/<кратко>` | Новая функциональность |
| `fix/<кратко>` | Бugfix |
| `chore/`, `docs/` | По тому же принципу короткого суффикса |

```powershell
git fetch origin
git checkout master
git merge origin/master
git checkout -b feature/my-task
```

**Не** gitflow (нет `develop` / release/hotfix по умолчанию). **Не** переименовывать default branch в `main`.

---

## История Git — жёсткий запрет (без явной просьбы пользователя)

Voice **не переписывает историю** — ни локально, ни на remote. Хуки и local shell hooks and Codex project rules это блокируют.

### Запрещено

| Категория | Команды |
|-----------|---------|
| Remote rewrite | `git push --force`, `-f`, `--force-with-lease`, `--force-if-includes`, `+refspec` |
| Local rewrite | `git rebase` (включая `-i`), `git pull --rebase`, `git commit --amend`, `git reset --hard`, `git filter-branch`, `git filter-repo` |
| Hook bypass | `git commit --no-verify` / `-n`, `git push --no-verify` / `-n` |
| Destructive | `git clean -fdx`, force-delete веток с незапушенной работой |

### Если hook упал

1. Исправить файлы (lint, format, tests — что требует hook).
2. Сделать **новый коммит** с исправлениями.
3. **Не** предлагать `--no-verify`, amend или rebase как обход.
4. Сообщить пользователю, что упало и что исправлено.

### Sync с upstream

```powershell
git fetch origin
git merge origin/master
```

**Merge, не rebase.** Конфликты решать в рабочем дереве, затем обычный commit merge.

### Если hook заблокировал команду

Не искать workaround. Спросить пользователя или исправить причину и продолжить линейными коммитами.

Установка хуков (если ещё нет): `.\scripts\git\install-hooks.ps1` — см. `docs/DEV_SETUP.md`.

---

## Коммиты

- Сообщения — **на английском**; первая строка ~72 символа, суть изменения.
- Conventional Commits **не обязательны**.
- Один коммит — одна логическая правка; TDD-итерации могут давать несколько коммитов в одном PR — это нормально и **желательно** (история не схлопывается).
- **Не** смешивать массовое форматирование с поведенческими изменениями без необходимости.

```powershell
git add <paths>
git commit -m "Fix gateway transcode for empty message body"
```

---

## Pull Request

1. Один PR — одна смысловая задача (крупное — разбить; см. skill `split-to-prs` при необходимости).
2. Описание: **что** и **зачем**; ссылка на issue/задачу.
3. Перед review — проверки из `docs/TESTING.md` для затронутых пакетов.
4. Изменения `protos/` — `make buf-ci`, при API break — `make buf-breaking`, регенерация stubs.
5. Merge в `master`: **merge commit only**

```powershell
gh pr merge <number> --merge
```

**Squash merge не использовать** — скрывает TDD/review-итерации (CONTRIBUTING).

6. Self-merge после зелёного CI допустим для solo; в команде — чужой approve.

---

## Интеграция worktree / агентов

Правила из community worktree-практик, адаптированные под Voice:

| Правило | Действие |
|---------|----------|
| Один агент ↔ одна рабочая директория | Не делить один worktree между агентами |
| Интеграция через git | **commit → push → PR → merge** на `master` |
| Никогда `cp`/rsync в main checkout | Только merge/cherry-pick через git после commit в worktree |
| Удалять worktree | После merge PR или явного abandon (treehouse `return` / worktree cleanup) |

Cherry-pick допустим для переноса **уже закоммиченных** изменений; rebase для «подтягивания» master **запрещён** — используй `merge origin/master`.

---

## Pre-PR checklist (агент)

- [ ] Ветка от актуального `master` (`fetch` + `merge origin/master`)
- [ ] Коммиты линейные, без amend/rebase
- [ ] Локальные проверки по `docs/TESTING.md` для затронутого scope
- [ ] `protos/` — buf lint/format/breaking при необходимости
- [ ] Доки `docs/` обновлены, если менялось поведение/контракт
- [ ] Секреты не в diff
- [ ] Временные файлы только в `tmp/<session>/`, не в `src/` / `docs/`
- [ ] PR будет merged с `--merge`, не squash

---

## PowerShell

В Windows-терминале цепочки команд — через **`;`**, не `&&` (`docs/TESTING.md`).

---

## Связанные документы

| Тема | Файл |
|------|------|
| Политика Git/PR | `docs/CONTRIBUTING.md` |
| Хуки | `scripts/git/README.md`, `docs/DEV_SETUP.md` §3 |
| Правило агентов | `AGENTS.md` / `.agent/AGENTS.md` |
| Тесты перед PR | `docs/TESTING.md` |
| Worktree pool | `the `treehouse` skill` |
| Split больших изменений | split into smaller PRs per `docs/CONTRIBUTING.md` |

