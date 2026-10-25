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
| `npm run test:browser` | Build actual Admin and run real Chromium staff journeys against a local HTTP fixture |

Browser characterization uses Node 24 built-ins and `CHROME_BIN` (default
`google-chrome`). Set `GITHUB_SHA` to the full source SHA and run
`npm run test:browser`. The runner reserves a disposable loopback port, builds
with that absolute API origin, OAuth enabled, no static staff token and client
ID `voice-admin`, then serves the built SPA on loopback
and validates actual OAuth S256, bearer, moderation and appeal HTTP requests.
Trusted browser input covers anonymous entry, invalid OAuth state, login,
Content/Spaces separation, staff assignment, denied resolution with retained
input, destructive sanction cancel/confirm, resolution retry, appeal approval,
and logout/direct reload. Unexpected requests or skipped assertions fail.
The command first runs the no-listener DOM readiness regression. A newly
navigated document without a body remains pending in the existing bounded
text wait; unexpected script errors still fail. Run that regression alone
with `node --test browser/text-readiness.node.mjs`.
JSON and PNG evidence is written to `../../tmp/admin-browser` (override with
`ADMIN_BROWSER_OUTPUT`), including source SHA and Chromium version. The
`Admin browser` workflow runs this suite for Admin changes. This fixture is
browser integration evidence; it does not prove live Auth/staff permissions,
backend sanction enforcement, staging OAuth, or final acceptance.

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
