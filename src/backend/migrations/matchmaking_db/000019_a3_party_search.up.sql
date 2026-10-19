BEGIN;

-- NULL preserves the independent existing LFP path and historical parties.
-- Public A3 search fills the complete immutable source binding atomically with
-- every member's search session and its durable queue projection intent.
ALTER TABLE parties
    ADD COLUMN search_operation_id UUID UNIQUE,
    ADD COLUMN search_request_sha256 BYTEA,
    ADD COLUMN initiator_account_id UUID,
    ADD COLUMN initiator_session_epoch BIGINT,
    ADD COLUMN voice_snapshot_protocol_version SMALLINT,
    ADD COLUMN voice_source_kind TEXT,
    ADD COLUMN voice_source_room_id UUID,
    ADD COLUMN voice_source_roster_version BIGINT,
    ADD COLUMN voice_member_manifest_sha256 BYTEA,
    ADD CONSTRAINT parties_voice_source_binding CHECK (
        (search_operation_id IS NULL AND search_request_sha256 IS NULL
         AND initiator_account_id IS NULL AND initiator_session_epoch IS NULL
         AND voice_snapshot_protocol_version IS NULL AND voice_source_kind IS NULL
         AND voice_source_room_id IS NULL AND voice_source_roster_version IS NULL
         AND voice_member_manifest_sha256 IS NULL)
        OR
        (search_operation_id IS NOT NULL
         AND search_operation_id <> '00000000-0000-0000-0000-000000000000'::UUID
         AND search_request_sha256 IS NOT NULL AND octet_length(search_request_sha256)=32
         AND initiator_account_id IS NOT NULL
         AND initiator_account_id <> '00000000-0000-0000-0000-000000000000'::UUID
         AND initiator_session_epoch IS NOT NULL AND initiator_session_epoch>0
         AND voice_snapshot_protocol_version IS NOT NULL AND voice_snapshot_protocol_version=1
         AND voice_source_kind IS NOT NULL
         AND voice_source_roster_version IS NOT NULL
         AND voice_member_manifest_sha256 IS NOT NULL AND octet_length(voice_member_manifest_sha256)=32
         AND jsonb_typeof(member_profile_ids)='array' AND jsonb_array_length(member_profile_ids)>0
         AND leader_profile_id <> '00000000-0000-0000-0000-000000000000'::UUID
         AND member_profile_ids ? leader_profile_id::text
         AND ((voice_source_kind='SOLO' AND voice_source_room_id IS NULL
               AND voice_source_roster_version=0 AND jsonb_array_length(member_profile_ids)=1)
              OR (voice_source_kind='VOICE_ROSTER' AND voice_source_room_id IS NOT NULL
                  AND voice_source_room_id <> '00000000-0000-0000-0000-000000000000'::UUID
                  AND voice_source_roster_version>0)))
    );

CREATE FUNCTION matchmaking_party_source_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF (OLD.search_operation_id IS NULL AND NEW.search_operation_id IS NOT NULL)
       OR (OLD.search_operation_id IS NOT NULL AND
           (to_jsonb(NEW)-'disbanded_at') IS DISTINCT FROM (to_jsonb(OLD)-'disbanded_at')) THEN
        RAISE EXCEPTION 'captured Voice party source and member manifest are immutable';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER matchmaking_party_source_immutable
    BEFORE UPDATE ON parties FOR EACH ROW EXECUTE FUNCTION matchmaking_party_source_immutable();

CREATE INDEX parties_voice_source_room_active_idx
    ON parties(voice_source_room_id,voice_source_roster_version,id)
    WHERE voice_source_kind='VOICE_ROSTER' AND disbanded_at IS NULL;
CREATE INDEX parties_voice_solo_active_idx
    ON parties(leader_profile_id,id)
    WHERE voice_source_kind='SOLO' AND disbanded_at IS NULL;

CREATE TABLE matchmaking_party_search_queue_intents (
    party_id UUID PRIMARY KEY REFERENCES parties(id) ON DELETE RESTRICT,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision>0),
    action TEXT NOT NULL CHECK (action IN ('ENQUEUE','RELEASE')),
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    attempts BIGINT NOT NULL DEFAULT 0 CHECK (attempts>=0),
    projected_at TIMESTAMPTZ,
    CHECK ((lease_token IS NULL)=(lease_until IS NULL))
);
CREATE INDEX matchmaking_party_search_queue_intents_ready_idx
    ON matchmaking_party_search_queue_intents(next_attempt_at,party_id);

COMMIT;
