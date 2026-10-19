# Private automod bootstrap routes

Render `__K_NAMESPACE__` for the selected environment. These dedicated ClusterIP
Services deliberately publish not-ready Pod endpoints. They expose only private
workload-mTLS authority/JWKS ports; ordinary public Services keep their readiness
filtering. Do not route them through Gateway or an external Ingress.

The protected certificate SANs must cover `voice-auth-authority` and
`voice-moderation-authority` respectively (including the exact DNS form used by
each client). Install the complete client CA and issuer/JWKS unions and compose
the existing NetworkPolicies: Auth/Messaging to Moderation9467, Moderation/
Messaging to Auth9093, and the corresponding verified JWKS clients to8443.
Do not replace another feature's CA/issuer map or create an incomplete standalone
isolation policy. These Services alone are not an activation package.

Moderation's main starts its sole local issuer and certificate-only JWKS before
database setup. Its private evidence listener uses
`MODERATION_AUTOMOD_PRINCIPAL_` configuration (default9467), the existing shared
`S2S_JWKS_URLS_JSON` entries for Auth/Messaging, mandatory workload TLS and replay
Redis. The same issuer backs `MODERATION_AUTOMOD_AUTH_` client settings:
`GRPC_ADDR`, `TLS_SERVER_NAME`, `CA_FILE`, `TLS_CERT_FILE`, `TLS_KEY_FILE`.
The Auth connection is lazy; no public-health dependency or reciprocal healthy
wait is added. Partial private configuration fails before an anonymous fallback.

`/health` remains liveness; `/ready` waits for this process's verified P5. Both
staging/prod Moderation readiness probes use `/ready`. P0 alone cannot advance
P1–P5; no terminal/recovery authority, public admission or old expiry worker is
enabled by binding these listeners. The coordinator and complete activation
configuration are required before rollout. Actual restore proof follows
`docs/testing/async-automod-recovery.md`; file integrity never opens readiness.
