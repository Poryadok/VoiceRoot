-- Original private personal actor proof only; no backfill/default type or
-- adoption of old rows. Actual protected writer and retention remain gates.
CREATE TABLE story_lfp_publication_actors (
    publication_operation_id UUID PRIMARY KEY REFERENCES story_lfp_publications(operation_id),
    canonical_actor BYTEA NOT NULL CHECK (octet_length(canonical_actor) BETWEEN 1 AND 2048),
    canonical_offer_state BYTEA NOT NULL CHECK (octet_length(canonical_offer_state) BETWEEN 1 AND 1048576),
    actor_sha256 BYTEA NOT NULL CHECK (octet_length(actor_sha256)=32)
);
CREATE TABLE story_lfp_consent_actors (
    consent_id UUID PRIMARY KEY REFERENCES story_lfp_consents(consent_id),
    canonical_actor BYTEA NOT NULL CHECK (octet_length(canonical_actor) BETWEEN 1 AND 2048),
    actor_sha256 BYTEA NOT NULL CHECK (octet_length(actor_sha256)=32),
    qualified_consent_sha256 BYTEA NOT NULL CHECK (octet_length(qualified_consent_sha256)=32)
);
CREATE TABLE story_lfp_actor_bound_acks (
    consent_id UUID PRIMARY KEY REFERENCES story_lfp_consent_actors(consent_id),
    qualified_consent_sha256 BYTEA NOT NULL CHECK (octet_length(qualified_consent_sha256)=32),
    actor_bound_registration_sha256 BYTEA NOT NULL CHECK (octet_length(actor_bound_registration_sha256)=32)
);
CREATE TRIGGER story_lfp_publication_actors_immutable BEFORE UPDATE ON story_lfp_publication_actors
FOR EACH ROW EXECUTE FUNCTION story_lfp_immutable_update();
CREATE TRIGGER story_lfp_consent_actors_immutable BEFORE UPDATE ON story_lfp_consent_actors
FOR EACH ROW EXECUTE FUNCTION story_lfp_immutable_update();
CREATE TRIGGER story_lfp_actor_bound_acks_immutable BEFORE UPDATE ON story_lfp_actor_bound_acks
FOR EACH ROW EXECUTE FUNCTION story_lfp_immutable_update();
