# Penpot file setup (Voice)

**Hosting:** [design.penpot.app](https://design.penpot.app) (cloud)  
**File ID:** `20d3f736-cc1b-8043-8008-561cb65228ef`  
**File name:** rename in UI to `Voice` when convenient (MCP may still show legacy name).

Open the file from your Penpot team/project dashboard. For share links to a **frame**, use Penpot **Share → Copy link** while the frame is selected; paste into [screens.md](screens.md).

**Layout, clip, placeholder content:** [penpot-workflow.md](penpot-workflow.md) — канон по вертикали, варианты по горизонтали; обязательно при добавлении или правке шаблонов.

## Pages in file (inventory)

| Page | Page ID | Purpose |
|------|---------|---------|
| `00_References` | `20d3f736-cc1b-8043-8008-561cb65228f0` | Discord / external refs; not core target |
| `01_Foundation` | `6d4c4410-c47e-8083-8008-561ce95f11e2` | Design tokens + swatches (`Foundation/Swatches`) + components (`Foundation/Components`) |
| `10_Screens_Desktop` | `6d4c4410-c47e-8083-8008-561cf0765607` | Frames **1280×800**, names `Screen/...` |
| `11_Screens_Mobile` | `6d4c4410-c47e-8083-8008-561cf5662204` | Frames **390×844** |
| `12_States` | `6d4c4410-c47e-8083-8008-561cfaa8677c` | empty / error / offline |
| `13_Panels_Desktop` | `6d4c4410-c47e-8083-8008-564229c3b00f` | `Panel/...` sheets and side panels over desktop shell (**1280×800** context) |
| `14_Panels_Mobile` | `6d4c4410-c47e-8083-8008-564229f6af85` | `Panel/...` as mobile bottom sheets (**390×844** context) |
| `15_Overlays` | `6d4c4410-c47e-8083-8008-56422a11288b` | `Overlay/...` call, matchmaking, onboarding, force-update |

In tasks and PRs — link to the **frame** (`Screen/...`, `Panel/...`, `Overlay/...`), not the page canvas. Frame inventory: [screens.md](screens.md).

Penpot may display `/` in frame names as spaces (e.g. `Screen / Shell / Desktop`). Inventory IDs use `Screen/...`; match by name in the UI.

## Design tokens (01_Foundation)

Canonical colors and layout values live in git: [design/tokens/voice.tokens.json](../../design/tokens/voice.tokens.json).

Penpot token sets (mirror only — **do not** treat Penpot export as runtime source):

| Set | Contents |
|-----|----------|
| `Foundation/Layout` | `space.*`, `radius.*`, `layout.*`, `type.*.*`, `stroke.*` |
| `Foundation/Accent` | `profileAccent.0` … `profileAccent.6` |
| `Theme/Light` | semantic colors for light mode |
| `Theme/Dark` | semantic colors for dark mode |
| `Theme/HighContrast` | semantic colors for high contrast |

**Themes (axis `Mode`):** Light (default), Dark, HighContrast — each activates the matching `Theme/*` set. `Foundation/*` sets stay enabled.

Re-sync after editing `voice.tokens.json`:

```bash
make penpot-tokens-export > /tmp/voice-penpot-tokens.json
# Penpot: Tokens panel → Tools → Import
```

Or push via Penpot MCP (`execute_code` / agent).

## Cursor + Penpot MCP

1. Penpot → **Account → Integrations → MCP Server** → generate key.
2. Cursor → **Settings → Tools & MCP** → add server (`url` with `userToken=…`). See [help.penpot.app/mcp](https://help.penpot.app/mcp).
3. Open this file in the browser → **File → MCP Server → Connect** (plugin must stay connected while the agent reads/edits designs).
4. **Do not** commit MCP keys to the repo.

Agent tools: `execute_code`, `export_shape`, `penpot_api_info`. Prefer reading frames before Flutter UI changes.

## Workflow

1. Product behavior — `docs/features/`, [brand.md](brand.md).
2. Visual constants — edit [voice.tokens.json](../../design/tokens/voice.tokens.json) → copy to Flutter asset → `make design-tokens-check` → re-sync Penpot tokens.
3. Layout — update Penpot frame → link in PR → implement in Flutter with `lib/ui/core/*` and `VoiceTheme`.

## Legacy Figma

Historical Figma file and node IDs: [figma-setup.md](figma-setup.md) (archive only).
