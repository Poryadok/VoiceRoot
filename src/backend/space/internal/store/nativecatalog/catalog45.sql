CREATE OR REPLACE FUNCTION native_membership_guard.catalog()
 RETURNS bytea
 LANGUAGE sql
 STABLE
 SET search_path TO 'pg_catalog', 'public', 'native_membership_guard', 'pg_temp'
AS $function$
 SELECT sha256(convert_to(jsonb_build_object(
  'tables',(SELECT jsonb_agg(jsonb_build_array(c.oid,c.relname,c.relowner,c.relkind,c.relpersistence,c.relrowsecurity,c.relforcerowsecurity,
    CASE WHEN n.nspname='native_membership_guard' THEN c.relacl END) ORDER BY c.oid) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR (n.nspname='native_membership_guard' AND c.relkind='r')),
  'columns',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.attrelid,a.attnum) FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE a.attnum>0 AND NOT a.attisdropped AND ((n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard')),
  'triggers',(SELECT jsonb_agg(to_jsonb(t) ORDER BY t.oid) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard'),
  'constraints',(SELECT jsonb_agg(to_jsonb(k) ORDER BY k.oid) FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard'),
  'functions',(SELECT jsonb_agg(jsonb_build_array(p.oid,pg_get_functiondef(p.oid),p.proowner,p.proacl) ORDER BY p.oid)
   FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
   WHERE p.prokind='f' AND n.nspname IN ('public','native_membership_guard'))
 )::text,'UTF8'))
$function$
