---
name: penpot-mockup-audit
description: >-
  Coordinates Phase 2 Voice Penpot mockup audit: docs↔frames gap check,
  one-screen-at-a-time rebuild/recheck via voice-designer, audit file tracking,
  MCP pacing and 504 recovery. Use when the user asks to audit mockups, пройти
  макеты, penpot-mockup-audit, tmp/penpot-mockup-audit.md, Phase 2 screens,
  or batch-verify buttons/controls against screen-controls.md.
---

# Penpot mockup audit (Voice)

Coordinator skill for batch QA/fix of Voice Penpot frames against docs.
Low-level Penpot MCP: follow **`penpot-voice`** strictly (read that skill first).
Designer agent: `.cursor/agents/voice-designer.md`.

## Scope terms (do not confuse)

| Term | Meaning |
|------|---------|
| **Product v1** | Screens on Penpot pages **10–17** (exclude 00/01). Progress denominator. |
| **Post-v1** | Deferred / `needs-user-docs` — **skip** unless user expands scope. |
| **Canon** | Read-only shipped frame @ `x≈0`, name without `·`. Never edit. |
| **`· v2` draft** | Editable clone @ `x≈1360`, `gapX≥80`. Ship surface when canon is RO. |

«Phase 2 v1 complete» = all **product-v1** Screen IDs are `ready`, not “we only looked at layers named v1”.

## Sources of truth

| Need | Path |
|------|------|
| Controls per screen | `docs/design/screen-controls.md` |
| Screen IDs / page placement | `docs/design/screens.md` |
| Feature behavior | `docs/features/*.md` |
| Tracker (tmp, no commit unless asked) | `tmp/penpot-mockup-audit.md` |

Do not invent product behavior. Missing docs → `needs-user-docs` + ask user.

## Audit statuses

- `pending` — not done
- `needs-recheck` — **material** edits this run → must re-QA
- `ready` — separate pass with **zero material edits** + `export_shape` OK
- `needs-user-docs` — blocked on specs
- `MCP-blocked` — Penpot down; never mark `ready` without QA

**Material** = missing control, broken layout/tokens/overflow/unreadability, wrong happy-path.
**Anti-loop:** no taste tweaks. Plain rect/text OK if library Toggle/SVG is invisible.

## Progress reporting

Once per session: set `total_v1` = ready + needs-recheck + pending (exclude post-v1).
After each Screen ID becomes `ready`, tell the user **`ready/total_v1`** (e.g. `62/66`).
Increment +1 — do not re-scan the whole audit every time.

## Coordinator loop

```
Task Progress:
- [ ] Read/create tmp/penpot-mockup-audit.md; fix total_v1
- [ ] Skip ready + post-v1
- [ ] Pick ONE next not-ready Screen ID
- [ ] Spawn voice-designer: rebuild/fix (see subagent-prompts.md)
- [ ] If needs-recheck → spawn RECHECK on same Screen ID
- [ ] Update audit for touched IDs only
- [ ] Report N/M after each ready
- [ ] Stop when all v1 ready → “Phase 2 v1 complete (M/M)”
```

Usually **one** designer agent at a time (MCP rate limits). No git commit unless user asks.

## MCP pacing (ALWAYS)

After **every** Penpot MCP call (`execute_code`, `export_shape`, `high_level_overview`, `penpot_api_info`, `mcp_auth`) — success or fail — wait **≥10 seconds** before the next. Never back-to-back. Not only on 504.

Pass this rule into every designer subagent prompt.

## 504 recovery

1. Wait ≥10s  
2. New Edge window (substitute `page-id`):

```
Start-Process msedge -ArgumentList "--new-window","https://design.penpot.app/#/workspace?team-id=8344a9c9-994c-8094-8008-267ea8bdeab5&project-id=8344a9c9-994c-8094-8008-267eb2c6049f&file-id=20d3f736-cc1b-8043-8008-561cb65228ef&page-id=<PAGE_ID>&layout=layers"
```

3. Retry the **same** call  

No 120s idle wait. Do not shrink batches to 1 op. Several 504s → moderate chunks of **3–8** ops. Persistent failure → mark `MCP-blocked`, tell user **File → MCP Server → Connect**; optionally skip to next screen.

Page IDs: `penpot-voice` File map (`13_Panels_Desktop`, `15_Overlays_Desktop`, …).

## Design hard constraints (remind designer)

- `fontWeight` **400 / 600 only** (500/700 break MCP)
- `borderRadius = 6`
- Tokens from project; inset ≥16; Discord/Telegram-quality placement for missing controls
- Happy-path only; off-frame states stay off-frame unless required
- `export_shape` after edits; strip junk (`now`, `12:34`)

## Done

**Phase 2 v1 complete** when every product-v1 Screen ID is `ready`. Report `M/M` and list post-v1 still out of scope.

## Additional resources

- Subagent prompt templates: [subagent-prompts.md](subagent-prompts.md)
- Penpot MCP details: `.cursor/skills/penpot-voice/SKILL.md`
