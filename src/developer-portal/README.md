# Voice Developer Portal (Phase 16)

Minimal Vite app for bot registration, manifest validate/apply, and token display.

## Authentication

Production flow uses **OAuth 2.0 authorization code + PKCE** against Voice Auth (via API Gateway):

1. **Sign in with Voice** → redirect to `{API_BASE}/api/v1/auth/oauth2/authorize`
2. Auth shows login form (email/password, optional TOTP)
3. Redirect back to `/callback?code=…&state=…`
4. Portal exchanges the code at `POST /api/v1/auth/oauth2/token` and stores `access_token` in `sessionStorage`
5. All API calls use `Authorization: Bearer …`

Configure in `.env` (see `.env.example`):

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
