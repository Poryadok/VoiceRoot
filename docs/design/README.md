# Design system (Voice client)

Источник визуала для Flutter: **design tokens в git** + макеты в **Penpot**. UX-канон — [brand.md](brand.md); продуктовое поведение — в `docs/features/` (например [navigation.md](../features/navigation.md)).

## Penpot

| | |
|--|--|
| **Hosting** | [design.penpot.app](https://design.penpot.app) |
| **File ID** | `20d3f736-cc1b-8043-8008-561cb65228ef` |
| **Setup** | [penpot-setup.md](penpot-setup.md) |
| **Workflow** | [penpot-workflow.md](penpot-workflow.md) — канон ↓ / варианты →, clip, placeholder content |

Структура файла: `00_References`, `01_Foundation`, `10_Screens_Desktop`, `11_Screens_Mobile`, `12_States`, `13_Panels_Desktop`, `14_Panels_Mobile`, `15_Overlays`. Экран = **фрейм** `Screen/...`, `Panel/...` или `Overlay/...`; в задачи — share URL фрейма из [screens.md](screens.md).

Инвентарь фреймов: [screens.md](screens.md).

## Cursor + Penpot MCP

- MCP server в Cursor (см. [penpot-setup.md](penpot-setup.md)); ключ — **Integrations** в Penpot, не коммитить.
- Файл Voice открыт в браузере, **File → MCP Server → Connect**.
- **Канон цветов — `design/tokens/voice.tokens.json`**, не подбор hex из MCP.

## Design tokens (обязательно)

| Путь | Назначение |
|------|------------|
| [design/tokens/voice.tokens.json](../../design/tokens/voice.tokens.json) | Канон (нейтраль + `profileAccent.defaults`) |
| [src/frontend/assets/design/voice.tokens.json](../../src/frontend/assets/design/voice.tokens.json) | Runtime copy |
| [tokens.md](tokens.md) | Семантика |
| [brand.md](brand.md) | Стиль и UX-референсы |

Синхронизация: `make design-tokens-check` (canonical ↔ asset). Penpot — зеркало: `make penpot-tokens-export`.

Правило PR: смена визуальных констант → diff в `voice.tokens.json` (+ asset) + re-sync Penpot; правки hex в `lib/ui/**` без JSON — блокер.

## UX baseline

- Базовые messaging-сценарии — ближе к Telegram: быстро, спокойно, без лишних окон.
- Спейсы, голос, роли — ближе к Discord по логике, без визуального шума.
- Voice-only фичи (матчмейкинг) — короткий happy path, явное состояние, cancel/retry.
- Flat style: нейтральные поверхности, точечный accent, умеренные скругления.

## Шаблон задачи (UI)

```text
Penpot frame: <URL or frame ID from docs/design/screens.md>
Tokens: design/tokens/voice.tokens.json
UX: docs/design/brand.md
Spec: docs/features/...
Use lib/ui/core/* and VoiceTheme only. Penpot MCP: file open + Connect.
```

## Flutter

- [src/frontend/lib/theme/](../../src/frontend/lib/theme/) — `VoiceTheme`, `VoiceColors`
- [src/frontend/lib/ui/core/](../../src/frontend/lib/ui/core/) — кнопки, поля (без сырых `Color(0x…)` в feature UI)
- Правило Cursor: `.cursor/rules/voice-design.mdc`

## Legacy Figma

Архив: [figma-setup.md](figma-setup.md) (не использовать для новых задач).
