BEGIN;
DO $$ BEGIN
 IF current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'terminal maintenance DOWN requires READ COMMITTED before locks' USING ERRCODE='55000';
 END IF;
END $$;
LOCK TABLE native_membership_guard.epoch,native_membership_guard.spaces,
 native_membership_guard.frames,native_membership_guard.terminal_maintenance_scope,
 native_membership_guard.catalog47_transition IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM native_membership_guard.catalog47_transition)
 OR EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE terminal_maintenance_phase IS NOT NULL)
 OR EXISTS(SELECT 1 FROM native_membership_guard.terminal_maintenance_scope)
 OR EXISTS(SELECT 1 FROM native_membership_guard.frames) THEN
  RAISE EXCEPTION 'terminal maintenance DOWN retains admitted proof' USING ERRCODE='55000';
 END IF;
END $$;
DO $native47_restore$ BEGIN
 EXECUTE replace($restore$CREATE OR REPLACE FUNCTION native_membership_guard.source_before() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE s uuid;r jsonb;old_space uuid;p uuid;frame native_membership_guard.frames;
BEGIN
 IF TG_OP='TRUNCATE' THEN RAISE EXCEPTION 'membership source TRUNCATE forbidden' USING ERRCODE='55000'; END IF;
 IF TG_TABLE_NAME='space_permanent_purge_fences' THEN
  IF TG_OP='DELETE' OR (TG_OP='UPDATE' AND to_jsonb(NEW) IS DISTINCT FROM to_jsonb(OLD)) THEN
   RAISE EXCEPTION 'permanent Space retirement cannot be erased or rebound' USING ERRCODE='55000';
  END IF;
  IF TG_OP='UPDATE' THEN RETURN NEW; END IF;
 END IF;
 IF TG_TABLE_NAME='spaces' AND TG_OP='UPDATE' AND to_jsonb(NEW)->>'owner_profile_id' IS NOT DISTINCT FROM to_jsonb(OLD)->>'owner_profile_id' THEN RETURN NEW; END IF;
 r:=CASE WHEN TG_OP='DELETE' THEN to_jsonb(OLD) ELSE to_jsonb(NEW) END;
 s:=(r->>TG_ARGV[0])::uuid;
 IF TG_OP='UPDATE' THEN old_space:=(to_jsonb(OLD)->>TG_ARGV[0])::uuid;
  IF old_space IS DISTINCT FROM s THEN RAISE EXCEPTION 'membership source cannot move Space' USING ERRCODE='55000'; END IF;
 END IF;
 -- Only the exact existing terminal workflow may bypass interactive capture.
 -- Existing native guards still impose their own permanent/current11 checks.
 IF (TG_OP='DELETE' OR (TG_OP='UPDATE' AND TG_TABLE_NAME IN ('space_ordinary_member_heads','space_community_member_heads','space_membership_ownership_heads')))
  AND native_membership_guard.terminal(s) THEN
  IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
 END IF;
 PERFORM native_membership_guard.scope(s);
 SELECT * INTO frame FROM native_membership_guard.frames WHERE xid=txid_current() AND space_id=s AND pid=pg_backend_pid() FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'native source mutation requires DB capture' USING ERRCODE='55000'; END IF;
 p:=(r->>'profile_id')::uuid;
 IF p IS NOT NULL AND NOT EXISTS(SELECT 1 FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=s AND profile_id=p) THEN
  RAISE EXCEPTION 'native mutation omitted affected profile' USING ERRCODE='55000';
 END IF;
 UPDATE native_membership_guard.frames SET sequence=sequence+1,completed_sequence=-1,completed_digest=NULL,completed_at=NULL
 WHERE xid=txid_current() AND space_id=s;
 IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END $$;

$restore$,chr(10),chr(13)||chr(10));
END $native47_restore$;

DROP FUNCTION public.space_native_terminal_expire(uuid);
DROP FUNCTION public.space_native_terminal_expire_events(uuid);
DROP FUNCTION public.space_native_terminal_compact(uuid);
DROP FUNCTION public.space_native_terminal_event_delivered(uuid,uuid,bytea,bytea);
DROP FUNCTION native_membership_guard.terminal_event_wire(public.space_lifecycle_outbox);
DROP FUNCTION native_membership_guard.terminal_wire_bytes(integer,bytea);
DROP FUNCTION native_membership_guard.terminal_varint(bigint);
DROP FUNCTION native_membership_guard.terminal_representation(uuid);
DROP FUNCTION native_membership_guard.terminal_serving_scope(uuid);
DROP TABLE native_membership_guard.terminal_maintenance_scope,native_membership_guard.catalog47_transition;
ALTER TABLE native_membership_guard.spaces DROP CONSTRAINT terminal_maintenance_phase_check;
ALTER TABLE native_membership_guard.spaces DROP COLUMN terminal_maintenance_phase;
COMMIT;
