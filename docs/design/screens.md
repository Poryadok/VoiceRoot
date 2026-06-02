# Screen inventory (Figma ↔ Flutter)

**fileKey:** `tIkNxn3e7vcp3APJ8I6bKi`  
**Base URL:** https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice

В задачи и PR — ссылка на **фрейм** (`node-id=…`), не на canvas страницы.

| Screen ID | Figma frame URL | Flutter / spec | Notes |
|-----------|-----------------|----------------|-------|
| `00_References/Discord19` | [node 0-1](https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice?node-id=0-1) | — | Референс layout only; не Phase-1 target |
| `Screen/TBD/FrameA` | [node 40-531](https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice?node-id=40-531) | TBD | Переименовать в Figma → `Screen/...` |
| `Screen/TBD/FrameB` | [node 40-532](https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice?node-id=40-532) | TBD | Переименовать в Figma → `Screen/...` |
| `Screen/Auth/Login` | — | [auth_screen.dart](../../src/frontend/lib/ui/auth/auth_screen.dart) | Phase 1 |
| `Screen/Shell/Desktop` | — | [three_column_shell.dart](../../src/frontend/lib/shell/three_column_shell.dart) | [navigation.md](../features/navigation.md) |
| `Screen/Chat/List` | — | [chat_list_panel.dart](../../src/frontend/lib/ui/chat/chat_list_panel.dart) | Phase 1 |
| `Screen/Chat/Room` | — | [chat_room_panel.dart](../../src/frontend/lib/ui/chat/chat_room_panel.dart) | Phase 1 |
| `Screen/Social/Panel` | — | [social_panel.dart](../../src/frontend/lib/ui/social/social_panel.dart) | Bottom sheet |

## Planned Figma pages

- `01_Foundation` — variables = [voice.tokens.json](../../design/tokens/voice.tokens.json)
- `10_Screens_Desktop` — 1280×800 frames
- `11_Screens_Mobile` — 390×844 frames
- `12_States` — empty / error / offline
