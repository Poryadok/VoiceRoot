# Voice fleet (lite)

Локальный **роевой workflow** для Cursor Multitask: один parent (капитан) + crew subagents + дисковый backlog. Не firstmate — только то, что нужно для Voice monorepo.

## Файлы

| Файл | Назначение |
|------|------------|
| `backlog.md` | Живой backlog: queued / in flight / captain hold / done |
| `tasks/TEMPLATE.meta` | Шаблон мета-задачи |
| `tasks/T-*.meta` | Одна строка = одна задача (локально, в `.gitignore`) |

## Когда использовать

- 2+ параллельные задачи (разные сервисы, Flutter + backend, design + code).
- Нужно помнить «что ещё летит» после compaction или новой сессии.
- Параллельный **код** — через **treehouse** (skill `.cursor/skills/treehouse/SKILL.md`).

## Капитан

Parent agent в Multitask:

1. Читает skill `.cursor/skills/voice-fleet-captain/SKILL.md`.
2. Делегирует через **Task** на `.cursor/agents/voice-*.md`.
3. Обновляет `backlog.md` и `tasks/<id>.meta`.
4. Не монополизирует код на несколько сервисов в одном потоке.

## Treehouse

Пул worktree в репо: `treehouse.toml` (`root = "."`). Один crew ↔ один path. См. skill `treehouse`.

## Git

Коммиты и PR — skill `voice-git-workflow`. Worktree: commit → push → PR → merge → `treehouse return`.
