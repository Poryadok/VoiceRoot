---
name: voice-fleet-captain
description: >-
  Voice fleet captain playbook for Cursor Multitask: parent agent delegates to
  crew subagents, maintains ignored tmp/fleet backlog, treehouse worktrees for
  parallel code, ExecPlans, verification. Use when orchestrating multiple
  parallel tasks, spawning crew agents, or running the Voice lite fleet workflow.
---

# Voice fleet captain

Parent agent в **Multitask Mode** = **капитан**. Crew = Cursor subagents из `.cursor/agents/voice-*.md`. Дисковый live-state = `tmp/fleet/`; tracked `.agent/fleet/` хранит только README и шаблоны.

**Не** клонировать firstmate. **Не** смешивать treehouse и Cursor `/worktree` на одну задачу.

---

## Prime directives

1. **Один вход** — капитан общается с пользователем; crew не «чатят» с капитаном напрямую (результаты приходят в parent чат).
2. **Делегируй** — работа на 2+ сервисов, отдельный PR, долгий TDD, design + code → **Task** на crew, не монополизируй parent turn.
3. **Docs-first** — поведение из `docs/`; продукт не выдумывать (`voice-project.mdc`, `.agent/AGENTS.md`).
4. **Диск > память** — `tmp/fleet/backlog.md` и `tmp/fleet/tasks/*.meta` обновлять при dispatch, progress, done.
5. **Live-state не в git** — не создавать и не обновлять tracked backlog со статусами PR/веток/CI; такие снапшоты быстро устаревают.
6. **Git** — `voice-git-workflow`; **worktree** — `treehouse` skill.

---

## Когда делегировать (эвристика)

| Ситуация | Действие |
|----------|----------|
| Один файл / один сервис / вопрос без кода | Parent может сделать сам |
| 2+ backend сервисов или gateway + сервис | Разнести на crew |
| Flutter + backend в одной фиче | `voice-flutter` + go crew параллельно |
| Penpot / tokens / макеты | `voice-designer` |
| `protos/` + сервис | `voice-protos` сначала или вместе с go crew |
| Auth Java | только `voice-java-auth` |
| Полная проверка перед merge | `voice-verify` или module skills |

---

## Crew roster

| Agent | Когда |
|-------|--------|
| `voice-go-gateway` | API Gateway, REST edge, transcode |
| `voice-go-realtime` | WebSocket, resume, NATS fan-out |
| `voice-go-chat-messaging` | Chat + Messaging (список чатов, сообщения, cursor) |
| `voice-go-backend` | Остальные Go сервисы (`src/backend/<name>/`) |
| `voice-java-auth` | Auth, JWT, Spring (`src/backend/auth/`) |
| `voice-flutter` | `src/frontend/`, Flutter web/mobile |
| `voice-protos` | `protos/`, buf lint/breaking/generate |
| `voice-verify` | CI-parity checks, sign-off (`voice-project-full-verification`) |
| `voice-designer` | Penpot, tokens, UX (уже есть) |

Карта сервисов: `docs/MICROSERVICES.md`, пути в коде: `docs/PLAN.md`.

---

## Жизненный цикл задачи

### 1. Intake (parent)

- Понять outcome для капитана (пользователя).
- Выбрать crew и scope (путь в monorepo).
- Назначить `id` — `T-001`, `T-002`, … (следующий свободный в `tmp/fleet/backlog.md`).
- Создать `tmp/fleet/tasks/T-<id>.meta` из `.agent/fleet/tasks/TEMPLATE.meta`.
- Добавить строку в `tmp/fleet/backlog.md` (queued или in flight).
- Для нетривиальной работы — ExecPlan в `.agent/execplans/` (см. `.agent/PLANS.md`); путь в `execplan=` meta.

### 2. Worktree (если параллельный **код**)

Параллель с другой in-flight задачей на код → **treehouse**, не общий checkout.

```powershell
cd D:\Git\Voice
$wt = treehouse get --lease --root .
# stdout = path only
```

Записать в meta: `worktree=<path>`, `treehouse_lease=` при `--json`.

**Workspace = worktree path** — subagent/терминал должны работать в `$wt`. В Cursor: открыть папку worktree или `move_agent_to_root` на этот path.

Одна задача, никого в flight на код → можно работать в основном clone (worktree опционально).

### 3. Brief для Task

Каждый spawn — структурированный prompt:

```markdown
## Fleet task T-<id>

**Outcome:** <что должно быть true для капитана>
**Scope:** <paths, сервис, docs to read>
**ExecPlan:** <path or "none">
**Worktree:** <path or "main clone">
**Branch:** <feature/... or "create from master">
**Constraints:** docs-only behavior; no rebase/amend/force-push; protos → buf-ci
**Verification:** <make targets / tests from docs/TESTING.md>
**Return format:**
- Changed
- Checked
- Risk
- PR URL if any
- Blockers / captain decisions
```

### 4. Spawn

```
Task tool → subagent_type generalPurpose (or explore for read-only)
model inherit
prompt = brief above
```

Для design — `voice-designer` agent file. Для узкого go — указать в prompt читать соответствующий `.cursor/agents/voice-go-*.md` charter.

Обновить meta: `status=in-flight`, `updated=`, `crew=`.

### 5. Relay (parent)

- Сообщить капитану **outcomes**, не внутреннюю кухню fleet (task id в user chat — только если полезно для отслеживания).
- Crew **всегда** отчитывается; не просить «молчи».
- `captain-hold` — одна decision per message, с опциями.

### 6. Done

- Meta: `status=done`, `pr=`, `verification=passed|…`
- Backlog: перенести в Recently done.
- Worktree: после merge PR — `treehouse return --force <path>` (или `return` из subshell).
- Удалить или архивировать meta при желании.

### 7. Abandon

`status=abandoned`, причина в `brief`/`notes`, return worktree если был.

---

## Параллельность

- Несколько **Task** в одном parent turn — ок для независимых crew.
- Два crew в **одном** worktree — **запрещено**.
- Один сервис — один in-flight crew (или явно разные scopes без overlap файлов).

---

## Интеграция с TDD workflow

Если пользователь просит `tdd-code-workflow` — crew следует `.agent/workflows/tdd-code-workflow/SKILL.md` внутри своего scope. Captain не сокращает шаги «ради скорости».

---

## Anti-patterns

| ✗ | ✓ |
|---|---|
| Parent правит gateway + flutter + protos в одном потоке | 3 crew или 2 + protos first |
| Workspace main, shell в treehouse | Один path |
| `cp` файлов в main checkout | commit → PR → merge |
| Забытый leased treehouse | `treehouse status` → return |
| Backlog только в чате | `tmp/fleet/backlog.md` + meta |
| treehouse + Cursor `/worktree` на одну задачу | Один механизм |

---

## Связанные skills

| Skill | Тема |
|-------|------|
| `treehouse` | worktree pool |
| `voice-git-workflow` | branches, PR, no rewrite |
| `tdd-code-workflow` | docs-first TDD |
| `go-microservice-task-evaluation` | Go sign-off |
| `java-microservice-task-evaluation` | Auth sign-off |
| `flutter-web-client-testing` | Flutter checks |
