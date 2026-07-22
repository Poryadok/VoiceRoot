# TODO — Design

[← Индекс](../TODO.md)

Penpot, design tokens, screen frames, parity дизайн ↔ Flutter (`docs/design/`, `design/tokens/`).

## Critical

_Пока пусто._

## High

### Penpot v2 — review и polish

- [ ] **Design review v2 (pages 11–15)** — пройти все `· v2` в Penpot viewer, собрать замечания или approve. Канон (`x≈0`) не трогать; `screens.md` для draft с `·` не обновлять ([penpot-workflow.md](../design/penpot-workflow.md) §1).
- [ ] **Polish `13_Panels_Desktop` panels 1–16 · v2** — parity с panels 17–33: AccentWrap, list rows 56px, avatars `profileAccent.*`, Voice placeholders, inset ≥16.
- [ ] **Visual fixes v2 (spot-check)** — починить баги из экспорта: Mute double-layer на `Panel / Chat / Info · v2`, clip/overflow где видно.

## Common

### Penpot

Penpot = active design tool ([penpot-setup.md](../design/penpot-setup.md)); правила макетов — [penpot-workflow.md](../design/penpot-workflow.md); Figma — legacy ([figma-setup.md](../design/figma-setup.md)). Runtime tokens stay in git.

- [x] **Penpot file rename** — UI name `Voice` (file ID `20d3f736-cc1b-8043-8008-561cb65228ef`).
- [x] **Penpot workflow doc** — [penpot-workflow.md](../design/penpot-workflow.md): clip/контейнеры, вертикальный канон + варианты по X, placeholder content.
- [x] **Penpot v2 spread (pages 11–15)** — эталон с `10_Screens_Desktop` (21 `· v2`) размножен на mobile screens, states, desktop/mobile panels, overlays. Итого **89** draft-фреймов `· v2` справа от канона; shipped snapshot не менялся.

- [ ] **Orphan cleanup `10_Screens_Desktop`** — удалить stray top-level boards: `Board`, `BlockerCard`, `SettingsNav` @ `x≈0, y≈0` (не канон, не `· v2`).
- [ ] **export_shape QA (pages 11–15)** — по 2–3 фрейма с каждой страницы; viewer URLs для PR (только draft `· v2`, не канон).

## Low

### Design ↔ code parity

- [ ] **Flutter chat/auth polish** — further list/room density vs Penpot (optional follow-up).
- [ ] **Flutter v2 parity (после approve)** — перенести утверждённые `· v2` в `src/frontend/lib/ui/`; shipped snapshot в Penpot обновить **только** скриптом заливки токенов (`make penpot-tokens-export`), не ручным merge в левый фрейм.


**Промпт-якорь:** `Design from docs/todo/design.md` + приоритет.
