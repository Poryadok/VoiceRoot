# TODO — Admin

[← Индекс](../TODO.md)

Moderation queue, analytics dashboards, OAuth staff; Developer Portal для ботов.

Admin panel (`src/admin/`) и Developer Portal (`src/developer-portal/`).

## High

### Admin


- [ ] **[Admin] Analytics UI: search/voice dashboards deferred** — Gateway `GetDashboard` supports product/engagement/revenue/health/moderation + retention REST; Admin pages wired (batch 4). Search/voice need an accepted backend contract before UI: the current wire has no unambiguous daily-series/distribution or dashboard-filter schema, and source telemetry lacks canonical click-position, concurrency/duration and negotiated-codec facts. Do not ship proxy counters under the final metric names. Contract/RED matrix: [analytics-search-voice-dashboard-contract.md](../testing/analytics-search-voice-dashboard-contract.md). Paths: `docs/features/analytics.md`, `src/backend/analytics/internal/store/query.go`
- Rich UX backlog (game catalog): rank ladder templates per [game-catalog.md](../features/game-catalog.md). Before implementation, define the template names, ordered rank names/values and whether applying a template replaces or merges existing ranks; these are not yet specified. Genre/platform suggestions and the structured mode/role/rank editor are shipped.

### Developer Portal


- [ ] **[Developer Portal] No E2E / live smoke for portal OAuth or bot registration — only static ingress checks in staging smoke; no compose live test, nothing in `.github/ci/e2e-features.yml`.** — `scripts/staging/smoke-staging.sh`, `.github/ci/e2e-features.yml`


## Common

### Admin


- [ ] **[Admin] Live staff browser acceptance** — deterministic Chromium coverage now exercises the built Admin OAuth/PKCE and moderation/appeal flows against a local HTTP fixture (`src/admin/browser/staff-flows.mjs`, `admin-browser.yml`). Actual Auth/staff authority, sanction enforcement and staging OAuth remain live acceptance gates; fixture success does not close them.
- [ ] **[Admin] Staging smoke: Admin UI routes only** — batch 5 adds SPA root + `/callback` when `VOICE_ADMIN_INGRESS_HOST` set; Gateway admin moderation/analytics still via `STAGING_STAFF_TOKEN` (not Admin UI bundle). Path: `scripts/staging/smoke-staging.sh`

### Developer Portal

_(Common Developer Portal batch closed in PR #124 — registration form, bot detail fetch, secrets clear on switch, privileged scope warnings, lifecycle UI, PLAN/DEPLOYMENT/README.)_


## Low

### Developer Portal


- [ ] **[Developer Portal] No design tokens / brand — raw CSS; admin uses `tokens.css`.** — `src/developer-portal/src/styles/global.css`, `src/admin/src/styles/tokens.css`
- [ ] **[Developer Portal] Ad-hoc routing — `window.location.pathname === '/callback'` instead of router.** — `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] Deployment runtime proof** — manifests define resource budgets and bounded rolling updates (one staging / two production replicas); production already has origin TLS, staging retains the documented HTTPS-edge/HTTP-origin route. Verify real readiness-failure handling, rollout capacity and public HTTPS OAuth callback under the protected environment window; offline validation is not rollout evidence. — `deploy/staging/developer-portal.yaml`, `deploy/prod/developer-portal.yaml`, `docs/DEPLOYMENT.md`
- [ ] **[Developer Portal] No slug / public bot page preview — `slug` exists on Bot proto but unused in portal.** — `src/developer-portal/src/App.tsx`, `protos/voice/bot/v1/bot.proto`


**Промпт-якорь:** `Admin from docs/todo/admin.md` + приоритет.
