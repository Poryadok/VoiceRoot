# Design tokens semantics

Канон: [design/tokens/voice.tokens.json](../../design/tokens/voice.tokens.json). Runtime: [src/frontend/assets/design/voice.tokens.json](../../src/frontend/assets/design/voice.tokens.json).

## Modes (neutral)

| Mode | Use |
|------|-----|
| `light` | Светлая тема |
| `dark` | Тёмная тема (default в User Service позже) |
| `highContrast` | Доступность — [accessibility.md](../features/accessibility.md) |

### Color (per mode)

| Token | Role |
|-------|------|
| `color.background.canvas` | Корневой фон приложения |
| `color.background.surface` | Колонки, панели |
| `color.background.elevated` | Приподнятые блоки |
| `color.text.primary` | Основной текст |
| `color.text.secondary` | Вторичный текст |
| `color.text.disabled` | Неактивный текст |
| `color.border.default` | Разделители |
| `color.border.strong` | Акцентные границы (high contrast) |
| `color.semantic.error` | Ошибки |
| `color.focus.ring` | Focus indicator |

## Profile accent (not per mode)

| Token | Role |
|-------|------|
| `profileAccent.defaults[]` | Дефолтный hex по индексу профиля (0..6, then wrap) |
| Runtime override | SharedPreferences per `profile_id` until User Service `accent_color` |

Accent применяется к: `ColorScheme.primary`, primary `FilledButton`, send CTA, индикатор активного профиля.

## Layout tokens (global)

| Token | Default |
|-------|---------|
| `space.4` … `space.32` | 4px grid |
| `radius.sm` | 4px (default controls) |
| `radius.md` | 6px |
| `radius.lg` | 8px (avatars) |

## Flutter mapping

- `VoiceTokenCatalog` — load JSON
- `VoiceColors` — `ThemeExtension`, neutral + `profileAccent`
- `VoiceTheme.light/dark/highContrast(profileAccent: Color)`

Имена в Penpot tokens (и legacy Figma Variables) совпадают с путями в JSON: `color.background.canvas` (точки в имени токена).
