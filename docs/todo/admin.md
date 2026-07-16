# TODO — Admin

[← Индекс](../TODO.md)

Moderation queue, analytics dashboards, OAuth staff; Developer Portal для ботов.

Admin panel (`src/admin/`) и Developer Portal (`src/developer-portal/`).

## Critical

### Admin


- [ ] **[Admin] Assign-to-me broken under OAuth (staging default)** — `staffProfileIdFromToken()` reads only `VITE_STAFF_TOKEN`, not the session OAuth JWT; staging builds use `VITE_OAUTH_DISABLED=false`. Paths: `src/admin/src/lib/jwt.ts`, `src/admin/src/pages/QueuePage.tsx`
- [ ] **[Admin] Audit log always empty** — Gateway returns hardcoded `{"entries":[]}`; Admin UI cannot show real audit data. Paths: `src/backend/gateway/transcode_moderation_admin.go`, `src/admin/src/pages/AuditPage.tsx`
- [ ] **[Admin] No resolve / dismiss report workflow** — only `new_status: "reviewing"` (assign); moderators cannot close reports as resolved/dismissed per `docs/features/reports.md`. Paths: `src/admin/src/pages/QueuePage.tsx`, `src/admin/src/components/ReportDetail.tsx`, `src/admin/src/api/moderation.ts`


## High

### Admin


- [ ] **[Admin] Appeals admin flow missing end-to-end** — `ReviewAppeal` in moderation gRPC, no Gateway admin REST, no Admin pages. Paths: `docs/microservices/moderation-service.md`, `src/backend/gateway/` (no appeal routes), `src/admin/src/`
- [ ] **[Admin] `temp_ban` without `expires_at`** — proto supports it; UI never sends duration. Paths: `protos/voice/moderation/v1/moderation.proto`, `src/admin/src/components/SanctionActions.tsx`, `src/admin/src/api/moderation.ts`
- [ ] **[Admin] Sanctions on non-`user` targets** — message/space/story reports: account resolution fails or misuses `target_id` as account. Paths: `src/admin/src/pages/QueuePage.tsx`, `src/admin/src/components/SanctionActions.tsx`
- [ ] **[Admin] No queue pagination** — `next_cursor` in types; `ListReports` never passes cursor. Paths: `src/admin/src/api/moderation.ts`, `src/admin/src/api/types.ts`, `src/admin/src/pages/QueuePage.tsx`, `protos/voice/moderation/v1/moderation.proto`
- [ ] **[Admin] Analytics UI far below spec** — only `product` dashboard + hardcoded `registration` funnel; no retention page (`fetchRetention` unused), no engagement/revenue/health/moderation/search/voice dashboards. Paths: `src/admin/src/api/analytics.ts`, `src/admin/src/pages/ProductAnalyticsPage.tsx`, `src/admin/src/pages/FunnelsPage.tsx`, `docs/features/analytics.md`
- [ ] **[Admin] No revoke-sanction UI** — `RevokeSanction` RPC exists server-side, not exposed in Admin. Paths: `docs/microservices/moderation-service.md`, `src/admin/src/api/moderation.ts`
- [ ] **[Admin] PLAN / README stale** — PLAN still «зарезервировано»; README covers moderation only (no analytics/OAuth). Paths: `docs/PLAN.md`, `src/admin/README.md`

### Developer Portal


- [ ] **[Developer Portal] OAuth `state` never persisted or validated on callback — generated in `signInWithVoice()` but not stored; `OAuthCallback` ignores `params.state`. Login CSRF vector (same pattern as admin).** — `src/developer-portal/src/App.tsx`, `src/developer-portal/src/OAuthCallback.tsx`
- [ ] **[Developer Portal] No JWT expiry handling — `isLoggedIn()` checks token presence only; `expires_in` from token exchange ignored; no refresh or re-auth on 401.** — `src/developer-portal/src/oauth/session.ts`, `src/developer-portal/src/oauth/api.ts`, `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] No manifest round-trip — backend stores manifest fields/commands but no REST to export YAML; `GetCommands` not transcoded in Gateway. Portal always shows `defaultManifest`, cannot load applied config.** — `src/developer-portal/src/App.tsx`, `src/developer-portal/src/manifestDefaults.ts`, `src/backend/gateway/transcode_bots.go`, `protos/voice/bot/v1/bot.proto`
- [ ] **[Developer Portal] No E2E / live smoke for portal OAuth or bot registration — only static `GET /` in staging smoke; no compose live test, nothing in `.github/ci/e2e-features.yml`.** — `scripts/staging/smoke-staging.sh`, `.github/ci/e2e-features.yml`


## Common

### Admin


- [ ] **[Admin] No admin E2E / browser tests** — `docs/TESTING.md` lists analytics live tests as backend-only; no Playwright/Cypress for Admin. Paths: `docs/TESTING.md`, `src/admin/src/test/`
- [ ] **[Admin] Thin unit coverage** — 4 tests (queue filters, sanction confirm, analytics client mocks); no OAuth callback, login, audit, analytics pages. Paths: `src/admin/src/test/analytics.test.ts`, `src/admin/src/test/QueueFilters.test.tsx`, `src/admin/src/test/SanctionConfirm.test.tsx`
- [ ] **[Admin] OAuth callback skips `state` validation** — CSRF risk on PKCE flow. Paths: `src/admin/src/oauth/OAuthCallback.tsx`, `src/admin/src/oauth/callback.ts`, `src/admin/src/App.tsx`
- [ ] **[Admin] Duplicate HTTP helpers** — `apiFetch` in both `src/admin/src/oauth/api.ts` and `src/admin/src/api/client.ts`
- [ ] **[Admin] Incomplete dev env template** — `.env.example` missing `VITE_OAUTH_CLIENT_ID`, `VITE_OAUTH_DISABLED`. Path: `src/admin/.env.example`
- [ ] **[Admin] No lint step in CI** — no ESLint/Prettier for `src/admin/`. Path: `.github/workflows/ci.yml` (job `admin`)
- [ ] **[Admin] Staging smoke does not exercise Admin APIs** — only `GET /health`; analytics smoke uses direct Gateway + `STAGING_STAFF_TOKEN`, not Admin UI. Path: `scripts/staging/smoke-staging.sh`

### Developer Portal


- [ ] **[Developer Portal] Bot registration UI is hardcoded — fixed name `"DevPortal Bot"`, description, single scope; no form for name/description/scopes per `docs/features/bots.md` manifest model.** — `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] Missing bot lifecycle UI — Gateway exposes `PATCH`/`DELETE` `/api/v1/bots/{id}` but portal has no update/delete.** — `src/developer-portal/src/App.tsx`, `src/backend/gateway/transcode_bots.go`
- [ ] **[Developer Portal] No bot detail fetch on selection — never calls `GET /api/v1/bots/{id}`; list shows name/id only.** — `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] Secrets leak across bot selection — `botToken` / `webhookSecret` state not cleared when switching bots.** — `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] Privileged scope warnings absent — `bots.md` requires explicit warning for `TEXT_CHAT_READ_HISTORY` / `SPACE_MANAGE_ROLES`; manifest textarea has no validation UX.** — `src/developer-portal/src/App.tsx`, `docs/features/bots.md`
- [ ] **[Developer Portal] Not in implementation map — `docs/PLAN.md` lists bots as partial but omits `src/developer-portal/` from “Размещение кода”.** — `docs/PLAN.md`
- [ ] **[Developer Portal] DEPLOYMENT.md stale on prod — says prod portal Ingress “not in-repo yet” but manifest exists.** — `docs/DEPLOYMENT.md`, `deploy/prod/developer-portal.yaml`
- [ ] **[Developer Portal] README points to missing local `.env.example` — refers to `src/developer-portal/.env.example`; vars live in repo root `.env.example`.** — `src/developer-portal/README.md`, `.env.example`
- [ ] **[Developer Portal] Test coverage is helper-only — no component tests for `App` / `OAuthCallback`; `webhook_secret.test.ts` mirrors parsing inline, not real UI.** — `src/developer-portal/src/test/`
- [ ] **[Developer Portal] Staging smoke incomplete vs TODO intent — HTTP 200 on `/` only; no `/callback` SPA route, no check that baked `VITE_VOICE_API_BASE` matches gateway FQDN.** — `scripts/staging/smoke-staging.sh`, `src/developer-portal/Dockerfile`
- [ ] **[Developer Portal] No `vite-env.d.ts` — unlike admin, no typed `import.meta.env`.** — `src/developer-portal/` (missing; compare `src/admin/src/vite-env.d.ts`)


## Low

### Developer Portal


- [ ] **[Developer Portal] No design tokens / brand — raw CSS; admin uses `tokens.css`.** — `src/developer-portal/src/styles/global.css`, `src/admin/src/styles/tokens.css`
- [ ] **[Developer Portal] Ad-hoc routing — `window.location.pathname === '/callback'` instead of router.** — `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] One-shot secrets UX — token/webhook shown in plain `<code>`; no copy-once modal, no clear-after-navigation.** — `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] K8s manifest minimal — no resources/limits, single replica, HTTP-only Traefik entrypoint (same as web/admin).** — `deploy/staging/developer-portal.yaml`, `deploy/prod/developer-portal.yaml`
- [ ] **[Developer Portal] No slug / public bot page preview — `slug` exists on Bot proto but unused in portal.** — `src/developer-portal/src/App.tsx`, `protos/voice/bot/v1/bot.proto`
- [ ] **[Developer Portal] No ESLint/analyze step in CI — only `vitest run`; no static analysis gate.** — `src/developer-portal/package.json`, `.github/workflows/ci.yml`


**Промпт-якорь:** `Admin from docs/todo/admin.md` + приоритет.
