-- A local store identity is distinct from currentness/restore authority.
CREATE TABLE moderation_startup_identity (
  singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
  incarnation uuid NOT NULL DEFAULT gen_random_uuid() CHECK(incarnation <> '00000000-0000-0000-0000-000000000000')
);
INSERT INTO moderation_startup_identity(singleton) VALUES(true);
CREATE FUNCTION moderation_startup_identity_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'moderation startup store identity is immutable';
END $$;
CREATE TRIGGER moderation_startup_identity_guard BEFORE UPDATE OR DELETE ON moderation_startup_identity
FOR EACH ROW EXECUTE FUNCTION moderation_startup_identity_guard();
