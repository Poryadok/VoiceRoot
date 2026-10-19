# Report threshold producer activation templates

These files prepare BE-046's User/Auth producers for composition with BE-045's
shared Moderation runtime. They are not included by the default deployment.
The four `*.patch.yaml` files are strategic merge patches, not standalone
Deployments/Services. Render `__K_NAMESPACE__`, `__INGRESS_CLASS__` and
`__USER_REPORT_JWKS_HOST__` for the actual environment. Preserve the complete
base documents and all unrelated ports, environment, volumes and trust entries.

User exposes the exact Moderation-only RPC on 9466; Auth exposes the exact
User-only classifier on 9465. Both require mTLS, request-bound service credentials
and shared Redis replay admission. Their ordinary RPC surfaces remain excluded.
The exact TLS Ingress route publishes only User's current/next public JWKS from
the existing HTTP endpoint. Auth uses
`https://__USER_REPORT_JWKS_HOST__/.well-known/principal-jwks.json` and validates
its real certificate chain/hostname. Never substitute plain HTTP or disable TLS.

Prepare these existing external objects before composing the patches:

- `voice-user-report-threshold-tls`: `tls.crt` and `tls.key`, valid for
  `voice-user`, with server and client authentication usages. User also uses this
  identity to call Auth and fetch Moderation's protected JWKS.
- `voice-auth-report-audience-tls`: server `tls.crt` and `tls.key`, valid for
  `voice-auth`. Its client CA must trust User's client identity.
- `voice-report-threshold-trust`: `service-ca.pem` for Auth's private listener,
  `client-ca.pem` for the corresponding trusted callers, `jwks-ca.pem` containing
  the required Moderation/User HTTPS trust chains **and every CA root needed by
  the workload's existing shared JWKS consumers**, and `redis-password` matching
  the actual replay Redis instance (explicitly empty only for an unauthenticated
  isolated instance). Use separate trust objects per workload if CA scopes differ.
- `voice-user-report-threshold-signing`: distinct RSA keys of at least 2048 bits
  in `current.pem`/`next.pem`, and distinct `current-kid`/`next-kid`. Rotate using
  the accepted current/next overlap; no generated keys are committed here.
- `voice-user-report-threshold-jwks-tls`: Ingress TLS identity for the rendered
  JWKS host, with its trusted CA available to Auth.
- ConfigMap `voice-report-threshold-authorities` with
  `USER_S2S_JWKS_URLS_JSON` and `AUTH_S2S_JWKS_URLS_JSON`: each value is the
  **complete currently required issuer map**, not a replacement containing only
  the new issuer. User's map adds `moderation` with BE-045's exact cert-only mTLS
  HTTPS JWKS URL
  `https://voice-moderation-authority:8443/internal/principals/jwks`;
  Auth's map adds `user` with the exact HTTPS endpoint above. Preserve all other
  callers required by the assembled application. Never infer issuer keys from a
  client request, ordinary metadata or target profile.

BE-045 owns this one Moderation signer/JWKS listener and its
`MODERATION_PRINCIPAL_{JWKS_LISTEN,TLS_CERT_FILE,TLS_KEY_FILE,CLIENT_CA_FILE}`
configuration. The key endpoint starts before domain/restore readiness and has
no JWT/floor lookup dependency, so owner verifiers can bootstrap without a trust
cycle. User's configured JWKS client identity and the union of allowed caller CAs
must match that listener. Do not retain the temporary report-targets signer or
publish a second set of Moderation keys.

Compose NetworkPolicies against the exact current base before activation:
permit `app=voice-moderation` to `app=voice-user` TCP 9466 and `app=voice-user`
to `app=voice-auth` TCP 9465, within the selected namespace. Do not add either
private port to a wildcard caller rule. Kubernetes policies are additive;
reconcile the union of current policies, ingress-controller access to User HTTP,
existing service ports, egress DNS/Redis/JWKS/owner RPCs and actual labels. An
isolating policy containing only the new ports would break existing traffic, so
no incomplete standalone NetworkPolicy is supplied. These patches do not prove
the running network isolation or complete activation topology.

Apply migrations and register BE-046's purge/closure/VALID hooks before BE-045's
terminal restore/bootstrap barrier releases workers. Shared Moderation owns its
single signer/JWKS/Auth client and
`MODERATION_REPORT_THRESHOLD_{USER_ADDR,TLS_SERVER_NAME,CA_FILE,TLS_CERT_FILE,TLS_KEY_FILE}`;
set User address to `voice-user:9466` and validate server name `voice-user`.
Do not activate a parallel legacy threshold or a second restriction writer.

Before a live rollout, assemble/render these patches against the exact immutable
candidate, validate all references and probe real TLS/JWKS/replay negatives.
Acquire the actual TT DEV/STAGING resource window and preserve updater locks.
This preparation does not claim deployment, stand authority or acceptance.
