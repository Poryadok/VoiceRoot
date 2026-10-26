package voice.backend.auth.roleidentityauthority;

import java.nio.charset.StandardCharsets;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Objects;
import java.util.regex.Pattern;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.support.TransactionSynchronizationManager;

/** Shared Auth DDL protection, dedicated Subscription capture semantics. No role selectors. */
final class SubscriptionLifecycleCatalog {
  private final AuthIdentityDatabasePolicy policy;
  private final Map<String, String> functions;

  SubscriptionLifecycleCatalog(AuthIdentityDatabasePolicy policy) {
    this.policy = Objects.requireNonNull(policy);
    try (var source = getClass().getResourceAsStream("/db/migration/V33__subscription_lifecycle_capture.sql")) {
      String sql = new String(Objects.requireNonNull(source).readAllBytes(), StandardCharsets.UTF_8).replace("\r\n", "\n");
      var matcher = Pattern.compile("CREATE FUNCTION public\\.(\\w+)\\(\\) RETURNS trigger\\nLANGUAGE plpgsql SECURITY INVOKER SET search_path=pg_catalog AS \\$\\$(.*?)\\$\\$;", Pattern.DOTALL).matcher(sql);
      var owned = new LinkedHashMap<String, String>();
      while (matcher.find()) require(owned.put(matcher.group(1), matcher.group(2)) == null);
      require(owned.keySet().equals(java.util.Set.of("auth_capture_subscription_deletion", "auth_capture_subscription_restore", "auth_subscription_fact_immutable")));
      try(var security=getClass().getResourceAsStream("/db/migration/V12__ownership_transfer_proofs.sql")) {
        String nativeSql=new String(Objects.requireNonNull(security).readAllBytes(),StandardCharsets.UTF_8).replace("\r\n","\n");
        var nativeGuard=Pattern.compile("CREATE FUNCTION auth_advance_security_revision\\(\\) RETURNS trigger LANGUAGE plpgsql AS \\$\\$(.*?)\\$\\$;",Pattern.DOTALL).matcher(nativeSql);
        require(nativeGuard.find());owned.put("auth_advance_security_revision",nativeGuard.group(1));require(!nativeGuard.find());
      }
      functions = Map.copyOf(owned);
    } catch (Exception failure) { throw unavailable(); }
  }

  void validate(JdbcTemplate jdbc) {
    require(TransactionSynchronizationManager.isActualTransactionActive());
    require(Integer.valueOf(java.sql.Connection.TRANSACTION_READ_COMMITTED).equals(TransactionSynchronizationManager.getCurrentTransactionIsolationLevel()));
    jdbc.execute("SET LOCAL search_path=public,pg_catalog");
    budget(jdbc);
    jdbc.execute("LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS SHARE MODE");
    AuthIdentityDdlGateCatalog.validate(jdbc, policy);
    require(Boolean.TRUE.equals(jdbc.queryForObject("SELECT session_user=current_user AND NOT EXISTS(SELECT 1 FROM pg_roles r "
        + "WHERE pg_has_role(current_user,r.oid,'MEMBER') AND (r.rolsuper OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication OR r.rolbypassrls "
        + "OR r.rolname IN ('pg_read_server_files','pg_write_server_files','pg_execute_server_program') OR has_schema_privilege(r.oid,'public','CREATE'))) "
        + "AND NOT pg_has_role(current_user,(SELECT oid FROM pg_roles WHERE rolname=?),'MEMBER') "
        + "AND NOT pg_has_role(current_user,(SELECT datdba FROM pg_database WHERE datname=current_database()),'MEMBER')", Boolean.class, policy.schemaOwner())));
    AuthIdentityMigrationHistory.validate(jdbc, policy.loader());
    // Earlier accepted Auth profiles do not certify this appended capture source.
    if (policy.loader() == AuthIdentityDatabasePolicy.Loader.FLYWAY) {
      var profile = AuthIdentityDdlGateCatalog.json("migration-history-subscription-lifecycle-v1.json");
      require(profile.path("canonicalVersion").asInt() == 34);
      Integer checksum = null;
      for (var row : profile.path("flyway")) if (row.path("version").asText().equals("33")) checksum = row.path("checksum").asInt();
      require(checksum != null && jdbc.queryForObject("SELECT count(*) FROM public.flyway_schema_history WHERE version='33' AND script='V33__subscription_lifecycle_capture.sql' AND checksum=? AND success", Long.class, checksum) == 1);
    } else {
      require(Boolean.TRUE.equals(jdbc.queryForObject("SELECT version=34 AND NOT dirty FROM public.schema_migrations", Boolean.class)));
    }
    jdbc.execute("LOCK TABLE public.accounts,public.account_deletion_operations,public.account_subscription_lifecycle_heads,public.account_subscription_lifecycle_outbox IN ACCESS SHARE MODE");
    for (String table : java.util.List.of("accounts", "account_deletion_operations", "account_subscription_lifecycle_heads", "account_subscription_lifecycle_outbox")) {
      alive();
      var rows = jdbc.queryForList("SELECT c.relkind='r' AND c.relpersistence='p' AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity "
          + "AND owner.rolname=? AND NOT EXISTS(SELECT 1 FROM pg_inherits i WHERE i.inhrelid=c.oid OR i.inhparent=c.oid) "
          + "AND NOT EXISTS(SELECT 1 FROM pg_rewrite r WHERE r.ev_class=c.oid) AS valid "
          + "FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_roles owner ON owner.oid=c.relowner WHERE n.nspname='public' AND c.relname=?", policy.schemaOwner(), table);
      require(rows.size() == 1 && Boolean.TRUE.equals(rows.getFirst().get("valid")));
    }
    for (var function : functions.entrySet()) {
      alive();
      var rows = jdbc.queryForList("SELECT p.prosrc,NOT p.prosecdef AND NOT p.proleakproof AND NOT p.proisstrict AND NOT p.proretset "
          + "AND p.pronargs=0 AND p.prorettype='trigger'::regtype AND p.prokind='f' AND p.provolatile='v' AND p.proparallel='u' "
          + "AND l.lanname='plpgsql' AND (CASE WHEN p.proname='auth_advance_security_revision' THEN p.proconfig IS NULL ELSE p.proconfig=ARRAY['search_path=pg_catalog'] END) AND owner.rolname=? AS valid "
          + "FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_language l ON l.oid=p.prolang JOIN pg_roles owner ON owner.oid=p.proowner "
          + "WHERE n.nspname='public' AND p.proname=?", policy.schemaOwner(), function.getKey());
      require(rows.size() == 1 && Boolean.TRUE.equals(rows.getFirst().get("valid")) && function.getValue().equals(((String)rows.getFirst().get("prosrc")).replace("\r\n","\n")));
    }
    trigger(jdbc, "account_deletion_operations", "auth_subscription_deletion_capture", "auth_capture_subscription_deletion", 5, null, "");
    trigger(jdbc, "accounts", "auth_subscription_restore_capture", "auth_capture_subscription_restore", 17,
        "(((old.status)::text = 'deleted'::text) AND (old.deleted_at IS NOT NULL) AND ((new.status)::text = 'active'::text) AND (new.deleted_at IS NULL))", "status,deleted_at");
    trigger(jdbc, "account_subscription_lifecycle_outbox", "auth_subscription_fact_immutable", "auth_subscription_fact_immutable", 19, null, "");
    trigger(jdbc, "accounts", "accounts_security_revision", "auth_advance_security_revision", 19, null, "");
  }

  private static void trigger(JdbcTemplate jdbc, String table, String name, String function, int type, String qualifier, String columns) {
    alive();
    var rows = jdbc.queryForList("SELECT t.tgtype::integer AS type,t.tgenabled='O' AND NOT t.tgisinternal AND t.tgconstraint=0 AND t.tgconstrrelid=0 "
        + "AND octet_length(t.tgargs)=0 AND t.tgoldtable IS NULL AND t.tgnewtable IS NULL AND p.proname=? AND pn.nspname='public' AS valid,"
        + "pg_get_triggerdef(t.oid,false) AS definition,t.tgqual IS NULL AS unconditional,COALESCE((SELECT string_agg(a.attname,',' ORDER BY u.ordinality) FROM unnest(t.tgattr::smallint[]) WITH ORDINALITY u(num,ordinality) "
        + "JOIN pg_attribute a ON a.attrelid=t.tgrelid AND a.attnum=u.num),'') AS columns "
        + "FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace pn ON pn.oid=p.pronamespace "
        + "WHERE n.nspname='public' AND c.relname=? AND t.tgname=?", function, table, name);
    require(rows.size() == 1);
    var row = rows.getFirst();
    require(Boolean.TRUE.equals(row.get("valid")) && ((Number)row.get("type")).intValue() == type && columns.equals(row.get("columns")));
    if (qualifier == null) require(Boolean.TRUE.equals(row.get("unconditional")));
    else require(((String)row.get("definition")).contains(" WHEN ("+qualifier+") EXECUTE FUNCTION "));
  }

  static void require(boolean valid) { if (!valid) throw unavailable(); }
  static void alive() { require(!io.grpc.Context.current().isCancelled()); }
  static void budget(JdbcTemplate jdbc) {
    alive();
    var deadline=io.grpc.Context.current().getDeadline();
    long remaining=deadline==null?500:Math.min(500,deadline.timeRemaining(java.util.concurrent.TimeUnit.MILLISECONDS));
    require(remaining>0);
    jdbc.execute("SET LOCAL statement_timeout='"+remaining+"ms';SET LOCAL lock_timeout='"+remaining+"ms'");
  }
  static IllegalStateException unavailable() { return new IllegalStateException("Subscription lifecycle owner source unavailable"); }
}
