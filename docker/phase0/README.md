# Phase0 local fixture

This opt-in Compose overlay prepares the signed-principal test environment for
Space ownership, GIS game-session grant mutations, Voice grant checks, and the
existing Auth proof listener. Fixture generation and offline tests validate the
credentials and Compose wiring; they do not prove a live deployment.

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
Gateway, Space, GIS, and Voice current/next PKCS8 RSA keys, a short-lived server
CA plus a separate GIS-only client CA, seven server certificates with DNS SANs,
five distinct client-auth certificates for Space, GIS-to-Role, Voice-to-Role,
GIS-to-Chat, and GIS-to-Voice, and a Java PKCS12 truststore. It deletes both CA
signing keys after issuance. All certificates expire
in two days; generate a fresh directory for a new run. `phase0-fixture` is the
public test-only truststore password. Never use this material or overlay for
staging/production.

The overlay runs its seven affected containers as root solely to read restrictive
host-owned fixture files (directories 0700, private files 0600). It does not
loosen host permissions. Every credential mount is read-only and service-scoped.
The JWKS proxy sees only its own TLS leaf certificate/key; GIS and Voice each
serve their own JWKS over a dedicated TLS listener. No verifier receives an
issuer signing key.

| Connection | Endpoint |
| --- | --- |
| Space to Role ownership | `role:9091`, mutual TLS with the fixture CA |
| GIS Apply/Revoke to Role | `role:9091`, mutual TLS as `gameintegration` |
| Voice grant check to Role | `role:9091`, mutual TLS as `voice` |
| GIS provisioning to Chat | `chat:9091`, GIS-only mutual TLS |
| GIS provisioning/close to Voice | `voice:9091`, GIS-only mutual TLS |
| Role reads Space JWKS | `https://phase0-jwks:8443/space/jwks.json` |
| Role reads GIS JWKS | `https://gameintegration:8443/internal/v1/principal/jwks.json` |
| Role reads Voice JWKS | `https://voice:8443/internal/v1/principal/jwks.json` |
| Voice reads GIS JWKS | `https://gameintegration:8443/internal/v1/principal/jwks.json` |
| Auth proof private surface | `auth:9091`, validated TLS with fixture CA |
| Auth reads Gateway JWKS | `https://phase0-jwks:8443/gateway/jwks.json` |

Ordinary gRPC endpoints on 9090 are retained. Private gRPC and HTTPS JWKS ports
are not published to the host. The proxy accepts GET only on the Gateway and
Space public-key routes and uses lazy Docker DNS resolution. Role and Voice fetch
GIS/Voice JWKS lazily; process `/health` is not evidence that a protected
principal can be verified.

Auth uses a fixture-only JVM truststore containing the fixture CA, and its
existing Redis configuration for replay/epoch checks. Role receives the fixture
CA as both its client-certificate trust bundle and the trust root for its three
HTTPS JWKS sources, plus the dedicated replay Redis address. GIS and Voice receive
distinct Role client identities and distinct service-signing keys. Pending
Space-to-Auth/Gateway-to-Auth proof client settings remain outside this fixture.

The offline tests generate fresh credentials, verify signing key independence,
certificate purposes/SANs and the JVM truststore, inject crypto-tool failure
after key creation to check cleanup, and render merged Compose JSON without
starting containers. Linux CI also verifies POSIX private-file modes.
`ci-script-tests` includes this target. A combined runtime proof should check
valid Space/GIS/Voice calls, wrong CA/SAN/plaintext rejection, replay denial,
Role Revoke denial for Voice, and ordinary 9090 traffic.
