# Durable Bot producer and native Space interop

This explicit fixture verifies Bot PostgreSQL installation intents and the real
signed private Add/Remove transport against the production Space receiver,
official Space migrations, PostgreSQL and an actual Redis server. It does not
replace a Role permission provider, because it selects the durable producer and
private RPC seam directly. Public Gateway install/uninstall admission and full
DeleteBot participants are separate qualification gates.

Use two immutable source builds in one owned private Linux environment, with
its own private Docker engine for testcontainers and no host ports. Build both
selected test packages before starting the bounded receiver. The Space source
must include the native42 receiver, its unknown-source cancellation correction,
and `native_bot_transport_test.go`; an old raw membership server is incompatible.

Start the Space receiver fixture from its immutable build:

```sh
export VOICE_BOT_NATIVE_HANDOFF_DIR=/tmp/native-handoff
export VOICE_BOT_NATIVE_ADVERTISED_HOST=127.0.0.1
cd /space-build/src/backend/space
go test -race -count=1 -timeout=150s \
  -run '^TestNativeBotProtectedTransport$' ./internal/grpcsvc
```

After its atomic `ready.json` appears, run the producer companion from the
separate Bot source build in the same container:

```sh
export VOICE_BOT_NATIVE_HANDOFF_DIR=/tmp/native-handoff
cd /bot-build/src/backend/bot
go test -race -count=1 -timeout=60s \
  -run '^TestNativeProducerActualSpacePersistenceCancelAndReinstall$' \
  ./internal/grpcsvc
```

The fixture directory contains newly generated test-only signing and transport
credentials. Keep it private and exclude credentials from proof packets. The
Space fixture terminates after `complete.json` and its database assertions, or
after 90 seconds without a companion. Both fixtures clean up their own database
and Redis containers. The producer test skips without an explicit fixture
directory.

The producer asserts committed installation identity before Add, retry after a
lost acknowledgement using a new store object and stable operation bytes with
a fresh transport JTI, cancellation and local revocation, stale Add rejection,
reinstall generation2, safe old cancellation replay, and another Bot's
cancel-before-Add tombstone. Space independently checks the final kind6 source,
its exact publication operation, generation2 and retired predecessor floor1.
No raw Chat.AddMembers call, human default Role grant or legacy-source adoption
is used or qualified by this fixture.
# Protected Chat observation lower seam

The optional Chat fixture and Bot caller run as separate processes inside one
owned disposable Linux runner. They use real Chat and Bot PostgreSQL databases,
real Redis replay, mTLS and HTTPS JWKS. No host port is exposed. Use a fresh
private directory for every invocation, and preserve raw logs without exporting
its generated test keys or `ready.json`.

Start the provider from `src/backend/chat`:

```sh
VOICE_BOT_CHAT_HANDOFF_DIR=/tmp/fresh-bot-chat-handoff go test ./internal/gisprincipal \
  -run '^TestBotScopeAuthorityRealNativeTLSJWKSRedisAndRetiredHead$' \
  -race -count=1 -timeout=210s
```

Concurrently run the consumer from `src/backend/bot` in the same runner:

```sh
VOICE_BOT_CHAT_HANDOFF_DIR=/tmp/fresh-bot-chat-handoff go test ./internal/grpcsvc \
  -run '^TestBotChatSourceCrossProcessPersistenceAndRetiredRetry$' \
  -race -count=1 -timeout=110s
```

The production Bot Chat caller signs its exact request, validates the actual
response and commits the source bytes before any Space Add. The provider then
deletes its actual Chat row, checks retained incarnation/revision, and the caller
rejects a new admission while recovering its original accepted intent. The
fixture expires after 150 seconds and removes its exported readiness/key files
on teardown. This proves the lower Chat source seam only, not public Role or
Gateway admission, native Space installation, cleanup or whole BE-170 readiness.

For a later shared-issuer fixture, optional `VOICE_BOT_CHAT_SIGNER_KEY_FILE` and
`VOICE_BOT_CHAT_SIGNER_KEY_ID` select the caller-owned test key. The external
`VOICE_BOT_CHAT_JWKS_URL`, `VOICE_BOT_CHAT_JWKS_CA_FILE`,
`VOICE_BOT_CHAT_JWKS_CLIENT_CERT_FILE`, `VOICE_BOT_CHAT_JWKS_CLIENT_KEY_FILE`
tuple is all-or-none. `VOICE_BOT_CHAT_SPACE_ID` selects the actual fixture Space.
The external shared-issuer branch requires its own execution proof.

