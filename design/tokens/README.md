# Design tokens (canonical)

- **File:** `voice.tokens.json` (`dsVersion` inside JSON)
- **Penpot:** file ID `20d3f736-cc1b-8043-8008-561cb65228ef` — [docs/design/penpot-setup.md](../../docs/design/penpot-setup.md)
- **Flutter asset:** `src/frontend/assets/design/voice.tokens.json` — must match this file (`make design-tokens-check`)

## Update workflow

1. Edit semantic values here (Penpot `01_Foundation` mirrors this file; do not treat Penpot export as canonical).
2. Copy to Flutter asset:  
   `cp design/tokens/voice.tokens.json src/frontend/assets/design/voice.tokens.json`
3. Run `make design-tokens-check`.
4. PR must include both paths if visual constants change.
5. Re-sync Penpot: `make penpot-tokens-export > /tmp/voice-penpot-tokens.json` → import in Penpot Tokens panel, or push via Penpot MCP.

## Naming

- Theme colors: `color.background.canvas` (dots in JSON keys)
- Penpot tokens: `color.background.canvas` (dots) — same semantics as JSON keys

## Profile accent

`profileAccent.defaults` — seven pastel colors; index `n % 7` per profile order. User override: client storage until User Service exposes `accent_color`.
