---
name: penpot-voice
description: >-
  Uses Penpot MCP against the Voice design file: connect, inspect frames,
  duplicate drafts to the right of shipped snapshots, edit via execute_code,
  export_shape QA, tokens re-sync. Use when the user mentions Penpot, макет,
  фрейм, MCP, export_shape, design.penpot.app, screens.md, · v2, or UI mock work.
---

# Penpot for Voice

Read this file completely before any Penpot MCP call. Detailed snippets: [mcp-recipes.md](mcp-recipes.md). Layout/clip/placeholder canon: `docs/design/penpot-workflow.md` — follow it, do not paraphrase a weaker subset.

## Preconditions

1. Voice file open in the browser: [design.penpot.app](https://design.penpot.app) — file ID `20d3f736-cc1b-8043-8008-561cb65228ef`.
2. **File → MCP Server → Connect** stays on for the whole session. If tools fail with auth/plugin errors, stop and tell the user to Connect; do not guess geometry.
3. MCP server id: `user-penpot`. Before the first call: `GetMcpTools` on that server. Then `CallMcpTool`.
4. Once per conversation, call `high_level_overview` **before** `execute_code` / `penpot_api_info` (skip if already read in this chat).
5. Never commit MCP keys. Setup: `docs/design/penpot-setup.md`.

PowerShell: chain with `;`, not `&&`.

## File map

| Page | Page ID | Size / names |
|------|---------|--------------|
| `00_References` | `20d3f736-cc1b-8043-8008-561cb65228f0` | refs only |
| `01_Foundation` | `6d4c4410-c47e-8083-8008-561ce95f11e2` | tokens, **all** component mains, showcase |
| `10_Screens_Desktop` | `6d4c4410-c47e-8083-8008-561cf0765607` | 1280×800 `Screen/...` |
| `11_Screens_Mobile` | `6d4c4410-c47e-8083-8008-561cf5662204` | 390×844 `Screen/...` |
| `12_States` | `6d4c4410-c47e-8083-8008-561cfaa8677c` | empty/error/offline |
| `13_Panels_Desktop` | `6d4c4410-c47e-8083-8008-564229c3b00f` | 1280×800 `Panel/...` |
| `14_Panels_Mobile` | `6d4c4410-c47e-8083-8008-564229f6af85` | 390×844 `Panel/...` |
| `15_Overlays` | `6d4c4410-c47e-8083-8008-56422a11288b` | `Overlay/...` |

Inventory (canons only): `docs/design/screens.md`. Viewer: `https://design.penpot.app/#/viewer/{fileId}/{pageId}/{frameId}`.

Penpot UI may show `Screen / Chat / List`; inventory uses `Screen/Chat/List`. Match both.

## Tools

| Tool | Use |
|------|-----|
| `high_level_overview` | Once per chat, before other Penpot tools |
| `penpot_api_info` | Type/member docs (`Penpot`, `Board`, `LibraryComponent`, …) when a call is unclear |
| `execute_code` | Inspect and mutate. JS in plugin context: `penpot`, `penpotUtils`, `storage` |
| `export_shape` | PNG/SVG of a shape id (`selection` / `page` allowed). **QA after every edit** |

`execute_code` rules:

- Return data; **never** `console.log` the same value you `return` (duplicated payload).
- Persist helpers and ids on `storage` across calls.
- `width`/`height` are read-only → `resize(w, h)`. After `appendChild`, always `resize` nested boards (default 100×100 causes empty clipped frames).
- `parentX`/`parentY` are read-only → `penpotUtils.setParentXY(shape, x, y)`.
- Prefer `penpotUtils.findShape` / `findShapes` / `shapeStructure` / `getPageById` over hand-rolled walks.
- Mutations need the **active** page: `await penpot.openPage(page)` then change shapes. Cross-page `appendChild` for component mains **does not work**.
- Penpot cloud 504: **one** `createComponent` / `swapComponent` / `remove` of a main per `execute_code`.

## Hard rules (Voice)

1. **Do not edit shipped snapshot** (`x ≈ 0`, name without `·`). No MCP writes, no “small color tweak”. Token fill of the canon is `make penpot-tokens-export` only.
2. Design work = **clone to the right** on the same Y, suffix `· v2` / `· draft` / `· WIP`. Gap ≥ 80 px on X, ≥ 100 px on Y. No overlapping top-level frames.
3. New scenario = **new row below**, left slot is canon (script/snapshot), draft on the right.
4. Colors from `design/tokens/voice.tokens.json` via Penpot token sets / `applyToken`. Do not invent hex.
5. Component **mains** only on `01_Foundation` → `Foundation / Component Mains`. Screen pages: instances. Recreate procedure: workflow §3.6.
6. Placeholder: 2–3 realistic rows, Voice names, timestamps, avatars `profileAccent.*`. No empty `Board` / layer named `Text` with content `Text`.
7. Inset ≥ 16 px for text/controls. Active nav row: `AccentWrap` flush left only (workflow §3.5).
8. Do not update `screens.md` for draft names with `·`. Canons only.

## Workflow

```
Task Progress:
- [ ] Spec: docs/features/… + brand.md + screen-controls.md
- [ ] Locate canon in screens.md (or confirm missing)
- [ ] MCP Connect + high_level_overview
- [ ] openPage + list top-level frames (x,y,w,h,name)
- [ ] Clone canon → right as · v2 (if editing)
- [ ] Build with library instances + type.* typography
- [ ] resize containers; clip off unless size matches content
- [ ] export_shape — readable without hover
- [ ] Checklist in penpot-workflow.md §5
```

Tokens path (git → Penpot, never the reverse as source of truth):

```text
make design-tokens-check
make penpot-tokens-export
```

Then Tokens panel import, or push sets via MCP (see recipes). Script: `scripts/design/voice-tokens-to-penpot.py`.

## Inspect then edit

Typical first `execute_code` (store page + frames on `storage`): list pages, `openPage`, map `page.root.children` to `{id,name,x,y,w,h}`. Identify the canon (`x` near 0, no `·`) and any existing draft.

Then: clone, rename, shift `x`, edit **only** the clone. Finish with `export_shape` on the draft id.

If the plugin is disconnected or `penpot.currentFile` is null — stop.

## Flutter handoff (when asked)

Penpot is the mock. Implementation uses `VoiceColors` / `lib/ui/core/*` (`.cursor/rules/voice-design.mdc`). Do not copy MCP hex into Dart.

## Related

- Designer agent: `.cursor/agents/voice-designer.md`
- Setup / pages: `docs/design/penpot-setup.md`
- Layout rules: `docs/design/penpot-workflow.md`
- Index: `docs/design/README.md`
- Open design tasks: `docs/todo/design.md`
