# Private native Role25 / Space44 fixture

Consumer: existing247/BE170 integration, in a caller-owned disposable Linux Go
runner. No production rollout, shared stand or live resource handoff. The
fixture runs actual independent Role/Space PostgreSQL, official migrators,
Redis, mTLS/JWKS and protected handlers. Space source is fixed to
`a36cc5e504fab412f6d48dd5233ed199b9b5d243`. Administrative cutover uses its real
`InitializeNativeMembership` API; serving uses an independent nonowner LOGIN.

Prepare the exact Space archive from Git objects, outside any foreign worktree:

```powershell
rtk git archive --format=tar -o tmp/swarm/native-space-a36cc5e504fab412f6d48dd5233ed199b9b5d243.tar a36cc5e504fab412f6d48dd5233ed199b9b5d243 src/backend
```

Use the selected immutable Role source as a separate tree inside your own Go1.26
Linux runner. Existing private Docker access and module caches are needed for
testcontainers; no host test executable/listener or fixed host port is required.
`ROLE_NATIVE_SPACE_ARCHIVE_DIR` may point to your read-only archive directory;
otherwise the build reads `tmp/swarm` below the selected source root. Never use a
mutable peer worktree or swap in another catalog/DTO.

From the Role module, in that private runner:

```sh
export ROLE_NATIVE_FIXTURE_EXPORT_DIR=/tmp/role-native-fixture-my-owned-run
export ROLE_NATIVE_FIXTURE_ROLE_SOURCE=YOUR_EXACT_40_HEX_ROLE_COMMIT
export ROLE_NATIVE_SPACE_ARCHIVE_DIR=/absolute/path/to/archives
sh testdata/run_native_role_fixture.sh
```

The export directory must be fresh. Only after the entire initialized44 positive
protocol succeeds does the process atomically publish `ready.json`. It contains
the actual Role loopback address, CA/server name/client certificate/key paths,
separate Bot-only fixture signer/key ID/JWKS URL, actual owner profile/account
and Space UUIDs, qualified Space application DSN and actual distinct PostgreSQL
system identifiers. The owner account is a controlled Role fixture identity,
not an Auth attestation. No bearer, verified private capability or readiness
token is exported. Fixture keys/DSN are disposable test data, never deployment
configuration; do not print or attach the full manifest as product proof.

Run your consumer inside the **same** private runner/network namespace. A real
signed smoke client is available:

```sh
go run ./testdata/native_role_probe /tmp/role-native-fixture-my-owned-run/ready.json
```

It checks actual2s Bot owner permission, replay rejection, actual cleanup
validator and closed generic Bot authority. It does not invent installation
evidence:247 must use its real initiating decision, native Space producer,
frozen cleanup journal/receipt validation and independent Gateway/Auth proof.
The application DSN allows that separately owned native producer to use the
same genuine Space; it is not an administrator credential.

After your bounded checks, create the manifest's `stop_file`. The process
removes ready/private-key files and normal test cleanup stops its own listeners,
children, Redis/PostgreSQL. Export expires after at most five minutes; failure to
stop is a failed run with teardown, never an unlimited lease. The longer fixture
lifecycle does not extend5s observations,2s consumer calls or JWT lifetimes.
The caller owns all processes; do not transfer live handles or claim public
BE170/all-writer/current core readiness from this component.

Space44 Add/Cancel Bot installation writers are a separate186-owned readiness
gate; this fixture does not turn inherited42 writer behavior into a44 positive.
The actual Role decision/cleanup serving layer can be exercised independently.

## Opt-in nonzero cleanup contribution (BE170c81)

The existing consumer247 needs actual effects selected by its own persisted Bot
installation, not only a zero-count signed transport response. This test-only
contribution uses the same Role25/Space44 provider and five-minute lifetime.
Production, DDL, public APIs, current46 and the stopped identity/core work are
excluded. Developer183 owns only the fixture;247 owns the real Bot journal and
signed caller. Implementation plan: reproduce the missing seed handshake in
the actual private PostgreSQL fixture, add atomic seed/readback and independently
checked committed postconditions, check default mode and invalid tuples, then
freeze source/proof for independent review and247's own actual consumer run.

Set `ROLE_NATIVE_FIXTURE_BOT_CLEANUP=1` before starting the existing runner.
After `ready.json`, write `seed.request.json` atomically with schema version1:

```json
{"schema_version":1,"bot_id":"UUID","actor_profile_id":"UUID","space_id":"UUID","installation_id":"UUID","install_operation_id":"UUID","generation":1}
```

Use the actual Bot database tuple. IDs must be canonical nonzero UUIDs and Space
must equal this fixture. Seed files are setup/synchronization, never capabilities.
The provider accepts one immutable seed, commits fixture custom roles and raw
legacy assignment rows without inventing membership contributions or receipts,
and publishes `seed.ready.json` only after its own database readback. The reply
identifies the created/custom control role and human control profile, plus exact
expected cleanup counts. System Member and existing Owner assignments are controls.

After your production signed cleanup worker has validated/persisted its actual
receipt, write `postcondition.request.json`: the same seed fields plus canonical
`operation_id`, `receipt_id`, `decision_id`. The provider loads the committed
request/response and request digest from its own Role database, checks exact
native installation binding, original nonzero counts, removed target rows,
preserved controls and one immutable operation, then publishes
`postcondition.ready.json`. A synchronization file cannot stand in for this
database evidence. A changed seed, wrong tuple or early stop fails the fixture;
missing phases expire as a failed bounded run. Only after the reply create
`stop`. Default mode does not seed or require these files.

Provider self-check `TestNativeFixtureBotCleanupNonzero` uses the actual signed
Role client and independent database checks. It is not evidence that247's Bot
selector/journal chose the correct installation; that consumer run stays separate.
