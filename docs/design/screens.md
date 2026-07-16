# Screen inventory (Penpot ↔ Flutter)

**File ID:** `20d3f736-cc1b-8043-8008-561cb65228ef`  
**Setup:** [penpot-setup.md](penpot-setup.md)

Viewer URL pattern: `https://design.penpot.app/#/viewer/{fileId}/{pageId}/{frameId}`

В задачи и PR — **viewer URL** фрейма или frame ID. Для новых экранов UX — [brand.md](brand.md).

| Screen ID | Penpot frame ID | Viewer (desktop / mobile / states) | Flutter / spec | Notes |
|-----------|-----------------|-------------------------------------|----------------|-------|
| `Screen/Auth/Login` | `6d4c4410-c47e-8083-8008-5624d33db6de` | [desktop](https://design.penpot.app/#/viewer/20d3f736-cc1b-8043-8008-561cb65228ef/6d4c4410-c47e-8083-8008-561cf0765607/6d4c4410-c47e-8083-8008-5624d33db6de) · [mobile](https://design.penpot.app/#/viewer/20d3f736-cc1b-8043-8008-561cb65228ef/6d4c4410-c47e-8083-8008-561cf5662204/6d4c4410-c47e-8083-8008-5625373991bf) | [auth_screen.dart](../../src/frontend/lib/ui/auth/auth_screen.dart) | app stack |
| `Screen/Shell/Desktop` | `6d4c4410-c47e-8083-8008-56225eaec295` | [desktop](https://design.penpot.app/#/viewer/20d3f736-cc1b-8043-8008-561cb65228ef/6d4c4410-c47e-8083-8008-561cf0765607/6d4c4410-c47e-8083-8008-56225eaec295) | [three_column_shell.dart](../../src/frontend/lib/shell/three_column_shell.dart) | [navigation.md](../features/navigation.md) |
| `Screen/Chat/List` | `6d4c4410-c47e-8083-8008-5624e3edbd34` | [desktop](https://design.penpot.app/#/viewer/20d3f736-cc1b-8043-8008-561cb65228ef/6d4c4410-c47e-8083-8008-561cf0765607/6d4c4410-c47e-8083-8008-5624e3edbd34) | [chat_list_panel.dart](../../src/frontend/lib/ui/chat/chat_list_panel.dart) | wireframe column |
| `Screen/Chat/Room` | `6d4c4410-c47e-8083-8008-5624ea01ef06` | [desktop](https://design.penpot.app/#/viewer/20d3f736-cc1b-8043-8008-561cb65228ef/6d4c4410-c47e-8083-8008-561cf0765607/6d4c4410-c47e-8083-8008-5624ea01ef06) | [chat_room_panel.dart](../../src/frontend/lib/ui/chat/chat_room_panel.dart) | main column |
| `Screen/Social/Panel` | `6d4c4410-c47e-8083-8008-5624f0114d16` | [desktop](https://design.penpot.app/#/viewer/20d3f736-cc1b-8043-8008-561cb65228ef/6d4c4410-c47e-8083-8008-561cf0765607/6d4c4410-c47e-8083-8008-5624f0114d16) | [social_panel.dart](../../src/frontend/lib/ui/social/social_panel.dart) | side sheet |
| `Screen/Shell/Mobile` | `6d4c4410-c47e-8083-8008-5625434e4f58` | [mobile](https://design.penpot.app/#/viewer/20d3f736-cc1b-8043-8008-561cb65228ef/6d4c4410-c47e-8083-8008-561cf5662204/6d4c4410-c47e-8083-8008-5625434e4f58) | [three_column_shell.dart](../../src/frontend/lib/shell/three_column_shell.dart) | 390×844 |
| `Screen/Shell/MobileChatOpen` | `6d4c4410-c47e-8083-8008-56254482152e` | [mobile](https://design.penpot.app/#/viewer/20d3f736-cc1b-8043-8008-561cb65228ef/6d4c4410-c47e-8083-8008-561cf5662204/6d4c4410-c47e-8083-8008-56254482152e) | [three_column_shell.dart](../../src/frontend/lib/shell/three_column_shell.dart) | strip + chat |
| `State/Chat/Empty` | `6d4c4410-c47e-8083-8008-56251e01bc50` | [states](https://design.penpot.app/#/viewer/20d3f736-cc1b-8043-8008-561cb65228ef/6d4c4410-c47e-8083-8008-561cfaa8677c/6d4c4410-c47e-8083-8008-56251e01bc50) | [voice_state_panel.dart](../../src/frontend/lib/ui/core/voice_state_panel.dart) | `12_States` |
| `State/Chat/Error` | `6d4c4410-c47e-8083-8008-56251f588202` | [states](https://design.penpot.app/#/viewer/20d3f736-cc1b-8043-8008-561cb65228ef/6d4c4410-c47e-8083-8008-561cfaa8677c/6d4c4410-c47e-8083-8008-56251f588202) | [voice_state_panel.dart](../../src/frontend/lib/ui/core/voice_state_panel.dart) | `12_States` |
| `State/Network/Offline` | `6d4c4410-c47e-8083-8008-5625214e0649` | [states](https://design.penpot.app/#/viewer/20d3f736-cc1b-8043-8008-561cb65228ef/6d4c4410-c47e-8083-8008-561cfaa8677c/6d4c4410-c47e-8083-8008-5625214e0649) | [voice_compact_banner.dart](../../src/frontend/lib/ui/core/voice_compact_banner.dart) | `12_States` |

## Penpot pages

| Page | Page ID |
|------|---------|
| `00_References` | `20d3f736-cc1b-8043-8008-561cb65228f0` |
| `01_Foundation` | `6d4c4410-c47e-8083-8008-561ce95f11e2` |
| `10_Screens_Desktop` | `6d4c4410-c47e-8083-8008-561cf0765607` |
| `11_Screens_Mobile` | `6d4c4410-c47e-8083-8008-561cf5662204` |
| `12_States` | `6d4c4410-c47e-8083-8008-561cfaa8677c` |

## Legacy Figma

Historical inventory: [figma-setup.md](figma-setup.md) (archive).
