package voice.backend.auth.roleidentityauthority;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.HexFormat;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.support.TransactionSynchronizationManager;

/** Owned V28 invariants, permitting unrelated compatible additive migrations. */
final class AuthIdentityCatalog {
  private static final ObjectMapper JSON = new ObjectMapper();
  // Resource loading and parsing happen during construction, outside SQL locks.
  private final JsonNode expected = load();

  private static JsonNode load() {
    try (var input = AuthIdentityCatalog.class.getResourceAsStream("/roleidentityauthority/catalog-v1.json")) {
      var result = JSON.readTree(input);
      require(result.path("version").asInt() == 1 && result.path("tables").size() == 6
          && result.path("functions").size() == 6 && result.path("triggers").size() == 3);
      return result;
    } catch (Exception failure) { throw unavailable(); }
  }

  void validate(JdbcTemplate jdbc) {
    require(TransactionSynchronizationManager.isActualTransactionActive());
    jdbc.execute("SET LOCAL lock_timeout='2s'");
    jdbc.execute("SET LOCAL statement_timeout='2s'");
    jdbc.execute("SET LOCAL search_path=public,pg_catalog");
    // Stabilize catalog definitions before the first domain-row lock. Compatible
    // owner writers can still take their normal account/lifetime/head row locks.
    jdbc.execute("LOCK TABLE public.accounts,public.role_identity_account_selectors,"
        + "public.role_identity_key_versions,public.role_identity_lifetimes,"
        + "public.role_identity_owner_journal,public.role_identity_owner_state IN ACCESS SHARE MODE");
    require(Boolean.TRUE.equals(jdbc.queryForObject("SELECT count(*)=1 AND bool_and("
        + "script='V28__role_identity_owner.sql' AND checksum=-1142833847 AND success) "
        + "FROM public.flyway_schema_history WHERE version='28'", Boolean.class)));

    for (var table : expected.path("tables")) validateTable(jdbc, table);
    for (var function : expected.path("functions")) validateFunction(jdbc, function);
    for (var trigger : expected.path("triggers")) {
      var rows = jdbc.queryForList("SELECT t.tgtype::integer AS type,t.tgenabled,t.tgisinternal,"
          + "t.tgconstraint=0 AND t.tgconstrrelid=0 AND octet_length(t.tgargs)=0 "
          + "AND t.tgoldtable IS NULL AND t.tgnewtable IS NULL AS plain,"
          + "p.proname,pg_get_triggerdef(t.oid,false) AS definition "
          + "FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid "
          + "JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_proc p ON p.oid=t.tgfoid "
          + "JOIN pg_namespace pn ON pn.oid=p.pronamespace "
          + "WHERE n.nspname='public' AND pn.nspname='public' AND c.relname=? AND t.tgname=?",
          trigger.path("table").asText(), trigger.path("name").asText());
      require(rows.size() == 1);
      var row = rows.getFirst();
      require(trigger.path("type").asInt() == ((Number) row.get("type")).intValue()
          && "O".equals(row.get("tgenabled")) && Boolean.FALSE.equals(row.get("tgisinternal"))
          && Boolean.TRUE.equals(row.get("plain"))
          && trigger.path("function").asText().equals(row.get("proname"))
          && trigger.path("definition").asText().equals(row.get("definition")));
    }
  }

  private void validateTable(JdbcTemplate jdbc, JsonNode table) {
    String name = table.path("name").asText();
    var rows = jdbc.queryForList("SELECT c.oid,c.relkind,c.relpersistence,c.relrowsecurity,c.relforcerowsecurity,"
        + "NOT EXISTS(SELECT 1 FROM pg_inherits i WHERE i.inhrelid=c.oid OR i.inhparent=c.oid) "
        + "AND NOT EXISTS(SELECT 1 FROM pg_rewrite r WHERE r.ev_class=c.oid) AS plain "
        + "FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=?", name);
    require(rows.size() == 1);
    var row = rows.getFirst();
    require("r".equals(row.get("relkind")) && "p".equals(row.get("relpersistence"))
        && Boolean.FALSE.equals(row.get("relrowsecurity")) && Boolean.FALSE.equals(row.get("relforcerowsecurity"))
        && Boolean.TRUE.equals(row.get("plain")));
    long oid = ((Number) row.get("oid")).longValue();
    for (var column : table.path("columns")) {
      String actual = jdbc.queryForObject("SELECT jsonb_build_object('name',a.attname,'type',format_type(a.atttypid,a.atttypmod),"
          + "'notNull',a.attnotnull,'defaultExpression',pg_get_expr(d.adbin,d.adrelid),"
          + "'generated',a.attgenerated,'identity',a.attidentity)::text "
          + "FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum "
          + "WHERE a.attrelid=?::oid AND a.attname=? AND a.attnum>0 AND NOT a.attisdropped", String.class, oid, column.path("name").asText());
      try { require(column.equals(JSON.readTree(actual))); }
      catch (Exception failure) { throw unavailable(); }
    }
    for (var constraint : table.path("constraints")) {
      var definitions = jdbc.queryForList("SELECT pg_get_constraintdef(con.oid,false) AS definition,"
          + "con.convalidated AND NOT con.condeferrable AND (con.conindid=0 OR EXISTS "
          + "(SELECT 1 FROM pg_index i WHERE i.indexrelid=con.conindid AND i.indisvalid AND i.indisready)) AS valid "
          + "FROM pg_constraint con WHERE con.conrelid=?::oid AND con.conname=?", oid, constraint.path("name").asText());
      require(definitions.size() == 1 && Boolean.TRUE.equals(definitions.getFirst().get("valid"))
          && constraint.path("definition").asText().equals(definitions.getFirst().get("definition")));
    }
    if (!name.equals("accounts")) {
      int count = jdbc.queryForObject("SELECT count(*)::integer FROM pg_trigger WHERE tgrelid=?::oid AND NOT tgisinternal", Integer.class, oid);
      require(count == (name.equals("role_identity_owner_journal") ? 1 : 0));
    }
  }

  private void validateFunction(JdbcTemplate jdbc, JsonNode function) {
    var rows = jdbc.queryForList("SELECT p.prosrc,p.prosecdef,p.proleakproof,p.provolatile,p.proparallel,p.proconfig,"
        + "l.lanname,pg_get_function_identity_arguments(p.oid) AS arguments,pg_get_function_result(p.oid) AS result "
        + "FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_language l ON l.oid=p.prolang "
        + "WHERE n.nspname='public' AND p.proname=?", function.path("name").asText());
    require(rows.size() == 1);
    var row = rows.getFirst();
    require(Boolean.FALSE.equals(row.get("prosecdef")) && Boolean.FALSE.equals(row.get("proleakproof"))
        && "v".equals(row.get("provolatile")) && "u".equals(row.get("proparallel"))
        && row.get("proconfig") == null && "plpgsql".equals(row.get("lanname"))
        && function.path("arguments").asText().equals(row.get("arguments"))
        && function.path("resultType").asText().equals(row.get("result")));
    try {
      byte[] source = ((String) row.get("prosrc")).replace("\r\n", "\n").getBytes(StandardCharsets.UTF_8);
      require(function.path("sourceSha256").asText().equals(HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(source))));
    } catch (Exception failure) { throw unavailable(); }
  }

  private static void require(boolean valid) { if (!valid) throw unavailable(); }
  private static IllegalStateException unavailable() { return new IllegalStateException("identity owner catalog unavailable"); }
}
