SELECT jsonb_build_object(
 'barrier_owner',br.rolname,
 'relation_ok',b.relkind='r' AND b.relpersistence='p' AND NOT b.relrowsecurity AND NOT b.relforcerowsecurity
   AND NOT EXISTS(SELECT 1 FROM pg_inherits i WHERE i.inhrelid=b.oid OR i.inhparent=b.oid)
   AND NOT EXISTS(SELECT 1 FROM pg_rewrite w WHERE w.ev_class=b.oid)
   AND NOT EXISTS(SELECT 1 FROM pg_trigger t WHERE t.tgrelid=b.oid)
   AND (SELECT count(*) FROM pg_attribute a WHERE a.attrelid=b.oid AND a.attnum>0 AND NOT a.attisdropped)=1
   AND EXISTS(SELECT 1 FROM pg_attribute a WHERE a.attrelid=b.oid AND a.attname='singleton'
       AND a.atttypid='boolean'::regtype AND a.attnotnull AND a.attgenerated='' AND a.attidentity=''
       AND a.attacl IS NULL AND NOT EXISTS(SELECT 1 FROM pg_attrdef d WHERE d.adrelid=b.oid AND d.adnum=a.attnum))
   AND (SELECT count(*) FROM pg_constraint c WHERE c.conrelid=b.oid)=0,
 'acl_ok',NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE(b.relacl,acldefault('r',b.relowner))) a
     WHERE a.grantee<>b.relowner AND NOT(a.grantee=0 AND a.privilege_type='SELECT' AND NOT a.is_grantable))
   AND EXISTS(SELECT 1 FROM aclexplode(COALESCE(b.relacl,acldefault('r',b.relowner))) a
     WHERE a.grantee=0 AND a.privilege_type='SELECT' AND NOT a.is_grantable),
 'function_owner',fr.rolname,
 'function_ok',p.prokind='f' AND p.pronargs=0 AND p.prorettype='event_trigger'::regtype
   AND NOT p.prosecdef AND NOT p.proleakproof AND p.provolatile='v' AND p.proparallel='u'
   AND NOT p.proisstrict AND NOT p.proretset AND p.proconfig IS NULL AND l.lanname='plpgsql'
   AND NOT EXISTS(SELECT 1 FROM pg_proc q WHERE q.pronamespace=p.pronamespace AND q.proname=p.proname AND q.oid<>p.oid)
   AND NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee<>p.proowner),
 'function_source',p.prosrc,
 'event_owner',er.rolname,'event_enabled',e.evtenabled,'event_tags',to_jsonb(e.evttags),
 'binding_ok',e.evtevent='ddl_command_start' AND e.evtfoid=p.oid,
 'major_ok',current_setting('server_version_num')::integer/10000=16,
 'runtime_ok',NOT runtime.rolsuper AND current_user<>br.rolname
)::text
FROM pg_class b JOIN pg_namespace bn ON bn.oid=b.relnamespace
JOIN pg_roles br ON br.oid=b.relowner
JOIN pg_namespace fn ON fn.nspname='public'
JOIN pg_proc p ON p.pronamespace=fn.oid AND p.proname='auth_identity_ddl_gate'
JOIN pg_roles fr ON fr.oid=p.proowner JOIN pg_language l ON l.oid=p.prolang
JOIN pg_event_trigger e ON e.evtname='auth_identity_ddl_gate'
JOIN pg_roles er ON er.oid=e.evtowner JOIN pg_roles runtime ON runtime.rolname=current_user
WHERE bn.nspname='public' AND b.relname='auth_identity_ddl_barrier'
