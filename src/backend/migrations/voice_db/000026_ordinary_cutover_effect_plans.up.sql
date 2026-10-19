-- Exact planned native obligations. These rows are not effect outcomes,
-- resource retirement receipts, or permission to mutate native state.
CREATE TABLE voice_ordinary_cutover_effect_plans (
 work_id UUID PRIMARY KEY REFERENCES voice_ordinary_cutover_works(work_id),
 protocol TEXT NOT NULL CHECK(protocol='voice.native-ordinary.effect-plan.v1'),
 work_sha256 BYTEA NOT NULL CHECK(octet_length(work_sha256)=32),
 closure_target_sha256 BYTEA NOT NULL CHECK(octet_length(closure_target_sha256)=32),
 plan_bytes BYTEA NOT NULL CHECK(octet_length(plan_bytes)>0),
 plan_sha256 BYTEA NOT NULL CHECK(octet_length(plan_sha256)=32),
 effect_count BIGINT NOT NULL CHECK(effect_count>0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE voice_ordinary_cutover_effect_plan_items (
 work_id UUID NOT NULL REFERENCES voice_ordinary_cutover_effect_plans(work_id),
 effect_id UUID NOT NULL,
 effect_revision BIGINT NOT NULL CHECK(effect_revision>=0),
 effect_kind TEXT NOT NULL CHECK(effect_kind IN ('ENSURE','CLOSE','REMOVE_MEMBER','RELEASE_FENCE')),
 step_kind TEXT NOT NULL CHECK(step_kind IN ('PRESERVE_OR_CANCEL_ENSURE','QUALIFY_MEDIA_REMOVAL','VERIFY_NATIVE_FENCE')),
 source_applied BOOLEAN NOT NULL,
 effect_bytes BYTEA NOT NULL CHECK(octet_length(effect_bytes)>0),
 effect_sha256 BYTEA NOT NULL CHECK(octet_length(effect_sha256)=32),
 target_sha256 BYTEA NOT NULL CHECK(octet_length(target_sha256)=32),
 PRIMARY KEY(work_id,effect_id,effect_revision),
 UNIQUE(work_id,effect_id)
);
REVOKE ALL ON voice_ordinary_cutover_effect_plans,voice_ordinary_cutover_effect_plan_items FROM PUBLIC;
CREATE FUNCTION voice_ordinary_cutover_effect_plan_preserve() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'ordinary effect plans are immutable retained source evidence';
END $$;
CREATE TRIGGER voice_ordinary_cutover_effect_plan_evidence BEFORE UPDATE OR DELETE ON voice_ordinary_cutover_effect_plans FOR EACH ROW EXECUTE FUNCTION voice_ordinary_cutover_effect_plan_preserve();
CREATE TRIGGER voice_ordinary_cutover_effect_item_evidence BEFORE UPDATE OR DELETE ON voice_ordinary_cutover_effect_plan_items FOR EACH ROW EXECUTE FUNCTION voice_ordinary_cutover_effect_plan_preserve();
