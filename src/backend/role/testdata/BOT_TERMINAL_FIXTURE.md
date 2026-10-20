# Private Role26 terminal provider

This TEST-only provider composes immutable44 `af1d4a59` on183 `1eab095`:
official Role26 migrations, independent nonowner LOGIN, production RoleGRPC and
server factory, real mTLS/HTTPS JWKS, and isolated Redis7/PostgreSQL16. No Space,
actor, cleanup evidence or human permission is manufactured for root registration.
The minimal result fences BotActor Role mutations only; it does not complete
Bot global deletion, per-Space cleanup, P3 or public Gateway admission.

Run only inside the caller-owned Linux Go runner with Docker socket and private
network, no published host ports:

```
go test -tags role_bot_terminal . -run 'Test(RoleBotTerminalActualProvider|TerminalCompleteStrictBoundary)$' -race -count=1 -timeout=7m -v
```

Without export variables this is the isolated default positive/negative control.
It verifies actual own four-scalar row and typed receipt, transport replay denial,
fresh-JWT stable result, service reinstantiation on the same database/Redis,
Bot/root rebind conflicts, wrong issuer/body/method rejection, plaintext rejection,
local same-TX Bot fence denial and unchanged closed BotActor Create/Assign/Revoke
routes. Closed-route negatives never claim an admitted writer or core activation.

For247's actual committed Bot014 caller set:

- `VOICE_BOT_ROLE26_HANDOFF_DIR`: fresh absolute directory, not pre-created.
- `VOICE_BOT_ROLE26_SOURCE_PIN`: exact40 lowercase Git commit executed by runner.

The provider atomically publishes0600 `ready.json` after its real controls. ABI:
schema_version1, catalog_version26, fixture_only, address, server_name, ca_file,
client_cert_file, client_key_file, signer_key_file, key_id, source_pin, expires_at,
stop_file. The files contain temporary test credentials and must stay private in
the same owned Linux runner. No DSN/password/token/key contents are logged.

The production Bot caller first commits its accepted root and calls the exact
RegisterBotGlobalTerminal RPC with pure service:bot/full hash/root requestID/fresh
JTI. After durable ROLE journal CAS it atomically writes `complete.json`:
schema_version1, bot_id, decision_id, receipt_id (canonical nonzero UUIDs).
This synchronization file cannot insert a marker: the provider first requires an
existing exact Role row. It then independently checks one own target row, original
DB timestamp/receipt, stable typed RPC replay and actual local fence/closed routes.
It writes `verified.json`; only then may the caller create the advertised stop
file. Maximum exported lifetime is five minutes. Stop before verification fails.
Every ready/completion/verification/stop and private key file is removed on return.

`VOICE_BOT_ROLE26_SELFTEST=1` exercises this handshake using only the provider's
own already committed control request and terminates immediately after verification.
That opt-in control is not247's production journal interoperability proof.

183 owns these TEST/helper files and provider evidence;44 owns production receiver
and c110 writer cohort;247 owns actual Bot caller/journal;121 owns publication/CI.
Mixed binary1eab old cleanup is unqualified; use actual af1 repaired old/new cohort.
Stops145/172/191/230/372, generic Role core/CURRENT/identity/provider gates remain
unchanged. No production source, migration, generated wire or closed writer is
modified by this fixture.
