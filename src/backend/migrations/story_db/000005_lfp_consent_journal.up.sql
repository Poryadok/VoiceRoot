-- Private immutable LFP evidence. No production handler is installed here.
-- Actual external controller/custody and account erasure remain gates. This
-- local head/barrier supplements that authority; it cannot create authority.
CREATE TABLE story_lfp_admission_head (
    singleton BOOLEAN PRIMARY KEY CHECK (singleton),
    environment_id UUID NOT NULL,
    installation_id UUID NOT NULL,
    authority_incarnation UUID NOT NULL,
    database_binding_sha256 BYTEA NOT NULL CHECK (octet_length(database_binding_sha256)=32),
    phase TEXT NOT NULL CHECK (phase IN ('closed','open'))
);
-- No initial row: only the actual installation controller may prepare this.
CREATE TABLE story_lfp_offers (
    story_id UUID NOT NULL,
    revision BIGINT NOT NULL CHECK (revision > 0),
    incarnation UUID NOT NULL,
    author_profile_id UUID NOT NULL,
    criteria_sha256 BYTEA NOT NULL CHECK (octet_length(criteria_sha256)=32),
    payload_sha256 BYTEA NOT NULL CHECK (octet_length(payload_sha256)=32),
    state_sha256 BYTEA NOT NULL CHECK (octet_length(state_sha256)=32),
    expires_at_millis BIGINT NOT NULL CHECK (expires_at_millis > 0),
    environment_id UUID NOT NULL,
    installation_id UUID NOT NULL,
    authority_incarnation UUID NOT NULL,
    database_binding_sha256 BYTEA NOT NULL CHECK (octet_length(database_binding_sha256)=32),
    PRIMARY KEY (story_id,incarnation,revision)
);
CREATE TABLE story_lfp_offer_heads (
    story_id UUID PRIMARY KEY,
    incarnation UUID NOT NULL,
    revision BIGINT NOT NULL CHECK (revision > 0),
    FOREIGN KEY (story_id,incarnation,revision) REFERENCES story_lfp_offers(story_id,incarnation,revision)
);
CREATE TABLE story_lfp_publications (
    operation_id UUID PRIMARY KEY,
    story_id UUID NOT NULL,
    incarnation UUID NOT NULL,
    revision BIGINT NOT NULL CHECK (revision > 0),
    author_profile_id UUID NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256)=32),
    FOREIGN KEY (story_id,incarnation,revision) REFERENCES story_lfp_offers(story_id,incarnation,revision)
);

-- No cascading FK to stories: archive cleanup cannot silently destroy private
-- response/recovery evidence. The owning erasure worker must explicitly account
-- for these PERSONAL rows; this migration grants no retention exemption.
CREATE TABLE story_lfp_consents (
    response_operation_id UUID PRIMARY KEY,
    consent_id UUID NOT NULL UNIQUE,
    story_id UUID NOT NULL,
    incarnation UUID NOT NULL,
    offer_revision BIGINT NOT NULL CHECK (offer_revision > 0),
    responder_account_id UUID NOT NULL,
    responder_profile_id UUID NOT NULL,
    canonical_consent BYTEA NOT NULL CHECK (octet_length(canonical_consent) BETWEEN 1 AND 2048),
    consent_sha256 BYTEA NOT NULL CHECK (octet_length(consent_sha256)=32),
    -- Immutable FK retains the EXACT full offer origin with each consent. It
    -- cannot point to a new cohort by a latest-head/profile/digest lookup.
    FOREIGN KEY (story_id,incarnation,offer_revision) REFERENCES story_lfp_offers(story_id,incarnation,revision)
);
CREATE INDEX story_lfp_consents_account ON story_lfp_consents(responder_account_id,response_operation_id);
CREATE TABLE story_lfp_registration_acks (
    consent_id UUID PRIMARY KEY REFERENCES story_lfp_consents(consent_id),
    canonical_registration BYTEA NOT NULL CHECK (octet_length(canonical_registration) BETWEEN 1 AND 2048),
    registration_sha256 BYTEA NOT NULL CHECK (octet_length(registration_sha256)=32)
);

CREATE FUNCTION story_lfp_immutable_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'immutable Story LFP evidence cannot be updated';
END;
$$;
CREATE TRIGGER story_lfp_offers_immutable BEFORE UPDATE ON story_lfp_offers
FOR EACH ROW EXECUTE FUNCTION story_lfp_immutable_update();
CREATE TRIGGER story_lfp_consents_immutable BEFORE UPDATE ON story_lfp_consents
FOR EACH ROW EXECUTE FUNCTION story_lfp_immutable_update();
CREATE TRIGGER story_lfp_publications_immutable BEFORE UPDATE ON story_lfp_publications
FOR EACH ROW EXECUTE FUNCTION story_lfp_immutable_update();
CREATE TRIGGER story_lfp_registration_acks_immutable BEFORE UPDATE ON story_lfp_registration_acks
FOR EACH ROW EXECUTE FUNCTION story_lfp_immutable_update();
