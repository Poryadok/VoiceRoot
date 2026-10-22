SELECT '{"tables": ' || COALESCE('[' || (SELECT string_agg((jsonb_build_array(c.oid,c.relname,c.relowner,c.relkind,c.relpersistence,c.relrowsecurity,c.relforcerowsecurity,
    CASE WHEN n.nspname='native_membership_guard' THEN c.relacl END))::text, ', ' ORDER BY c.oid) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR (n.nspname='native_membership_guard' AND c.relkind='r')) || ']', 'null') ||
 ', "columns": ' || COALESCE('[' || (SELECT string_agg((to_jsonb(a))::text, ', ' ORDER BY a.attrelid,a.attnum) FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE a.attnum>0 AND NOT a.attisdropped AND ((n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard')) || ']', 'null') ||
 ', "triggers": ' || COALESCE('[' || (SELECT string_agg((to_jsonb(t))::text, ', ' ORDER BY t.oid) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard') || ']', 'null') ||
 ', "functions": ' || COALESCE('[' || (SELECT string_agg((jsonb_build_array(p.oid,pg_get_functiondef(p.oid),p.proowner,p.proacl))::text, ', ' ORDER BY p.oid)
   FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
   WHERE p.prokind='f' AND n.nspname IN ('public','native_membership_guard')) || ']', 'null') ||
 ', "constraints": ' || COALESCE('[' || (SELECT string_agg((to_jsonb(k))::text, ', ' ORDER BY k.oid) FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard') || ']', 'null') || '}'