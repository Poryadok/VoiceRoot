# Design system (Voice client)

Источник визуала для Flutter: **design tokens в git** + макеты в Figma. UX-канон и визуальные принципы — [brand.md](brand.md); продуктовое поведение — в `docs/features/` (например [navigation.md](../features/navigation.md)).

> **Temporary status:** Figma сейчас недоступна. До восстановления доступа UI-работы ведём без Figma/MCP: источники истины — `docs/design/brand.md`, `docs/design/tokens.md`, `design/tokens/voice.tokens.json`, `docs/features/` и текущий Flutter UI.

## Figma

| | |
|--|--|
| **File** | [Voice](https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice) |
| **fileKey** | `tIkNxn3e7vcp3APJ8I6bKi` |
| **Референс Discord** | MainPage / «Discord 19» — только `00_References`, не Phase-1 target |

Структура файла: `00_References`, `01_Foundation`, `10_Screens_Desktop`, `11_Screens_Mobile`, `12_States`. Экран = **фрейм** `Screen/...`; в задачи агенту — URL с **node-id фрейма**, не canvas `0-1`.

Инвентарь фреймов: [screens.md](screens.md). Настройка страниц Figma: [figma-setup.md](figma-setup.md).

## Cursor + Figma MCP

- Плагин Figma в Cursor; авторизация — **OAuth Connect** (Settings → MCP → Figma → Connect). **Personal access token в MCP не нужен.**
- PAT только для опциональных REST-скриптов (`FIGMA_ACCESS_TOKEN` в локальном `.env`, не коммитить).
- На плане **Starter** лимитированы read-вызовы (`get_design_context`, `get_metadata`). **Канон цветов — `design/tokens/voice.tokens.json`**, не подбор hex из MCP.

## Design tokens (обязательно)

| Путь | Назначение |
|------|------------|
| [design/tokens/voice.tokens.json](../../design/tokens/voice.tokens.json) | Канон (нейтраль + `profileAccent.defaults`) |
| [src/frontend/assets/design/voice.tokens.json](../../src/frontend/assets/design/voice.tokens.json) | Runtime copy |
| [tokens.md](tokens.md) | Семантика |
| [brand.md](brand.md) | Стиль и UX-референсы |

Синхронизация: `make design-tokens-check` (сравнение canonical ↔ asset).

Правило PR: смена визуальных констант → diff в `voice.tokens.json` (+ asset); правки hex в `lib/ui/**` без JSON — блокер.

## UX baseline

- Базовые messaging-сценарии проектируем ближе к Telegram: быстро, спокойно, без лишних окон.
- Спейсы, голосовые комнаты, роли и community-flow проектируем ближе к Discord, но без копирования его визуального шума.
- Новые Voice-only фичи, включая матчмейкинг, должны иметь короткий happy path, явное состояние процесса и понятный cancel/retry.
- Визуально держим flat style: нейтральные поверхности, точечный accent, умеренные скругления, без "вырвиглаз" цветов.

## Шаблон задачи (UI)

```text
Figma frame: unavailable while Figma access is down
Tokens: design/tokens/voice.tokens.json
UX: docs/design/brand.md
Spec: docs/features/...
Use lib/ui/core/* and VoiceTheme only. Do not depend on Figma/MCP until access is restored.
```

## Flutter

- [src/frontend/lib/theme/](../../src/frontend/lib/theme/) — `VoiceTheme`, `VoiceColors`
- [src/frontend/lib/ui/core/](../../src/frontend/lib/ui/core/) — кнопки, поля (без сырых `Color(0x…)` в feature UI)
- Правило Cursor: `.cursor/rules/voice-design.mdc`
