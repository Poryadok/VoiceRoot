# Voice Admin Panel

Internal React admin for platform staff: moderation queue, appeals, game catalog, and product analytics. Calls Gateway staff routes under `/api/v1/admin/**` and staff-gated analytics under `/api/v1/analytics/**`.

## Setup

```bash
cd src/admin
npm install
cp .env.example .env
# Set VITE_VOICE_API_BASE; staff JWT via OAuth or VITE_STAFF_TOKEN when VITE_OAUTH_DISABLED=true
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
| `VITE_OAUTH_CLIENT_ID` | OAuth client id (`voice-admin` in staging/prod) |
| `VITE_OAUTH_DISABLED` | When `true`, skip OAuth and use paste-JWT login |
| `VITE_STAFF_TOKEN` | Bearer JWT with staff role (dev-only when OAuth disabled) |

## Routes

| Path | Feature |
|------|---------|
| `/queue` | Moderation report queue |
| `/appeals` | Sanction appeals review |
| `/game-requests` | User-submitted game catalog requests |
| `/games/new` | Staff CreateGame form (with catalog dedup search) |
| `/analytics/*` | Product, engagement, revenue, health, moderation, retention, funnels, export |
| `/audit` | Gateway analytics export audit log |

Design tokens: CSS variables in `src/styles/tokens.css` mirror `design/tokens/voice.tokens.json`.
