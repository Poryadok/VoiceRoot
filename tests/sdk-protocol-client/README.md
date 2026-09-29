# SDK protocol test client

This is an internal conformance client for Auth's T15 device-key routes. It is
not a product SDK and is not included in a release artifact. Each `DeviceKey`
is an independent in-memory P-256 key; public JWK output and RFC 7638
thumbprints never expose the private scalar.

The client signs strict RFC 8785 protocol claim objects as ES256 compact JWS and
calls only the frozen Auth SDK device-key routes for challenge, rotate, recover,
and revoke. HTTP is HTTPS-only except loopback test servers, and redirects are
not followed so provider credentials or proofs cannot be forwarded to another
host. Protocol proof values and provider tokens are caller-owned and are not
persisted by this package.

Run the isolated acceptance package with:

```powershell
rtk make sdk-protocol-client-acceptance
```

Tests use synthetic route fixtures and standard-library signature verification.
They do not make Google/provider calls, deploy Auth, or constitute device,
production-key, or live API acceptance.
