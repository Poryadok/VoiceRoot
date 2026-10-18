BEGIN;

ALTER TABLE matchmaking_match_squad_operations
    ADD COLUMN dispatch_protocol_version SMALLINT NOT NULL DEFAULT 0 CHECK (dispatch_protocol_version IN (0,1)),
    ADD COLUMN provision_disposition TEXT NOT NULL DEFAULT 'LIVE' CHECK (provision_disposition IN ('LIVE','ABANDONED')),
    ADD COLUMN provision_revision BIGINT NOT NULL DEFAULT 1 CHECK (provision_revision > 0),
    ADD COLUMN provision_abandoned_at TIMESTAMPTZ,
    ADD COLUMN provision_abandon_reason TEXT,
    ADD COLUMN provision_abandon_evidence_sha256 BYTEA,
    ADD COLUMN chat_dispatch_state TEXT NOT NULL DEFAULT 'NOT_ISSUED' CHECK (chat_dispatch_state IN ('NOT_ISSUED','MAY_HAVE_CREATED','RECEIPT_KNOWN')),
    ADD COLUMN chat_first_dispatch_id UUID,
    ADD COLUMN chat_first_dispatch_at TIMESTAMPTZ,
    ADD COLUMN voice_dispatch_state TEXT NOT NULL DEFAULT 'NOT_ISSUED' CHECK (voice_dispatch_state IN ('NOT_ISSUED','MAY_HAVE_CREATED','RECEIPT_KNOWN')),
    ADD COLUMN voice_first_dispatch_id UUID,
    ADD COLUMN voice_first_dispatch_at TIMESTAMPTZ;

-- Historical frozen requests may already be in flight. No receipt is not a
-- no-create proof. A legacy stored Voice request also counts as possibly issued.
UPDATE matchmaking_match_squad_operations SET
    chat_dispatch_state=CASE WHEN chat_receipt_id IS NULL THEN 'MAY_HAVE_CREATED' ELSE 'RECEIPT_KNOWN' END,
    chat_first_dispatch_id=gen_random_uuid(),chat_first_dispatch_at=created_at,
    voice_dispatch_state=CASE WHEN voice_receipt_id IS NOT NULL THEN 'RECEIPT_KNOWN'
        WHEN voice_request_bytes IS NOT NULL THEN 'MAY_HAVE_CREATED' ELSE 'NOT_ISSUED' END,
    voice_first_dispatch_id=CASE WHEN voice_request_bytes IS NOT NULL THEN gen_random_uuid() END,
    voice_first_dispatch_at=CASE WHEN voice_request_bytes IS NOT NULL THEN created_at END;

ALTER TABLE matchmaking_match_squad_operations
    ADD CONSTRAINT matchmaking_provision_abandon_binding CHECK (
      (provision_disposition='LIVE' AND provision_abandoned_at IS NULL AND provision_abandon_reason IS NULL AND provision_abandon_evidence_sha256 IS NULL) OR
      (provision_disposition='ABANDONED' AND provision_abandoned_at IS NOT NULL AND provision_abandon_reason IS NOT NULL AND provision_abandon_evidence_sha256 IS NOT NULL AND octet_length(provision_abandon_evidence_sha256)=32)
    ),
    ADD CONSTRAINT matchmaking_provision_chat_dispatch_binding CHECK (
      (chat_dispatch_state='NOT_ISSUED' AND chat_first_dispatch_id IS NULL AND chat_first_dispatch_at IS NULL AND chat_receipt_id IS NULL) OR
      (chat_dispatch_state='MAY_HAVE_CREATED' AND chat_first_dispatch_id IS NOT NULL AND chat_first_dispatch_at IS NOT NULL AND chat_receipt_id IS NULL) OR
      (chat_dispatch_state='RECEIPT_KNOWN' AND chat_first_dispatch_id IS NOT NULL AND chat_first_dispatch_at IS NOT NULL AND chat_receipt_id IS NOT NULL)
    ),
    ADD CONSTRAINT matchmaking_provision_voice_dispatch_binding CHECK (
      (voice_dispatch_state='NOT_ISSUED' AND voice_first_dispatch_id IS NULL AND voice_first_dispatch_at IS NULL AND voice_receipt_id IS NULL) OR
      (voice_dispatch_state='MAY_HAVE_CREATED' AND voice_first_dispatch_id IS NOT NULL AND voice_first_dispatch_at IS NOT NULL AND voice_receipt_id IS NULL AND voice_request_bytes IS NOT NULL) OR
      (voice_dispatch_state='RECEIPT_KNOWN' AND voice_first_dispatch_id IS NOT NULL AND voice_first_dispatch_at IS NOT NULL AND voice_receipt_id IS NOT NULL)
    );

COMMIT;
