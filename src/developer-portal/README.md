# Voice Developer Portal (bots (docs/features/bots.md))

Minimal Vite app for bot registration, manifest validate/apply, and token display.

Selecting a bot loads its server-assigned, read-only public slug and a preview
of its saved name, description, requested scopes, and registered commands.
Unsaved editor changes do not alter the preview. The public-page link uses
`https://voice.gg/bots/{slug}`; a bot without a slug has no public-page link.
Applying a manifest clears and reloads both saved public details and commands;
the previous preview stays hidden if the detail refresh fails.
Changing selection or signing out clears the preview, and unavailable catalog
data is shown explicitly. This is a preview of public details, not an installation
or live delivery check. Bot tokens, webhook secrets and owner/private webhook
fields are not part of the preview. The existing API does not support editing
slugs during registration or update.

## Authentication

Production flow uses **OAuth 2.0 authorization code + PKCE** against Voice Auth (via API Gateway):

1. **Sign in with Voice** → redirect to `{API_BASE}/api/v1/auth/oauth2/authorize`
2. Auth shows login form (email/password, optional TOTP)
3. Redirect back to `/callback?code=…&state=…`
4. Portal exchanges the code at `POST /api/v1/auth/oauth2/token` and stores `access_token` in `sessionStorage`
5. All API calls use `Authorization: Bearer …`

BrowserRouter selects the `/callback` route and follows browser history changes;
other paths display the portal. Successful OAuth still replaces the document at
`/` after storing the token.

Configure in `.env` (see repo root [`.env.example`](../../.env.example) — `DEVELOPER_PORTAL_*` vars):

- `DEVELOPER_PORTAL_OAUTH_CLIENT_ID`
- `DEVELOPER_PORTAL_OAUTH_REDIRECT_URIS` (must include `http://localhost:9082/callback` for compose)
- `VOICE_API_PUBLIC_URL` (gateway URL reachable from the browser)

Set `DEVELOPER_PORTAL_OAUTH_DISABLED=true` or `VITE_OAUTH_DISABLED=true` to show a **paste JWT** field for local debugging only.

## Run locally

```bash
cd src/developer-portal
npm install
VITE_VOICE_API_BASE=http://127.0.0.1:18080 \
VITE_OAUTH_CLIENT_ID=voice-developer-portal \
npm run dev
```

Open http://localhost:5174 — Vite dev server; add `http://localhost:5174/callback` to Auth redirect URIs if testing OAuth locally.

## Compose

With `docker compose --profile app up`, portal is at http://localhost:9082 (or `DEVELOPER_PORTAL_PORT`).

## Tests

```bash
npm test
```
