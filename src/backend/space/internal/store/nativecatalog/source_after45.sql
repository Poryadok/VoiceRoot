CREATE OR REPLACE FUNCTION native_membership_guard.source_after()
 RETURNS trigger
 LANGUAGE plpgsql
 SECURITY DEFINER
 SET search_path TO 'pg_catalog', 'public', 'native_membership_guard', 'pg_temp'
AS $function$
DECLARE s uuid;r native_membership_guard.profiles;src jsonb;at_time timestamptz;
BEGIN
 IF TG_TABLE_NAME='space_effective_member_heads' THEN RETURN NULL; END IF;
 IF TG_TABLE_NAME='spaces' AND TG_OP='UPDATE' AND to_jsonb(NEW)->>'owner_profile_id' IS NOT DISTINCT FROM to_jsonb(OLD)->>'owner_profile_id' THEN RETURN NULL; END IF;
 s:=((CASE WHEN TG_OP='DELETE' THEN to_jsonb(OLD) ELSE to_jsonb(NEW) END)->>TG_ARGV[0])::uuid;
 IF NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE xid=txid_current() AND space_id=s) THEN RETURN NULL; END IF;
 at_time:=clock_timestamp();
 -- Capture intermediate semantic disappearance too: ban/unban ABA cannot hide
 -- a real loss merely because its final ordinary witness equals the old one.
 FOR r IN SELECT * FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=s LOOP
  src:=native_membership_guard.sources(s,r.profile_id,at_time);
  UPDATE native_membership_guard.profiles SET
   ordinary_cut=ordinary_cut OR (r.old_sources->>'ordinary' IS NOT NULL AND r.old_sources->>'ordinary' IS DISTINCT FROM src->>'ordinary'),
   community_cut=community_cut OR (r.old_sources->>'community' IS NOT NULL AND r.old_sources->>'community' IS DISTINCT FROM src->>'community')
   WHERE xid=r.xid AND space_id=s AND profile_id=r.profile_id;
 END LOOP;
 UPDATE native_membership_guard.frames SET last_source_at=clock_timestamp(),sequence=sequence+1,completed_sequence=-1,
  completed_digest=NULL,completed_at=NULL WHERE xid=txid_current() AND space_id=s;
 RETURN NULL;
END $function$
