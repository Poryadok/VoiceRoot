# TODO — Admin

[← Индекс](../TODO.md)

Moderation queue, analytics dashboards, OAuth staff; Developer Portal для ботов.

Admin panel (`src/admin/`) и Developer Portal (`src/developer-portal/`).

## High

### Admin


- [ ] **[Admin] Analytics UI: search/voice dashboards deferred** — Gateway `GetDashboard` supports product/engagement/revenue/health/moderation + retention REST; Admin pages wired (batch 4). Search/voice dashboard types absent in `src/backend/analytics/internal/store/query.go` — needs backend before UI. Paths: `docs/features/analytics.md`, `src/backend/analytics/internal/store/query.go`
- [ ] **[Admin] Game catalog: advanced mode/role editor deferred** — staff CreateGame form + name dedup via `SearchGames` live (batch 4); rich mode/role UX per [game-catalog.md](../features/game-catalog.md) still thin.

### Developer Portal


- [ ] **[Developer Portal] No E2E / live smoke for portal OAuth or bot registration — only static ingress checks in staging smoke; no compose live test, nothing in `.github/ci/e2e-features.yml`.** — `scripts/staging/smoke-staging.sh`, `.github/ci/e2e-features.yml`


## Common

### Admin


- [ ] **[Admin] No admin E2E / browser tests** — `docs/TESTING.md` lists analytics live tests as backend-only; no Playwright/Cypress for Admin. Paths: `docs/TESTING.md`, `src/admin/src/test/`
- [ ] **[Admin] Thin unit coverage (partial)** — OAuth callback, login (`AppLogin`), audit page, canonical `api/client` tests added (batch 5); analytics pages covered in batch 4. Remaining gaps: full App shell routing smoke in vitest only. Paths: `src/admin/src/test/`
- [ ] **[Admin] No lint step in CI** — no ESLint/Prettier for `src/admin/`. Path: `.github/workflows/ci.yml` (job `admin`); deferred batch 5 (ESLint bootstrap)
- [ ] **[Admin] Staging smoke: Admin UI routes only** — batch 5 adds SPA root + `/callback` when `VOICE_ADMIN_INGRESS_HOST` set; Gateway admin moderation/analytics still via `STAGING_STAFF_TOKEN` (not Admin UI bundle). Path: `scripts/staging/smoke-staging.sh`

### Developer Portal


- [ ] **[Developer Portal] Bot registration UI is hardcoded — fixed name `"DevPortal Bot"`, description, single scope; no form for name/description/scopes per `docs/features/bots.md` manifest model.** — `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] Missing bot lifecycle UI — Gateway exposes `PATCH`/`DELETE` `/api/v1/bots/{id}` but portal has no update/delete.** — `src/developer-portal/src/App.tsx`, `src/backend/gateway/transcode_bots.go`
- [ ] **[Developer Portal] No bot detail fetch on selection — never calls `GET /api/v1/bots/{id}`; list shows name/id only.** — `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] Secrets leak across bot selection — `botToken` / `webhookSecret` state not cleared when switching bots.** — `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] Privileged scope warnings absent — `bots.md` requires explicit warning for `TEXT_CHAT_READ_HISTORY` / `SPACE_MANAGE_ROLES`; manifest textarea has no validation UX.** — `src/developer-portal/src/App.tsx`, `docs/features/bots.md`
- [ ] **[Developer Portal] Not in implementation map — `docs/PLAN.md` lists bots as partial but omits `src/developer-portal/` from “Размещение кода”.** — `docs/PLAN.md`
- [ ] **[Developer Portal] DEPLOYMENT.md stale on prod — says prod portal Ingress “not in-repo yet” but manifest exists.** — `docs/DEPLOYMENT.md`, `deploy/prod/developer-portal.yaml`
- [ ] **[Developer Portal] README points to missing local `.env.example` — refers to `src/developer-portal/.env.example`; vars live in repo root `.env.example`.** — `src/developer-portal/README.md`, `.env.example`


## Low

### Developer Portal


- [ ] **[Developer Portal] No design tokens / brand — raw CSS; admin uses `tokens.css`.** — `src/developer-portal/src/styles/global.css`, `src/admin/src/styles/tokens.css`
- [ ] **[Developer Portal] Ad-hoc routing — `window.location.pathname === '/callback'` instead of router.** — `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] One-shot secrets UX — token/webhook shown in plain `<code>`; no copy-once modal, no clear-after-navigation.** — `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] K8s manifest minimal — no resources/limits, single replica, HTTP-only Traefik entrypoint (same as web/admin).** — `deploy/staging/developer-portal.yaml`, `deploy/prod/developer-portal.yaml`
- [ ] **[Developer Portal] No slug / public bot page preview — `slug` exists on Bot proto but unused in portal.** — `src/developer-portal/src/App.tsx`, `protos/voice/bot/v1/bot.proto`
- [ ] **[Developer Portal] No ESLint/analyze step in CI — only `vitest run`; no static analysis gate.** — `src/developer-portal/package.json`, `.github/workflows/ci.yml`


**Промпт-якорь:** `Admin from docs/todo/admin.md` + приоритет.
