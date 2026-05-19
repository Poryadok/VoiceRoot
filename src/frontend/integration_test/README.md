# Integration tests (device / browser / live API)

End-to-end scenarios with a real API use the same Flutter HTTP/WS clients as the app.

## Live Gateway stack (Phase 1 DM + Realtime)

API-level live tests run on the VM with `flutter test` (not a browser driver).

| Test | Scope |
|------|--------|
| [`test/gateway_dm_ws_live_integration_test.dart`](../test/gateway_dm_ws_live_integration_test.dart) | REST auth, DM, send, WS `message_create` |
| [`test/phase1_two_users_e2e_live_test.dart`](../test/phase1_two_users_e2e_live_test.dart) | Two accounts, DM, WS, JWT **refresh**, REST **mark read**, WS `mark_read` fanout |

Shared flags: `VOICE_RUN_LIVE_INTEGRATION=true`, `VOICE_API_BASE_URL=...`

**Phase-1 E2E run (PowerShell):**

```powershell
cd src/frontend
flutter test test/phase1_two_users_e2e_live_test.dart `
  --dart-define=VOICE_RUN_LIVE_INTEGRATION=true `
  --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:8080
```

**Smoke (gateway DM + WS only):**

- REST: `register` → JWT
- REST: `POST /api/v1/chats/dm`
- REST: `POST /api/v1/messages/send`
- WS: `/ws` → `hello` → `subscribe` → `message_create`

**Prerequisites:** Gateway with Auth, User, Chat, Messaging, Realtime upstreams and NATS JetStream (`message.events`). Compose profile `app` alone only exposes Gateway + static web — wire upstream URLs or use staging.

**Run (PowerShell):**

```powershell
cd src/frontend
flutter test test/gateway_dm_ws_live_integration_test.dart `
  --dart-define=VOICE_RUN_LIVE_INTEGRATION=true `
  --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:8080
```

Without `VOICE_RUN_LIVE_INTEGRATION=true` the test **skips** so `make flutter-ci` stays green.

With the flag set, the test **fails** if Gateway is up but Auth (or downstream) is not wired — e.g. compose profile `app` alone returns HTTP 404 on `/api/v1/auth/register`.

**Staging example:**

```powershell
flutter test test/gateway_dm_ws_live_integration_test.dart `
  --dart-define=VOICE_RUN_LIVE_INTEGRATION=true `
  --dart-define=VOICE_API_BASE_URL=https://voice.tastytest.online
```

See `docs/TESTING.md` and the `flutter-web-client-testing` skill.

## Future: `integration_test` + web driver

UI-driven `integration_test/` targets (Chrome / device) can be added when CI wires ChromeDriver; API-level checks above do not require it.
