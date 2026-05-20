# Integration tests (device / browser / live API)

End-to-end scenarios with a real API use the same Flutter HTTP/WS clients as the app.

## Live Gateway stack (Phase 1 DM + Realtime)

API-level live tests run on the VM with `flutter test` (not a browser driver).

| Test | Scope |
|------|--------|
| [`test/gateway_dm_ws_live_integration_test.dart`](../test/gateway_dm_ws_live_integration_test.dart) | REST auth, DM, send, WS `message_create` |
| [`test/phase1_two_users_e2e_live_test.dart`](../test/phase1_two_users_e2e_live_test.dart) | Two accounts, DM, WS, JWT **refresh**, REST **mark read**, WS `mark_read` fanout |

Shared flags: `VOICE_RUN_LIVE_INTEGRATION=true`, `VOICE_API_BASE_URL=...`

### Prerequisites (local compose)

From the **repo root**, start the Phase 1 stack (Auth, User, Social, Chat, Messaging, Realtime, Gateway, web):

```text
make compose-app-up
```

In `.env`, publish the gateway on host port **18080** (recommended; see root [README.md](../../../README.md)):

```text
GATEWAY_PORT=18080
VOICE_API_PUBLIC_URL=http://127.0.0.1:18080
```

Wait until `docker compose --profile app ps` shows gateway (and dependencies) healthy. Base URL for live tests:

```text
VOICE_API_BASE_URL=http://127.0.0.1:18080
```

**Phase-1 E2E run (PowerShell):**

```powershell
cd src/frontend
flutter test test/phase1_two_users_e2e_live_test.dart `
  --dart-define=VOICE_RUN_LIVE_INTEGRATION=true `
  --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
```

**Smoke (gateway DM + WS only):**

- REST: `register` → JWT
- REST: `POST /api/v1/chats/dm`
- REST: `POST /api/v1/messages/send`
- WS: `/ws` → `hello` → `subscribe` → `message_create`

**Run (PowerShell):**

```powershell
cd src/frontend
flutter test test/gateway_dm_ws_live_integration_test.dart `
  --dart-define=VOICE_RUN_LIVE_INTEGRATION=true `
  --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080
```

Without `VOICE_RUN_LIVE_INTEGRATION=true` the tests **skip** so `make flutter-ci` stays green.

With the flag set, tests **fail** if the stack is down or gateway returns errors (e.g. 404 on `/api/v1/chats` when Phase-1 services are not running).

**Staging example:**

```powershell
flutter test test/gateway_dm_ws_live_integration_test.dart `
  --dart-define=VOICE_RUN_LIVE_INTEGRATION=true `
  --dart-define=VOICE_API_BASE_URL=https://voice.tastytest.online
```

See `docs/TESTING.md` and the `flutter-web-client-testing` skill.

## Future: `integration_test` + web driver

UI-driven `integration_test/` targets (Chrome / device) can be added when CI wires ChromeDriver; API-level checks above do not require it.
