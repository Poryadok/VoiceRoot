# Integration tests (device / browser / live API)

End-to-end scenarios with a real API use the same Flutter HTTP/WS clients as the app.

## Live Gateway stack (app stack–3 + Voice)

API-level live tests run on the VM with `flutter test` (not a browser driver).

| Test | Scope |
|------|--------|
| [`test/gateway_dm_ws_live_integration_test.dart`](../test/gateway_dm_ws_live_integration_test.dart) | REST auth, DM, send, WS `message_create` |
| [`test/dm_two_users_e2e_live_test.dart`](../test/dm_two_users_e2e_live_test.dart) | Two accounts, DM, WS, JWT **refresh**, REST **mark read**, WS `mark_read` fanout |
| [`test/friends_e2e_live_test.dart`](../test/friends_e2e_live_test.dart) | Friend invite → accept → both list friends |
| [`test/auth_logout_e2e_live_test.dart`](../test/auth_logout_e2e_live_test.dart) | Logout → JWT blacklisted (401 on `/api/v1/chats`) |
| [`test/ws_resume_e2e_live_test.dart`](../test/ws_resume_e2e_live_test.dart) | WS gap + **REST** message catch-up (`resume` bookkeeping) |
| [`test/voice_call_signaling_e2e_live_test.dart`](../test/voice_call_signaling_e2e_live_test.dart) | Voice call start/accept/decline, join token (no LiveKit media) |
| [`test/message_typing_e2e_live_test.dart`](../test/message_typing_e2e_live_test.dart) | `typing_start` → peer `typing` WS |
| [`test/message_edit_delete_e2e_live_test.dart`](../test/message_edit_delete_e2e_live_test.dart) | REST edit/delete → WS `message_update`, history |
| [`test/message_delivery_e2e_live_test.dart`](../test/message_delivery_e2e_live_test.dart) | `delivery_ack` → sender `message_delivered` |
| [`test/dm_requests_e2e_live_test.dart`](../test/dm_requests_e2e_live_test.dart) | Stranger DM → `requests` inbox → accept → `main` |
| [`test/file_attachment_e2e_live_test.dart`](../test/file_attachment_e2e_live_test.dart) | File upload + attachment message (skips without MinIO/R2) |
| [`test/mobile_layout_e2e_live_test.dart`](../test/mobile_layout_e2e_live_test.dart) | platforms (docs/features/platforms.md) mobile layout: gateway health + narrow breakpoint contract |
| [`test/windows_version_e2e_live_test.dart`](../test/windows_version_e2e_live_test.dart) | Windows desktop `/version` policy + dynamic 426 for outdated client |
| [`test/trust_e2e_live_test.dart`](../test/trust_e2e_live_test.dart) | privacy/trust (docs/features/privacy.md): privacy DM block, report 202, 2FA enroll/verify/login gate |
| [`test/bots_slash_e2e_live_test.dart`](../test/bots_slash_e2e_live_test.dart) | bots (docs/features/bots.md): install polling bot, `/ping` → pong in history |
| [`test/bots_ephemeral_live_test.dart`](../test/bots_ephemeral_live_test.dart) | bots (docs/features/bots.md): ephemeral slash response (invoker-only) |
| [`test/bots_botc_live_test.dart`](../test/bots_botc_live_test.dart) | bots (docs/features/bots.md) BOT-C: presence heartbeat, space members, create chat |

Go Gateway mirror tests: `src/backend/gateway/compose_*_live_test.go` — run via `make compose-e2e-live` (Go only) or **`make compose-e2e-full`** (Go + Flutter).

Shared flags: `VOICE_RUN_LIVE_INTEGRATION=true`, `VOICE_API_BASE_URL=...`

### Prerequisites (local compose)

From the **repo root**, start the app stack stack (Auth, User, Social, Chat, Messaging, Realtime, Voice, File, Gateway, web):

```text
make compose-app-up
```

In `.env`, publish the gateway on host port **18080** (recommended; see root [README.md](../../../README.md)):

```text
GATEWAY_PORT=18080
VOICE_API_PUBLIC_URL=http://127.0.0.1:18080
```

For file attachment e2e, configure MinIO/R2 vars in `.env` (see [`.env.example`](../../../.env.example)).

Wait until `docker compose --profile app ps` shows gateway (and dependencies) healthy. Base URL for live tests:

```text
VOICE_API_BASE_URL=http://127.0.0.1:18080
```

**Full live suite (Go + Flutter):**

```powershell
make compose-e2e-full
```

**core E2E run (PowerShell, Flutter only):**

```powershell
cd src/frontend
flutter test test/dm_two_users_e2e_live_test.dart `
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

With the flag set, tests **fail** if the stack is down or gateway returns errors (e.g. 404 on `/api/v1/chats` when core services are not running).

**Staging example:**

```powershell
flutter test test/gateway_dm_ws_live_integration_test.dart `
  --dart-define=VOICE_RUN_LIVE_INTEGRATION=true `
  --dart-define=VOICE_API_BASE_URL=https://voice.comrade.click
```

Default staging hostnames live in [`deploy/staging/domains.defaults`](../../../deploy/staging/domains.defaults).

See `docs/TESTING.md` and the `flutter-web-client-testing` skill.

## Chrome integration_test (deep-links/platforms (docs/features/deep-links.md) deep links)

UI-driven navigation in Chrome (no live API required — ProviderScope overrides):

| Test | Scope |
|------|--------|
| [`integration_test/deeplink_web_test.dart`](deeplink_web_test.dart) | `applyDeepLinkNavigation` → selected chat + conversation semantics |

**CI:** job `flutter-web-integration` in [`.github/workflows/ci.yml`](../../../.github/workflows/ci.yml).

**Local (PowerShell):**

```powershell
cd src/frontend
flutter test integration_test/deeplink_web_test.dart -d chrome `
  --dart-define=VOICE_API_BASE_URL=http://127.0.0.1:18080 `
  --web-browser-flag=--disable-gpu
```

Prod App Links / on-device AASA — [TODO.md](../../../docs/TODO.md) Batch 6.
