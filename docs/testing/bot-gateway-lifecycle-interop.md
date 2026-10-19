# Bot lifecycle HTTP interop fixture

This explicitly selected fixture verifies the existing DeleteBot path through
real HTTP, Gateway access-token validation, the protected initiating client,
Bot mTLS/JWKS verification, a fresh Java Auth floor read and PostgreSQL. It uses
an empty installation set; it does not qualify remote cleanup participants or
native install/cancel/reinstall. Bot and Gateway replay/floor Redis fixtures are
miniredis. Their production Redis clients and commands execute, but this is not
an actual Redis server qualification.

Run only inside an owned private Linux environment. No host port, shared stand,
production identity or source mutation is required. Start the portable Java
`AuthBotSessionFloorGrpcTest` interop main first and copy its fresh test-only
certificate/key/ready files into that environment. Set these environment values
for the Bot test:

```sh
export BOT_AUTH_FLOOR_INTEROP_DIR=/tmp/auth-http-fixture
export BOT_AUTH_FLOOR_INTEROP_HOST=auth-floor
export BOT_AUTH_FLOOR_INTEROP_EXPECT=9
export BOT_GATEWAY_HTTP_FIXTURE_DIR=/tmp/bot-http-fixture
cd src/backend/bot
go test -race -count=1 -timeout=210s \
  -run '^TestBotGatewayHTTPFixture$' ./internal/grpcsvc
```

The Bot test creates its own real PostgreSQL database, starts the production
protected verifier and handler on an ephemeral container-loopback port, and
atomically writes `ready.json`. While it runs, select the Gateway companion in
the same Linux container, with the same fixture directory:

```sh
cd src/backend/gateway
go test -race -count=1 -timeout=30s \
  -run '^TestBotLifecycleActualHTTPToProtectedBot$' .
```

The companion loads only the fresh test key, configures the real protected
transport factory, starts an HTTP Gateway, rejects duplicate Bearer and stale
access-token cases, then sends the accepted request with spoofed raw identity
headers. It writes `done`; the Bot fixture verifies database postconditions and
exits. The fixture has a 180-second lifetime and does not create a background
production service. Without explicit fixture environment both tests skip.

For each additional case use a fresh Bot fixture directory. Set only the Java
test fixture's retained-floor control file to `20` or `0`, and set
`BOT_AUTH_FLOOR_INTEROP_EXPECT` to the same value before starting the Bot test.
Raised floor returns HTTP401; unavailable floor returns HTTP503. Both cases
must retain Bot live status and create no deletion decision. Floor9 returns
HTTP204, disables the Bot, commits one complete root and retains the verified
account/profile/session epoch in its immutable decision. These operations do
not write the production Auth floor or identity schema.

The ordinary Bot lifecycle listener cut and the full BE-170 participants,
receipts, restart/retry, retention and purge criteria remain separate gates.
