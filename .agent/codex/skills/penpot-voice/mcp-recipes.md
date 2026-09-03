# Penpot MCP recipes (Voice)

Call via MCP server `penpot` (Cursor legacy: `user-penpot`), tool
`execute_code`, argument `code`. After edits, `export_shape` with the draft
`shapeId`.

Do not `console.log` values you also `return`.

## Timeout / 504 recovery

If a call times out or Penpot cloud returns 504:

1. Do not repeat the same heavy call.
2. In Penpot: reload/reopen the file if needed, then **File → MCP Server →
   Disconnect/Connect**. Keep the plugin window open.
3. If simple reads still hang, restart/reconnect the MCP client.
4. Resume with small read-only calls and the last known page/board ids.

Prefer local MCP + Firefox for long design sessions when Chromium/Edge keeps
failing on local network/plugin connections.

## Micro-query inspection protocol

Start with the cheapest possible read:

```javascript
return {
  hasCurrentFile: !!penpot.currentFile,
  fileName: penpot.currentFile ? penpot.currentFile.name : null,
  currentPageName: penpot.currentPage ? penpot.currentPage.name : null,
  currentPageId: penpot.currentPage ? penpot.currentPage.id : null,
};
```

Then list only top-level frames:

```javascript
const page = penpot.currentPage;
storage.pageId = page.id;
storage.frames = page.root.children.map((s) => ({
  id: s.id,
  name: s.name,
  type: s.type,
  x: Math.round(s.x),
  y: Math.round(s.y),
  w: Math.round(s.width),
  h: Math.round(s.height),
  childCount: s.children ? s.children.length : 0,
}));
return storage.frames;
```

After choosing one board id, inspect one shallow level at a time:

```javascript
const boardId = storage.boardId;
const board = penpot.currentPage.root.children.find((s) => s.id === boardId);
if (!board) throw new Error("board not found");
return {
  id: board.id,
  name: board.name,
  x: Math.round(board.x),
  y: Math.round(board.y),
  w: Math.round(board.width),
  h: Math.round(board.height),
  children: (board.children || []).slice(0, 20).map((s) => ({
    id: s.id,
    name: s.name,
    type: s.type,
    x: Math.round(s.x),
    y: Math.round(s.y),
    w: Math.round(s.width),
    h: Math.round(s.height),
    childCount: s.children ? s.children.length : 0,
  })),
};
```

Avoid full recursive walks of page/board trees, broad `findShape` scans, and
`penpotUtils.shapeStructure` on large boards unless a narrow subtree is already
selected.

## Switch page and list top-level frames

```javascript
const page = penpotUtils.getPageById("6d4c4410-c47e-8083-8008-561cf0765607");
await penpot.openPage(page);
storage.page = page;
storage.frames = page.root.children.map((s) => ({
  id: s.id,
  name: s.name,
  x: s.x,
  y: s.y,
  w: s.width,
  h: s.height,
}));
return storage.frames;
```

Find a frame (UI may insert spaces around `/`):

```javascript
const want = "Screen/Chat/List";
const shape = penpotUtils.findShape((s) => {
  const n = s.name.replace(/ /g, "");
  return n === want || s.name === want;
}, storage.page.root);
storage.target = shape;
return shape && { id: shape.id, name: shape.name, x: shape.x, y: shape.y, w: shape.width, h: shape.height };
```

## Duplicate canon to the right (draft)

Never mutate the shape at `x ≈ 0` without `·` in the name.

```javascript
const canon = storage.target;
if (!canon) throw new Error("no canon in storage.target");
const clone = canon.clone();
clone.name = canon.name.includes("·") ? canon.name : `${canon.name} · v2`;
clone.x = canon.x + canon.width + 80;
clone.y = canon.y;
storage.draft = clone;
return { id: clone.id, name: clone.name, x: clone.x, y: clone.y };
```

Then `export_shape` on `storage.draft.id`.

## Structure / clip debug

```javascript
return penpotUtils.shapeStructure(storage.draft, 4);
```

```javascript
const board = storage.draft;
const overflow = penpotUtils.analyzeDescendants(board, (root, shape) => {
  if (!penpotUtils.isContainedIn(shape, root)) {
    return { id: shape.id, name: shape.name, w: shape.width, h: shape.height };
  }
});
return overflow.map((o) => o.result);
```

After `appendChild` on a nested board:

```javascript
child.resize(parent.width, child.height);
```

## Library instance (not a new main)

```javascript
const comp = penpot.library.local.components.find(
  (c) => c.path === "Button" && c.name === "Primary"
);
if (!comp) throw new Error("component not found");
const inst = comp.instance();
storage.draft.appendChild(inst);
inst.resize(120, 40);
return { id: inst.id, name: inst.name };
```

Canonical widgets: `Button/*`, `Avatar/40`, `List/Row`, `ChatBubble/in|out`, `Nav/Item`, `Row/Friend|Game|Setting|Bot`, `Composer/Default`, `Input/Field`, `SearchBar/Default`, `AppBar/Default`, `TabBar/Default`, `Divider/Default`, `AccentWrap/Default`, `State/Empty`, `Banner/Offline`.

Lists: `List/Row` + child `Avatar/40`. Do not create a `Member` component.

Typography: `text.applyTypography(typo)` where `typo` is `penpot.library.local.typographies` named `type/body`, `type/title`, … — not `Text/*` library components.

## Apply a token

```javascript
const token = penpotUtils.findTokenByName("color.background.surface");
if (!token) throw new Error("token missing");
storage.draft.applyToken(token, ["fill"]);
return { name: token.name, resolved: token.resolvedValue };
```

Wait ~100 ms in a follow-up call before reading updated fills.

Sets (mirror of git): `Foundation/Layout`, `Foundation/Accent`, `Theme/Light`, `Theme/Dark`, `Theme/HighContrast`. Git remains canonical: `make penpot-tokens-export`.

## New component main (Foundation only)

1. `await penpot.openPage` → `01_Foundation`.
2. Build the shape inside `Foundation / Component Mains`.
3. `penpot.library.local.createComponent([shape])`; set `path` + `name` (`Button` / `Primary`).
4. Add an **instance** (not the main) to `Foundation / Components v2`.
5. On screen pages, only instances.

If a main is stuck on a screen page: recreate via SVG on Foundation (workflow §3.6). Do not `appendChild` the main across pages.

## Overlap check (top-level)

```javascript
const kids = storage.page.root.children;
const hits = [];
for (let i = 0; i < kids.length; i++) {
  for (let j = i + 1; j < kids.length; j++) {
    const a = kids[i], b = kids[j];
    const overlap =
      a.x < b.x + b.width && a.x + a.width > b.x &&
      a.y < b.y + b.height && a.y + a.height > b.y;
    if (overlap) hits.push([a.name, b.name]);
  }
}
return hits;
```

## export_shape

Tool `export_shape`: `shapeId` = draft frame id, `format` `png` (review) or `svg` (structure). Use `page` only for a zoomed-out overlap check — prefer a single frame.

## Inventory script (repo)

Canons without `·`: `scripts/design/generate-screens-md.mjs`. Do not put drafts into `docs/design/screens.md`.
