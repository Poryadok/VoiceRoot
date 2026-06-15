# Voice Developer Portal (Phase 16)

Minimal Vite app for bot registration, manifest validate/apply, and token display.

## Run locally

```bash
cd src/developer-portal
npm install
VITE_VOICE_API_BASE=http://127.0.0.1:18080 npm run dev
```

Paste a user JWT from the Voice client (account that owns bots). Separate deploy from main `app` compose profile is OK for production.
