package voice.backend.auth.roleidentityauthority;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.sql.Connection;
import java.util.*;
import java.util.regex.Pattern;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.junit.jupiter.api.Test;

/** Valid V28 and installation policy cannot substitute for an installed DDL gate. */
final class AuthIdentityMissingGateTest {
  @Test void validOwnedV28CannotSubstituteForInstalledDdlGate() throws Exception {
    var jdbc = new LegacyCatalogJdbc();
    TransactionSynchronizationManager.setActualTransactionActive(true);
    TransactionSynchronizationManager.setCurrentTransactionIsolationLevel(Connection.TRANSACTION_READ_COMMITTED);
    try {
      boolean denied = false;
      try {
        new AuthIdentityCatalog(new AuthIdentityDatabasePolicy(
            "trusted_auth_schema_owner", AuthIdentityDatabasePolicy.Loader.FLYWAY)).validate(jdbc);
      }
      catch (IllegalStateException unavailable) { denied = true; }
      if (!denied) throw new AssertionError("valid V28 accepted without an installed DDL gate");
      if (jdbc.gateQueries != 1) {
        throw new AssertionError("missing-gate metadata query was not reached exactly once");
      }
      int barrier = jdbc.operations.indexOf(
          "LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS SHARE MODE");
      int gate = jdbc.operations.indexOf(AuthIdentityDdlGateCatalog.QUERY);
      if (barrier < 0 || gate <= barrier) {
        throw new AssertionError("direct barrier must precede missing-gate metadata query");
      }
    } finally { TransactionSynchronizationManager.clear(); }
  }

  static final class LegacyCatalogJdbc extends JdbcTemplate {
    final JsonNode catalog;
    final Map<String, String> bodies = new HashMap<>();
    final List<String> operations = new ArrayList<>();
    int gateQueries;
    LegacyCatalogJdbc() throws Exception {
      var json = new ObjectMapper();
      try (var input = getClass().getResourceAsStream("/roleidentityauthority/catalog-v1.json")) {
        catalog = json.readTree(input);
      }
      String migration;
      try (var input = getClass().getResourceAsStream("/db/migration/V28__role_identity_owner.sql")) {
        migration = new String(input.readAllBytes(), java.nio.charset.StandardCharsets.UTF_8);
      }
      var matcher = Pattern.compile("CREATE FUNCTION ([a-z_]+).*?LANGUAGE plpgsql AS \\$\\$(.*?)\\$\\$;", Pattern.DOTALL).matcher(migration);
      while (matcher.find()) bodies.put(matcher.group(1), matcher.group(2));
    }
    @Override public void execute(String sql) { operations.add(sql); }
    @Override public <T> T queryForObject(String sql, Class<T> type) {
      operations.add(sql);
      if (sql.equals(AuthIdentityDdlGateCatalog.QUERY)) {
        gateQueries++;
        throw new IllegalStateException("gate is absent");
      }
      if (type == Boolean.class && sql.contains("flyway_schema_history")) return type.cast(true);
      throw new IllegalStateException("gate is absent");
    }
    @Override public <T> T queryForObject(String sql, Class<T> type, Object... args) {
      if (sql.contains("pg_attribute")) {
        for (var table : catalog.path("tables")) if (index(table) == ((Number) args[0]).longValue()) {
          for (var column : table.path("columns")) if (column.path("name").asText().equals(args[1])) return type.cast(column.toString());
        }
      }
      if (sql.contains("count(*)::integer FROM pg_trigger")) {
        var table = catalog.path("tables").get(((Number) args[0]).intValue()-1);
        return type.cast(table.path("name").asText().equals("role_identity_owner_journal") ? 1 : 0);
      }
      throw new IllegalStateException("gate is absent");
    }
    long index(JsonNode table) {
      for (int i=0;i<catalog.path("tables").size();i++) if (catalog.path("tables").get(i).equals(table)) return i+1;
      throw new AssertionError();
    }
    @Override public List<Map<String,Object>> queryForList(String sql, Object... args) {
      if (sql.contains("FROM pg_class c")) for (var table : catalog.path("tables")) if (table.path("name").asText().equals(args[0])) {
        return List.of(Map.of("oid",index(table),"relkind","r","relpersistence","p","relrowsecurity",false,"relforcerowsecurity",false,"plain",true));
      }
      if (sql.contains("FROM pg_constraint")) for (var table : catalog.path("tables")) if (index(table)==((Number)args[0]).longValue()) {
        for (var constraint : table.path("constraints")) if (constraint.path("name").asText().equals(args[1])) return List.of(Map.of("definition",constraint.path("definition").asText(),"valid",true));
      }
      if (sql.contains("FROM pg_proc p")) for (var function : catalog.path("functions")) if (function.path("name").asText().equals(args[0])) {
        var row = new HashMap<String,Object>();
        row.put("prosrc",bodies.get(args[0])); row.put("prosecdef",false); row.put("proleakproof",false);
        row.put("provolatile","v"); row.put("proparallel","u"); row.put("proconfig",null);
        row.put("lanname","plpgsql"); row.put("arguments",function.path("arguments").asText()); row.put("result",function.path("resultType").asText());
        return List.of(row);
      }
      if (sql.contains("FROM pg_trigger t")) for (var trigger : catalog.path("triggers")) if (trigger.path("table").asText().equals(args[0]) && trigger.path("name").asText().equals(args[1])) {
        return List.of(Map.of("type",trigger.path("type").asInt(),"tgenabled","O","tgisinternal",false,"plain",true,"proname",trigger.path("function").asText(),"definition",trigger.path("definition").asText()));
      }
      throw new IllegalStateException("gate is absent");
    }
  }
  public static void main(String[] args) throws Exception {
    if (!System.getProperty("os.name").equals("Linux")) throw new AssertionError("Linux required");
    new AuthIdentityMissingGateTest().validOwnedV28CannotSubstituteForInstalledDdlGate();
    System.out.println("PASS valid-policy missing-gate query after direct barrier; no PostgreSQL execution");
  }
}
