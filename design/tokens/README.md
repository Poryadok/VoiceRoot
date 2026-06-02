# Design tokens (canonical)

- **File:** `voice.tokens.json` (`dsVersion` inside JSON)
- **Figma:** https://www.figma.com/design/tIkNxn3e7vcp3APJ8I6bKi/Voice (`fileKey` `tIkNxn3e7vcp3APJ8I6bKi`)
- **Flutter asset:** `src/frontend/assets/design/voice.tokens.json` — must match this file (`make design-tokens-check`)

## Update workflow

1. Edit semantic values here (or export from Figma Variables on `01_Foundation` with matching names).
2. Copy to Flutter asset:  
   `cp design/tokens/voice.tokens.json src/frontend/assets/design/voice.tokens.json`
3. Run `make design-tokens-check`.
4. PR must include both paths if visual constants change.

## Naming

- Theme colors: `color.background.canvas` (dots in JSON keys)
- Figma variables: `color/background/canvas` (slashes) — same semantics

## Profile accent

`profileAccent.defaults` — seven pastel colors; index `n % 7` per profile order. User override: client storage until User Service exposes `accent_color`.
