# Staging NATS proof credential

Run this command only on a trusted Linux host. Issuance and preflight both fail
closed on other operating systems because `0600` does not establish equivalent
Windows ACL protection. The APP account seed, NATS Secret bundle, and credential
input must be regular files with mode `0400` or `0600`. The fresh output must be
inside an existing `0700` directory and is created exclusively with mode `0600`.

From `src/backend/pkg`, issue a temporary proof credential into a new file:

```sh
go run ./cmd/nats-proof-credential \
  --namespace voice-staging --generation rYYYYMMDDsuffix \
  --account-seed /protected/app-account.seed \
  --bundle /protected/versioned-secrets.json \
  --output /protected/proof.creds --ttl 60m
```

The default TTL is 60 minutes; the maximum is two hours. The command prints
only an expiry timestamp. Before changing the active NATS generation, validate
the credential against the exact generation bundle:

```sh
go run ./cmd/nats-proof-credential \
  --namespace voice-staging --generation rYYYYMMDDsuffix \
  --bundle /protected/versioned-secrets.json \
  --credential /protected/proof.creds --check-min-validity 30m
```

Preflight verifies the APP signature, credential seed and JWT subject, lifetime,
and the exact bounded publish/subscribe grants. It does not contact NATS or
print credential material.
