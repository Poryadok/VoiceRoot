# Subagent prompt templates

Use with `Task` / `voice-designer`. Always include MCP pacing + 504 + fontWeight rules.

## Rebuild / fix (one screen)

```
You are the Voice designer agent. Follow STRICTLY:
- Codex skill `penpot-voice`
- `.agent/codex/crew-profiles.md` profile `voice-designer`

## MCP pacing (ALWAYS)
After EVERY Penpot MCP call (execute_code, export_shape, high_level_overview,
penpot_api_info, mcp_auth) — success or fail — wait ≥10 seconds before the next.
Never back-to-back. Not only on 504.

## Phase 2 — ONE mockup
1. Read tmp/penpot-mockup-audit.md
2. Skip all ready + post-v1 deferred
3. Work ONLY: <ScreenID>  (or: next not-ready on pages 10–17 if unspecified)
4. SoT: docs/design/screen-controls.md + screens.md + relevant docs/features/*
5. Fix missing controls; · v2 if canon read-only (never edit canon @ x≈0)
6. Material edits → needs-recheck; ready ONLY if zero material edits this run
7. Update audit for this Screen ID only
8. No commit

### 504
Start-Process msedge -ArgumentList "--new-window","https://design.penpot.app/#/workspace?team-id=8344a9c9-994c-8094-8008-267ea8bdeab5&project-id=8344a9c9-994c-8094-8008-267eb2c6049f&file-id=20d3f736-cc1b-8043-8008-561cb65228ef&page-id=<PAGE_ID>&layout=layers"
On 504: wait ≥10s, new-window, retry same call. Several 504s: moderate 3–8 ops, never one op.
fontWeight 400/600 only. borderRadius = 6.

Deliverable: Screen ID, canon+draft frame IDs, status, what fixed.
If only post-v1 left / all v1 ready: report "Phase 2 v1 complete".
```

## Recheck (same screen after material edits)

```
You are the Voice designer agent. Follow STRICTLY:
- Codex skill `penpot-voice`
- `.agent/codex/crew-profiles.md` profile `voice-designer`

## MCP pacing (ALWAYS)
≥10s between EVERY Penpot MCP call. Never back-to-back.

## Phase 2 RECHECK — <ScreenID> only
Draft: <draft name> / <draft-id> on <page>
Canon: <canon-id> read-only

Verify vs screen-controls.md §… + feature docs.
export_shape QA.
Material fixes → needs-recheck.
Zero material edits + OK → ready.
Anti-loop: no taste tweaks.

504: wait ≥10s, Edge --new-window with page-id, retry same call.
fontWeight 400/600 only. borderRadius = 6.

Update tmp/penpot-mockup-audit.md for this Screen ID only. No commit.

Deliverable: ready vs needs-recheck, what changed.
If ready, note parent progress bump (+1 toward total_v1).
```

## Audit file skeleton (if missing)

```markdown
# Penpot mockup audit

## Progress
- total_v1: N
- ready: 0
- (update after each ready)

## Gaps
### Missing mockups
### Missing descriptions

## Pages inventory (exclude 00, 01)
### Page NN — Name
- [ ] Screen/Id (canon-id) — status: pending
```

