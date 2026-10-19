BEGIN;
-- Preparation only: no terminal state or writable retirement boolean. These
-- owner-authoritative bytes are consumed by the protected resource verifier.
CREATE TABLE voice_owned_game_cutover_work (
 work_id UUID PRIMARY KEY CHECK(work_id <> '00000000-0000-0000-0000-000000000000'),
 source_kind TEXT NOT NULL CHECK(source_kind IN ('OWNED_ROOM','GAME_PROVISION_AND_CONVERSION')),
 owner_protocol TEXT NOT NULL,
 request_sha256 BYTEA NOT NULL CHECK(octet_length(request_sha256)=32),
 work_bytes BYTEA NOT NULL CHECK(octet_length(work_bytes)>0),
 work_sha256 BYTEA NOT NULL CHECK(octet_length(work_sha256)=32),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
REVOKE ALL ON voice_owned_game_cutover_work FROM PUBLIC;
CREATE FUNCTION voice_owned_game_cutover_immutable_fn() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'owned/game cutover work is immutable' USING ERRCODE='55000';
END;
$$;
CREATE TRIGGER voice_owned_game_cutover_immutable BEFORE UPDATE OR DELETE ON voice_owned_game_cutover_work
 FOR EACH ROW EXECUTE FUNCTION voice_owned_game_cutover_immutable_fn();
CREATE TRIGGER voice_owned_game_cutover_no_truncate BEFORE TRUNCATE ON voice_owned_game_cutover_work
 FOR EACH STATEMENT EXECUTE FUNCTION voice_owned_game_cutover_immutable_fn();
COMMIT;
