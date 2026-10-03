CREATE TABLE federation_nodes (
 id UUID PRIMARY KEY,
 operator_id UUID NOT NULL,
 environment TEXT NOT NULL,
 endpoint TEXT NOT NULL,
 certificate_sha256 TEXT NOT NULL UNIQUE CHECK (length(certificate_sha256)=64),
 status TEXT NOT NULL CHECK (status IN ('pending','active','suspended','defederated')),
 epoch BIGINT NOT NULL DEFAULT 1 CHECK (epoch>0),
 credential_hash TEXT,
 credential_expires_at TIMESTAMPTZ,
 approved_by TEXT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE federation_placements (
 space_id UUID PRIMARY KEY,
 node_id UUID NOT NULL REFERENCES federation_nodes(id),
 generation BIGINT NOT NULL DEFAULT 1 CHECK (generation>0),
 revision BIGINT NOT NULL DEFAULT 0 CHECK (revision>=0),
 snapshot BYTEA,
 snapshot_hash TEXT,
 valid_until TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE federation_lease_nonces (
 node_id UUID NOT NULL REFERENCES federation_nodes(id),
 nonce UUID NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 PRIMARY KEY(node_id,nonce)
);
CREATE INDEX federation_lease_nonces_expiry ON federation_lease_nonces(expires_at);
