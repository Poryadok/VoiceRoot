# Frontend (Flutter)

Voice client (Flutter Web/Desktop/Mobile). Phases 0–10 UI on local `make compose-app-up` — см. [docs/PLAN.md](../../docs/PLAN.md).

## Client

- Three-column shell ([docs/features/navigation.md](../../docs/features/navigation.md)); mobile strip + responsive layout (Phase 8).
- Auth, social, DM/groups/spaces, matchmaking, search, voice, push hooks — чеклисты в [docs/PLAN.md](../../docs/PLAN.md).
- API base URL: `--dart-define=VOICE_API_BASE_URL=https://your-gateway` (empty = offline / tests inject via Riverpod).
- Riverpod + Gateway/Auth/Users/Social/Chat/Messaging/Realtime clients.

## Commands

```bash
cd src/frontend
flutter pub get
flutter analyze
flutter test
```

**Live API (Gateway + full Phase-1 stack):** see [`integration_test/README.md`](integration_test/README.md) — `gateway_dm_ws_live_integration_test.dart` (smoke) and `phase1_two_users_e2e_live_test.dart` (two users, refresh, mark read) with `VOICE_RUN_LIVE_INTEGRATION=true`.

Из корня репозитория (нужен Flutter на `PATH`): **`make flutter-ci`** — те же шаги ([Makefile](../../Makefile)).

CI: [.github/workflows/ci.yml](../../.github/workflows/ci.yml) jobs `flutter` and `flutter-windows`. Цель **`make build-all`** Flutter не запускает — см. [docs/TESTING.md](../../docs/TESTING.md).

## Windows desktop release

```powershell
.\scripts\release\windows-build.ps1 -Version 1.0.0 -ApiBaseUrl http://127.0.0.1:18080
```

- Build uses `auto_updater` (WinSparkle) for signed delta updates; publish `appcast.xml` from [`scripts/release/appcast-template.xml`](../../scripts/release/appcast-template.xml) to CDN/R2.
- Gateway serves `/api/v1/version?platform=windows` and returns `426` when `X-Voice-Client-Version` is below `min_supported_version` ([docs/features/updates.md](../../docs/features/updates.md)).

## Docker (Phase 1 stack)

Из корня репозитория:

```bash
make compose-app-up
```

В `.env` (рекомендуется): `GATEWAY_PORT=18080`, `VOICE_API_PUBLIC_URL=http://127.0.0.1:18080`, `WEB_PORT=9080`. Веб: **`http://127.0.0.1:9080`**, gateway: **`http://127.0.0.1:18080`**. Подробнее — [README.md](../../README.md).
