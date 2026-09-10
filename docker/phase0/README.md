# Phase0 local fixture

This opt-in Compose overlay prepares the signed-principal test environment. It
requires the Space issuer/caller, Role private listener, and Auth private listener
changes to be integrated before runtime acceptance. Rendering this overlay proves
configuration only; it does not prove authenticated RPCs or a live deployment.

From the repository root, with Python 3, OpenSSL, a JDK `keytool`, and Docker
Compose installed:

```bash
mkdir -p tmp/phase0
python3 scripts/dev/generate-phase0-fixture.py tmp/phase0/credentials
export PHASE0_FIXTURE_DIR="$(pwd)/tmp/phase0/credentials"
docker compose -f docker-compose.yml -f docker-compose.phase0.yml --profile app config --quiet
make phase0-fixture-test
```

The destination must not already exist. The generator creates independent
Gateway and Space current/next PKCS8 RSA keys, a short-lived fixture CA, three
server certificates with DNS SANs, and a Java PKCS12 truststore. It deletes the
CA signing key after issuing certificates. All certificates expire in two days;
generate a fresh directory for a new run. `phase0-fixture` is the public test-only
truststore password. Never use this material or overlay for staging/production.

The overlay runs its five affected containers as root solely to read restrictive
host-owned fixture files (directories 0700, private files 0600). It does not
loosen host permissions. Every credential mount is read-only and service-scoped;
the JWKS proxy sees only its own TLS leaf certificate/key, never issuer signing
keys. Use a separate disposable Compose project when the runtime proof is added.

| Connection | Endpoint |
| --- | --- |
| Space to Role ownership | `role:9091`, validated TLS with fixture CA |
| Auth proof private surface | `auth:9091`, validated TLS with fixture CA |
| Role and Auth read Space JWKS | `https://phase0-jwks:8443/space/jwks.json` |
| Auth reads Gateway JWKS | `https://phase0-jwks:8443/gateway/jwks.json` |

Ordinary gRPC endpoints on 9090 are retained. No new private port is published to
the host. The proxy accepts GET only on the two exact public-key routes and uses
lazy Docker DNS resolution. Consumers fetch JWKS lazily, avoiding startup cycles
with issuers that depend on those consumers. Process `/health` is not evidence
that a protected principal can be verified.

Auth uses a fixture-only JVM truststore containing the fixture CA, and its
existing Redis configuration for replay/epoch checks. Role gets its dedicated
replay Redis address. Pending Space-to-Auth/Gateway-to-Auth proof client settings
are intentionally absent until their runtime contracts exist.

The offline tests generate fresh credentials, verify keys/certificates/SANs and
the JVM truststore, inject crypto-tool failure after key creation to check
cleanup, and render merged Compose JSON without starting containers. Linux CI
also verifies POSIX private-file modes. `ci-script-tests` includes this target.
The later combined-runtime proof must check valid signed ownership requests,
wrong CA/SAN/plaintext rejection, replay/epoch denial, and ordinary 9090 traffic.
