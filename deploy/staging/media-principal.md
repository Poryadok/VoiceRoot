# Staging media principal projection

This projects the implemented contract from `docker-compose.voice-space-media.yml` and [Voice service](../../docs/microservices/voice-service.md) into ordinary staging manifests. It enables Gateway→Voice user principals on9093, Voice→Auth session-floor checks on9092, Voice→Role grants on9091, and Voice HTTPS public JWKS on8443. Legacy9090 listeners remain available. It does not activate the separate Space lifecycle listener or claim Space/Game Integration private API readiness.

Provision these dedicated Secrets in `voice-staging` before FULL/app-only preparation. Generate real staging keys in a root-owned0700 directory with umask077; never use Compose fixture keys or Auth JWT signing keys. Ordinary deploy preserves existing Secret identities and bytes; it does not generate or rotate credentials. The source-plan prerequisite checks every listed key before fencing and repeats authority binding before apply.

| Secret | Keys and meaning |
| --- | --- |
| `voice-media-ca` | `ca.crt`: dedicated private media CA only; `jwks-ca.crt`: public-root trust bundle plus media CA for HTTPS issuer fetches |
| `voice-media-jvm-trust` | binary `truststore.p12`: PKCS12 trusted-certificate store cloned from the exact Auth runtime JDK default CA set, with media CA added; no private keys; public integrity password `changeit` |
| `voice-auth-media-server-tls` | `tls.crt`, `tls.key`: server-auth certificate, DNS SAN `voice-auth` |
| `voice-role-media-server-tls` | `tls.crt`, `tls.key`: server-auth certificate, DNS SAN `voice-role` |
| `voice-voice-media-server-tls` | `tls.crt`, `tls.key`: server-auth certificate, DNS SAN `voice-voice` |
| `voice-voice-media-jwks-tls` | `tls.crt`, `tls.key`: distinct server-auth certificate, DNS SAN `voice-voice` |
| `voice-voice-media-client-tls` | `tls.crt`, `tls.key`: distinct Voice client-auth identity for Auth/Role and HTTPS issuer requests |
| `voice-gateway-media-client-tls` | `tls.crt`, `tls.key`: distinct Gateway client-auth identity for Voice |
| `voice-voice-principal-signing` | `current.pem`, `next.pem`: distinct RSA PKCS8 private keys, at least2048 bits; KIDs `current`/`next` |
| `voice-gateway-principal-signing` | `current.pem`, `next.pem`, `active-kid`: distinct RSA PKCS8 keys; `active-kid` is `current` or `next` and is projected only into env, never into the key directory |
| `voice-media-resolver` | `token`: random shared secret projected identically into Space and Voice |

Use separate private keys for every TLS identity and signer. Keep CA private keys outside Kubernetes. Certificates need non-CA constraints, the stated SAN/EKU, valid chains and validity windows; server and client roles must not be interchangeable. Secret volumes are read-only0440 with pod fsGroup65532. Only the application container mounts its keys; the NATS sidecar receives no media material.

Auth uses the JDK default HTTPS client for JWKS. Setting `javax.net.ssl.trustStore` replaces its global public trust, so a private-CA-only store is invalid: clone the exact deployed JDK's default trusted certificates, add the media CA, and verify the original trusted-certificate fingerprint set is retained. This store contains public certificates only: its integrity password `changeit` is deliberately non-secret and may appear in JVM startup diagnostics. Never interpolate a confidential password into `JAVA_TOOL_OPTIONS`, which the JVM echoes. File provenance and the bound Secret provide integrity. The manifest appends truststore options to both existing heap-percentage flags. The Go issuer-fetch bundle likewise retains public roots; public roots must never be added to inbound mTLS client trust. Verify a valid private Voice JWKS chain and the actual public Gateway HTTPS route, and reject an unrelated CA, before accepting provisioning. Key-presence preflight is not cryptographic proof.

Gateway publishes existing public keys at `https://<configured gateway ingress host>/.well-known/voice-principal-jwks.json`; there is no new native Gateway TLS listener. The endpoint must return the actual Gateway signer keys directly over trusted HTTPS, without redirects. Auth's exact issuer map includes Gateway, Space and Voice; Role's map includes Space, Game Integration and Voice. Initial issuer fetch remains lazy to avoid startup cycles. Space/Game Integration URLs remain fail-closed and unverified until their independent listener/trust projection is activated; never point those issuer names at another service's keys.

Existing source NetworkPolicies do not isolate Auth, Role, Voice or Gateway and define no general egress deny. Before rollout, inspect actual cluster policies: require Voice→Auth9092, Voice→Role9091, Gateway→Voice9093, Auth/Role→Voice8443, Redis6379 and DNS, plus issuer HTTPS443 through the configured ingress path. Preserve ordinary8080 health and9090 traffic. Source policy inspection alone is not live connectivity proof; Space's current policy does not admit a new8443 issuer listener.

After deployment, require actual mTLS/principal/session-floor calls and current-build application acceptance. A credential-presence check or healthy process alone does not prove protected media authorization. Keep certificate/key values, passwords, JWTs and private Secret JSON out of logs and retained public evidence.
