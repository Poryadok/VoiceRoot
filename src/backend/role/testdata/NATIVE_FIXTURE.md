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
