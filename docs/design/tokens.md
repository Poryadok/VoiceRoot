# Design tokens semantics

Канон: [design/tokens/voice.tokens.json](../../design/tokens/voice.tokens.json) (`dsVersion` 0.2.0). Runtime: [src/frontend/assets/design/voice.tokens.json](../../src/frontend/assets/design/voice.tokens.json).

**Стиль:** messaging surface ближе к Telegram / soft messenger (bubbles, pill search, тонкий app rail); **функция** shell для spaces — Discord-like (tree, members, voice), но без визуального шума. См. [brand.md](brand.md).

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
| `color.background.muted` | Приглушённые зоны |
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

## Space

| Token | Default | Typical use |
|-------|--------:|-------------|
| `space.2` | 2 | same-author message gap |
| `space.4` … `space.16` | 4–16 | 4px grid gutters, bubble padding (`12`) |
| `space.20` | 20 | info-panel section gaps |
| `space.24` … `space.40` | 24–40 | крупные вертикальные паузы |

## Radius

| Token | Default | Use |
|-------|--------:|-----|
| `radius.sm` | 4 | channel highlight, menus, compact chrome |
| `radius.md` | 6 | inputs, compact controls |
| `radius.lg` | 8 | composer shell, soft cards |
| `radius.xl` | 12 | quote blocks, larger soft controls |
| `radius.bubble` | 16 | message bubbles (no tails) |
| `radius.pill` | 999 | search field, unread badge, toggles |

Avatars — круг (`BorderRadius.circular(size/2)`), не отдельный numeric token.

## Layout

| Token | Default | Use |
|-------|--------:|-----|
| `layout.railWidth` | 56 | app rail (не Discord guild rail 72) |
| `layout.listWidth` | 320 | DM list / space tree |
| `layout.panelWidth` | 320 | members / info / stickers panel |
| `layout.headerHeight` | 56 | column headers |
| `layout.composerMinHeight` | 52 | composer bar |
| `layout.listRowHeight` | 64 | DM chat list row |
| `layout.channelRowHeight` | 34 | space channel tree row |
| `layout.avatarSm` / `Md` / `Lg` | 32 / 40 / 80 | list / message / profile |
| `layout.iconSm` / `iconMd` | 16 / 20 | UI icons |
| `layout.messageGutter` | 48 | avatar column in message history |
| `layout.bubbleMaxWidthFraction` | 0.72 | max bubble width vs chat column |

## Type

| Token | size / weight / lineHeight | Role |
|-------|----------------------------|------|
| `type.display` | 20 / 600 / 28 | rare titles |
| `type.title` | 16 / 600 / 22 | chat/space header name |
| `type.body` | 15 / 400 / 22 | message text |
| `type.bodyStrong` | 15 / 500 / 22 | sender name in stream |
| `type.label` | 14 / 500 / 20 | list title, channel name |
| `type.subtitle` | 13 / 400 / 18 | list preview |
| `type.caption` | 12 / 400 / 16 | timestamp, status, date text |
| `type.overline` | 11 / 500 / 14 (+ letterSpacing 0.6) | ALL CAPS category labels |

## Stroke

| Token | Default | Use |
|-------|--------:|-----|
| `stroke.hairline` | 1 | dividers |
| `stroke.strong` | 2 | high contrast / emphasis |

## Flutter mapping

- `VoiceTokenCatalog` — load JSON (`space`, `radius`, `layout`, `type`, `stroke`, themes)
- `VoiceMetrics` — `ThemeExtension` for space/radius/layout/stroke/type
- `VoiceColors` — neutral + `profileAccent`
- `VoiceTheme.light/dark/highContrast(profileAccent: Color)`

Имена в Penpot tokens совпадают с путями: `layout.railWidth`, `type.body.size`, `radius.bubble`.
