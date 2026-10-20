DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Bot purge handoff migration requires READ COMMITTED';
    END IF;
END $$;

LOCK TABLE bots,bot_space_lifecycle_heads,bot_installation_intents,bot_uninstall_operations IN ACCESS EXCLUSIVE MODE;

-- Accepted journals own their immutable installation tuple. Pending P3 work
-- must be captured and positively covered before the projection can retire.
ALTER TABLE bot_uninstall_operations DROP CONSTRAINT bot_uninstall_operations_installation_id_fkey;

CREATE TABLE bot_space_purge_handoffs (
    space_id UUID PRIMARY KEY,
    deletion_operation_id UUID NOT NULL UNIQUE,
    generation BIGINT NOT NULL CHECK (generation>0),
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256)=32),
    state TEXT NOT NULL CHECK (state IN ('PENDING','COVERED','COMPLETE')),
    source_count BIGINT NOT NULL CHECK (source_count>=0),
    claim_generation BIGINT NOT NULL DEFAULT 0 CHECK (claim_generation>=0),
    claimed_until TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CHECK ((state='COMPLETE')=(completed_at IS NOT NULL))
);

CREATE TABLE bot_space_purge_sources (
    space_id UUID NOT NULL REFERENCES bot_space_purge_handoffs(space_id),
    bot_id UUID NOT NULL,
    source_bytes BYTEA NOT NULL CHECK (octet_length(source_bytes)>0),
    source_sha256 BYTEA NOT NULL CHECK (octet_length(source_sha256)=32),
    state TEXT NOT NULL DEFAULT 'PENDING' CHECK (state IN ('PENDING','COVERED')),
    coverage_bytes BYTEA,
    coverage_sha256 BYTEA CHECK (octet_length(coverage_sha256)=32),
    covered_at TIMESTAMPTZ,
    PRIMARY KEY (space_id,bot_id),
    CHECK ((coverage_bytes IS NULL)=(coverage_sha256 IS NULL)),
    CHECK ((state='COVERED')=(covered_at IS NOT NULL)),
    CHECK (state<>'COVERED' OR coverage_bytes IS NOT NULL)
);

CREATE INDEX bot_space_purge_handoffs_due_idx ON bot_space_purge_handoffs(next_attempt_at) WHERE state<>'COMPLETE';

-- Old binaries perform projection deletes and receipt insertion in one TX.
-- Denying that insertion rolls back the whole old path after FK decoupling.
-- This INVOKER guard grants no new mutation authority or evidence provenance.
CREATE FUNCTION public.bot_require_prepared_purge_handoff() RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog,public
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM public.bot_space_purge_handoffs h
        JOIN public.bot_space_lifecycle_heads l ON l.space_id=h.space_id
        WHERE h.space_id=NEW.space_id
          AND h.deletion_operation_id=NEW.deletion_operation_id
          AND h.generation=NEW.generation
          AND h.request_bytes=NEW.request_bytes
          AND h.request_sha256=NEW.request_sha256
          AND h.state='COVERED'
          AND l.deletion_operation_id=h.deletion_operation_id
          AND l.generation=h.generation AND l.desired_state='PURGED'
          AND h.source_count=(SELECT count(*) FROM public.bot_space_purge_sources s WHERE s.space_id=h.space_id)
          AND NOT EXISTS (
              SELECT 1 FROM public.bot_space_purge_sources s
              WHERE s.space_id=h.space_id AND
                (s.state<>'COVERED' OR s.coverage_bytes IS NULL OR s.coverage_sha256 IS NULL OR s.covered_at IS NULL)
          )
    ) THEN
        RAISE EXCEPTION 'Bot purge requires its prepared and covered handoff';
    END IF;
    RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION public.bot_require_prepared_purge_handoff() FROM PUBLIC;
CREATE TRIGGER bot_space_purge_receipt_handoff_guard
BEFORE INSERT ON public.bot_space_lifecycle_purge_receipts
FOR EACH ROW EXECUTE FUNCTION public.bot_require_prepared_purge_handoff();
