# TODO ΓÇö Admin

[ΓåÉ ╨ÿ╨╜╨┤╨╡╨║╤ü](../TODO.md)

Moderation queue, analytics dashboards, OAuth staff; Developer Portal ╨┤╨╗╤Å ╨▒╨╛╤é╨╛╨▓.

Admin panel (`src/admin/`) ╨╕ Developer Portal (`src/developer-portal/`).

## High

### Admin


- [ ] **[Admin] Analytics UI: search/voice dashboards deferred** ΓÇö Gateway `GetDashboard` supports product/engagement/revenue/health/moderation + retention REST; Admin pages wired (batch 4). Search/voice dashboard types absent in `src/backend/analytics/internal/store/query.go` ΓÇö needs backend before UI. Paths: `docs/features/analytics.md`, `src/backend/analytics/internal/store/query.go`
- Rich UX backlog (game catalog): genre/platforms presets, rank ladder templates per [game-catalog.md](../features/game-catalog.md) ΓÇö structured mode/role/rank editor already shipped.

### Developer Portal


- [ ] **[Developer Portal] No E2E / live smoke for portal OAuth or bot registration ΓÇö only static ingress checks in staging smoke; no compose live test, nothing in `.github/ci/e2e-features.yml`.** ΓÇö `scripts/staging/smoke-staging.sh`, `.github/ci/e2e-features.yml`


## Common

### Admin


- [ ] **[Admin] No admin E2E / browser tests** ΓÇö `docs/TESTING.md` lists analytics live tests as backend-only; no Playwright/Cypress for Admin. Paths: `docs/TESTING.md`, `src/admin/src/test/`
- [ ] **[Admin] Staging smoke: Admin UI routes only** ΓÇö batch 5 adds SPA root + `/callback` when `VOICE_ADMIN_INGRESS_HOST` set; Gateway admin moderation/analytics still via `STAGING_STAFF_TOKEN` (not Admin UI bundle). Path: `scripts/staging/smoke-staging.sh`

### Developer Portal

_(Common Developer Portal batch closed in PR #124 ΓÇö registration form, bot detail fetch, secrets clear on switch, privileged scope warnings, lifecycle UI, PLAN/DEPLOYMENT/README.)_


## Low

### Developer Portal


- [ ] **[Developer Portal] No design tokens / brand ΓÇö raw CSS; admin uses `tokens.css`.** ΓÇö `src/developer-portal/src/styles/global.css`, `src/admin/src/styles/tokens.css`
- [ ] **[Developer Portal] Ad-hoc routing ΓÇö `window.location.pathname === '/callback'` instead of router.** ΓÇö `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] One-shot secrets UX ΓÇö token/webhook shown in plain `<code>`; no copy-once modal, no clear-after-navigation.** ΓÇö `src/developer-portal/src/App.tsx`
- [ ] **[Developer Portal] K8s manifest minimal ΓÇö no resources/limits, single replica, HTTP-only Traefik entrypoint (same as web/admin).** ΓÇö `deploy/staging/developer-portal.yaml`, `deploy/prod/developer-portal.yaml`
- [ ] **[Developer Portal] No slug / public bot page preview ΓÇö `slug` exists on Bot proto but unused in portal.** ΓÇö `src/developer-portal/src/App.tsx`, `protos/voice/bot/v1/bot.proto`


**╨ƒ╤Ç╨╛╨╝╨┐╤é-╤Å╨║╨╛╤Ç╤î:** `Admin from docs/todo/admin.md` + ╨┐╤Ç╨╕╨╛╤Ç╨╕╤é╨╡╤é.
