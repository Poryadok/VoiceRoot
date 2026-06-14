# Voice Moderation Admin (Phase 14)

Internal React admin for platform moderators. Calls Gateway staff routes under `/api/v1/admin/moderation/**`.

## Setup

```bash
cd src/admin
npm install
cp .env.example .env
# Set VITE_VOICE_API_BASE and VITE_STAFF_TOKEN (staff JWT)
npm run dev
```

## Scripts

| Script | Description |
|--------|-------------|
| `npm run dev` | Vite dev server |
| `npm run build` | Production build |
| `npm run test` | Vitest + React Testing Library |

## Environment

| Variable | Purpose |
|----------|---------|
| `VITE_VOICE_API_BASE` | Gateway base URL (e.g. `http://localhost:8080`) |
| `VITE_STAFF_TOKEN` | Bearer JWT with `staff` role; `profile_id` claim used for assign-to-me |

Design tokens: CSS variables in `src/styles/tokens.css` mirror `design/tokens/voice.tokens.json`.
