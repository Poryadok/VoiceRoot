# Figma file setup (Voice)

**fileKey:** `tIkNxn3e7vcp3APJ8I6bKi`  
**URL:** https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice

## Pages (create or rename)

| Page | Purpose |
|------|---------|
| `00_References` | Move «Discord 19» import here |
| `01_Foundation` | Variables synced with [voice.tokens.json](../../design/tokens/voice.tokens.json) |
| `10_Screens_Desktop` | Frames 1280×800, names `Screen/...` |
| `11_Screens_Mobile` | Frames 390×844 |
| `12_States` | empty / error / offline |

## Variables (01_Foundation)

Mirror JSON keys:

- Neutral: `color/background/canvas`, `surface`, `elevated`, `muted`, `text/primary`, …
- Modes: Light, Dark, High contrast
- Accent samples: `accent/profile-0` … `accent/profile-6` (hex from `profileAccent.defaults`)

## Agent / MCP

- OAuth in Cursor (not PAT in MCP config).
- Prefer `use_figma` for structure; avoid `get_metadata` on page `0:1` (Starter read limits).
- Track frames in [screens.md](screens.md) with **frame** URLs.
