-- Native per-chat authority survives deletion/recreation; no FK or TTL may
-- erase its monotonic head. Source facts and heads commit in one transaction.
ALTER TABLE chats ADD COLUMN scope_incarnation UUID NOT NULL DEFAULT gen_random_uuid();
CREATE TABLE chat_scope_authority_heads (
    chat_id UUID PRIMARY KEY,
    scope_incarnation UUID NOT NULL,
    revision BIGINT NOT NULL CHECK (revision > 0),
    space_id UUID,
    present BOOLEAN NOT NULL
);
CREATE INDEX chat_scope_authority_heads_space_idx ON chat_scope_authority_heads(space_id);
INSERT INTO chat_scope_authority_heads(chat_id,scope_incarnation,revision,space_id,present)
SELECT id,scope_incarnation,1,space_id,true FROM chats;

CREATE FUNCTION maintain_chat_scope_authority() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE incarnation UUID;
BEGIN
    IF TG_OP = 'DELETE' THEN
        UPDATE chat_scope_authority_heads SET present=false,revision=revision+1 WHERE chat_id=OLD.id AND present;
        IF NOT FOUND THEN RAISE EXCEPTION 'Chat authority head unavailable'; END IF;
        RETURN OLD;
    END IF;
    IF TG_OP = 'UPDATE' THEN
        IF NEW.id IS DISTINCT FROM OLD.id THEN RAISE EXCEPTION 'Chat identity is immutable'; END IF;
        IF ROW(NEW.space_id,NEW.type,NEW.creator_profile_id,NEW.managed_by_application_id,
               NEW.managed_environment_id,NEW.external_chat_key)
           IS NOT DISTINCT FROM
           ROW(OLD.space_id,OLD.type,OLD.creator_profile_id,OLD.managed_by_application_id,
               OLD.managed_environment_id,OLD.external_chat_key) THEN
            NEW.scope_incarnation := OLD.scope_incarnation;
            RETURN NEW;
        END IF;
    END IF;
    incarnation := gen_random_uuid();
    INSERT INTO chat_scope_authority_heads(chat_id,scope_incarnation,revision,space_id,present)
    VALUES(NEW.id,incarnation,1,NEW.space_id,true)
    ON CONFLICT(chat_id) DO UPDATE SET scope_incarnation=EXCLUDED.scope_incarnation,
        revision=chat_scope_authority_heads.revision+1,space_id=EXCLUDED.space_id,present=true;
    NEW.scope_incarnation := incarnation;
    RETURN NEW;
END $$;
CREATE TRIGGER chat_scope_authority_source BEFORE INSERT OR UPDATE OR DELETE ON chats
FOR EACH ROW EXECUTE FUNCTION maintain_chat_scope_authority();

-- MatchSquad ownership is a native consumed fact even if its current writer
-- creates standalone resources. Bump the same head in its insertion transaction.
CREATE FUNCTION advance_chat_scope_managed_authority() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    UPDATE chat_scope_authority_heads SET revision=revision+1 WHERE chat_id=NEW.chat_id;
    IF NOT FOUND THEN RAISE EXCEPTION 'Chat authority head unavailable'; END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER chat_scope_authority_managed AFTER INSERT ON chat_match_squad_operations
FOR EACH ROW EXECUTE FUNCTION advance_chat_scope_managed_authority();

CREATE FUNCTION protect_chat_scope_authority_head() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Chat authority heads are permanent'; END IF;
    IF NEW.chat_id IS DISTINCT FROM OLD.chat_id OR NEW.revision <= OLD.revision THEN
        RAISE EXCEPTION 'Chat authority revision must advance without reset';
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER chat_scope_authority_head_guard BEFORE UPDATE OR DELETE ON chat_scope_authority_heads
FOR EACH ROW EXECUTE FUNCTION protect_chat_scope_authority_head();

CREATE FUNCTION advance_chat_scope_lifecycle_authority() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'Chat lifecycle authority fences are permanent'; END IF;
    IF TG_OP = 'UPDATE' AND NEW.space_id IS DISTINCT FROM OLD.space_id THEN
        RAISE EXCEPTION 'Chat lifecycle scope is immutable';
    END IF;
    IF TG_OP = 'INSERT' OR ROW(NEW.generation,NEW.state) IS DISTINCT FROM ROW(OLD.generation,OLD.state) THEN
        UPDATE chat_scope_authority_heads SET revision=revision+1 WHERE space_id=NEW.space_id;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER chat_scope_authority_lifecycle AFTER INSERT OR UPDATE OR DELETE ON chat_space_lifecycle_fences
FOR EACH ROW EXECUTE FUNCTION advance_chat_scope_lifecycle_authority();

CREATE FUNCTION reject_chat_scope_authority_truncate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Chat scope authority cannot be truncated'; END $$;
CREATE TRIGGER chat_scope_authority_source_truncate BEFORE TRUNCATE ON chats
FOR EACH STATEMENT EXECUTE FUNCTION reject_chat_scope_authority_truncate();
CREATE TRIGGER chat_scope_authority_heads_truncate BEFORE TRUNCATE ON chat_scope_authority_heads
FOR EACH STATEMENT EXECUTE FUNCTION reject_chat_scope_authority_truncate();
CREATE TRIGGER chat_scope_authority_fences_truncate BEFORE TRUNCATE ON chat_space_lifecycle_fences
FOR EACH STATEMENT EXECUTE FUNCTION reject_chat_scope_authority_truncate();
