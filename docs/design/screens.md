# Screen inventory (Figma ↔ Flutter)

**fileKey:** `tIkNxn3e7vcp3APJ8I6bKi`  
**Base URL:** https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice

В задачи и PR — ссылка на **фрейм** (`node-id=…`), не на canvas страницы. Canvas всех страниц файла — [figma-setup.md](figma-setup.md#pages-in-file-inventory). Для новых и обновляемых экранов UX-проверка идёт через [brand.md](brand.md): Telegram-first для базового messaging, Discord-like только для voice/community там, где Telegram не даёт паттерна.

| Screen ID | Figma frame URL | Flutter / spec | Notes |
|-----------|-----------------|----------------|-------|
| `Screen/Auth/Login` | — | [auth_screen.dart](../../src/frontend/lib/ui/auth/auth_screen.dart) | app stack |
| `Screen/Shell/Desktop` | — | [three_column_shell.dart](../../src/frontend/lib/shell/three_column_shell.dart) | [navigation.md](../features/navigation.md) |
| `Screen/Chat/List` | — | [chat_list_panel.dart](../../src/frontend/lib/ui/chat/chat_list_panel.dart) | app stack |
| `Screen/Chat/Room` | — | [chat_room_panel.dart](../../src/frontend/lib/ui/chat/chat_room_panel.dart) | app stack |
| `Screen/Social/Panel` | — | [social_panel.dart](../../src/frontend/lib/ui/social/social_panel.dart) | Bottom sheet |

## Figma pages

Целевые имена и текущие canvas URL — [figma-setup.md](figma-setup.md).
