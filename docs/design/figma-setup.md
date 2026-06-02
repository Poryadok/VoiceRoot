# Figma file setup (Voice)

**fileKey:** `tIkNxn3e7vcp3APJ8I6bKi`  
**URL:** https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice

> **Temporary status:** Figma сейчас недоступна. До восстановления доступа не использовать Figma/MCP как обязательный шаг; дизайн и UI-решения принимаются по [brand.md](brand.md), [tokens.md](tokens.md), `design/tokens/voice.tokens.json`, `docs/features/` и текущей реализации Flutter.

## Pages in file (inventory)

Сейчас в [Voice](https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice) **5 страниц** (canvas). В задачи — ссылка на **фрейм** `Screen/...` из [screens.md](screens.md); canvas страницы — только для навигации и переименования.

| Page | node-id | Canvas URL | Purpose |
|------|---------|------------|---------|
| `00_References` | `0:1` | https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice?node-id=0-1 | «Discord 19» import; не Phase-1 target |
| `01_Foundation` | `40:531` | https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice?node-id=40-531 | Variables = [voice.tokens.json](../../design/tokens/voice.tokens.json) |
| `10_Screens_Desktop` | `40:532` | https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice?node-id=40-532 | Frames 1280×800, names `Screen/...` |
| `11_Screens_Mobile` | `47:533` | https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice?node-id=47-533 | Frames 390×844 |
| `12_States` | `47:534` | https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice?node-id=47-534 | empty / error / offline |

## Variables (01_Foundation)

Mirror JSON keys:

- Neutral: `color/background/canvas`, `surface`, `elevated`, `muted`, `text/primary`, …
- Modes: Light, Dark, High contrast
- Accent samples: `accent/profile-0` … `accent/profile-6` (hex from `profileAccent.defaults`)

## Agent / MCP

- OAuth in Cursor (not PAT in MCP config).
- Prefer `use_figma` for structure; avoid `get_metadata` on page `0:1` (Starter read limits).
- Track frames in [screens.md](screens.md) with **frame** URLs.
