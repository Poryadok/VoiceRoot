-- A sealed protected terminal view identifies the Auth authority generation.
-- Restore still requires independently recovered authoritative history; a
-- rolled-back generation/high-water is not self-certifying.
CREATE TABLE moderation_authority_generation (
  singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
  generation uuid NOT NULL DEFAULT gen_random_uuid(),
  terminal_high_water bigint NOT NULL DEFAULT 0 CHECK(terminal_high_water>=0)
);
INSERT INTO moderation_authority_generation(terminal_high_water)
SELECT count(*) FROM moderation_terminal_subjects;
CREATE FUNCTION moderation_authority_generation_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' OR NEW.generation IS DISTINCT FROM OLD.generation OR NEW.singleton IS DISTINCT FROM OLD.singleton OR NEW.terminal_high_water<OLD.terminal_high_water THEN
    RAISE EXCEPTION 'moderation authority generation and terminal progress cannot be reset';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER moderation_authority_generation_guard BEFORE UPDATE OR DELETE ON moderation_authority_generation
FOR EACH ROW EXECUTE FUNCTION moderation_authority_generation_guard();
CREATE FUNCTION moderation_terminal_progress() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP<>'INSERT' THEN RAISE EXCEPTION 'terminal moderation authority is permanent'; END IF;
  UPDATE moderation_authority_generation SET terminal_high_water=terminal_high_water+1 WHERE singleton;
  IF NOT FOUND THEN RAISE EXCEPTION 'moderation authority generation is unavailable'; END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER moderation_terminal_progress AFTER INSERT ON moderation_terminal_subjects
FOR EACH ROW EXECUTE FUNCTION moderation_terminal_progress();
CREATE TRIGGER moderation_terminal_permanent BEFORE UPDATE OR DELETE ON moderation_terminal_subjects
FOR EACH ROW EXECUTE FUNCTION moderation_terminal_progress();
