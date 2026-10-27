# TODO — Admin

[← Индекс](../TODO.md)

Moderation queue, analytics dashboards, OAuth staff; Developer Portal для ботов.

Admin panel (`src/admin/`) и Developer Portal (`src/developer-portal/`).

## High

### Admin


- [ ] **[Admin] Analytics UI: search/voice dashboards deferred** — Gateway `GetDashboard` supports product/engagement/revenue/health/moderation + retention REST; Admin pages wired (batch 4). Search/voice need an accepted backend contract before UI: the current wire has no unambiguous daily-series/distribution or dashboard-filter schema, and source telemetry lacks canonical click-position, concurrency/duration and negotiated-codec facts. Do not ship proxy counters under the final metric names. Contract/RED matrix: [analytics-search-voice-dashboard-contract.md](../testing/analytics-search-voice-dashboard-contract.md). Paths: `docs/features/analytics.md`, `src/backend/analytics/internal/store/query.go`
- Rich UX backlog (game catalog): rank ladder templates per [game-catalog.md](../features/game-catalog.md). Before implementation, define the template names, ordered rank names/values and whether applying a template replaces or merges existing ranks; these are not yet specified. Genre/platform suggestions and the structured mode/role/rank editor are shipped.

### Developer Portal


- [x] **[Developer Portal] Compose OAuth and bot-registration HTTP E2E** — `TestComposeDeveloperPortal_live` is selected by `.github/ci/e2e-features.yml` and exercises real Voice password authorization, S256 PKCE, callback state, one-use code, authenticated bot registration/list/detail and cleanup. Redirect/password/verifier/replay/unauthenticated failures are asserted; an unavailable opted-in portal fails. This proves backend protocol and persistence; browser execution, staging OAuth and live-provider acceptance remain separate. — `src/backend/gateway/compose_developer_portal_live_test.go`, `src/developer-portal/README.md`


## Common

### Admin


- [ ] **[Admin] Live staff browser acceptance** — deterministic Chromium coverage now exercises the built Admin OAuth/PKCE and moderation/appeal flows against a local HTTP fixture (`src/admin/browser/staff-flows.mjs`, `admin-browser.yml`). Actual Auth/staff authority, sanction enforcement and staging OAuth remain live acceptance gates; fixture success does not close them.
- [ ] **[Admin] Staging smoke: Admin UI routes only** — batch 5 adds SPA root + `/callback` when `VOICE_ADMIN_INGRESS_HOST` set; Gateway admin moderation/analytics still via `STAGING_STAFF_TOKEN` (not Admin UI bundle). Path: `scripts/staging/smoke-staging.sh`

### Developer Portal

_(Common Developer Portal batch closed in PR #124 — registration form, bot detail fetch, secrets clear on switch, privileged scope warnings, lifecycle UI, PLAN/DEPLOYMENT/README.)_


## Low

### Developer Portal


- [ ] **[Developer Portal] No design tokens / brand — raw CSS; admin uses `tokens.css`.** — `src/developer-portal/src/styles/global.css`, `src/admin/src/styles/tokens.css`
- [x] **[Developer Portal] Ad-hoc routing** — `BrowserRouter` selects the case-sensitive `/callback` route and updates on browser history navigation; direct callback queries, state/PKCE checks, token exchange and full-document success redirect remain intact, with the Portal fallback for unknown and case-mismatched paths. Covered by `src/developer-portal/src/test/AppRouting.test.tsx` and `src/developer-portal/src/test/OAuthCallback.test.tsx`; OAuth transport was not changed. — `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] Deployment runtime proof** — manifests define resource budgets and bounded rolling updates (one staging / two production replicas); production already has origin TLS, staging retains the documented HTTPS-edge/HTTP-origin route. Verify real readiness-failure handling, rollout capacity and public HTTPS OAuth callback under the protected environment window; offline validation is not rollout evidence. — `deploy/staging/developer-portal.yaml`, `deploy/prod/developer-portal.yaml`, `docs/DEPLOYMENT.md`
- [x] **[Developer Portal] Slug / public bot preview** — Fresh selected-bot details show the server-assigned read-only slug and saved public name/description/scopes/commands, with an encoded canonical public-page link. Preview clears on selection/logout; missing slug and unavailable data stay explicit. No slug-write API or live install proof is implied. — `src/developer-portal/src/PublicBotPreview.tsx`, `src/developer-portal/src/test/AppBotPreview.test.tsx`


**Промпт-якорь:** `Admin from docs/todo/admin.md` + приоритет.
