package voice.backend.auth.roleidentityauthority;

import com.fasterxml.jackson.databind.JsonNode;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.springframework.jdbc.core.JdbcTemplate;

/** Exact finite release profile; never repairs, synthesizes or mixes engine histories. */
final class AuthIdentityMigrationHistory {
  private static final JsonNode PROFILE = AuthIdentityDdlGateCatalog.json("migration-history-v1.json");
  private static final JsonNode LINKED_OAUTH = AuthIdentityDdlGateCatalog.json("migration-history-linked-oauth-v1.json");
  private AuthIdentityMigrationHistory() {}
  static void validate(JdbcTemplate jdbc, AuthIdentityDatabasePolicy.Loader loader) {
    AuthIdentityDdlGateCatalog.require(loader != null);
    // The barrier covers later relevant function/schema maintenance; these table
    // locks additionally prevent concurrent replacement of history relations.
    if (loader == AuthIdentityDatabasePolicy.Loader.FLYWAY) {
      AuthIdentityDdlGateCatalog.require(Boolean.TRUE.equals(jdbc.queryForObject(
          "SELECT to_regclass('public.schema_migrations') IS NULL", Boolean.class)));
      jdbc.execute("LOCK TABLE ONLY public.flyway_schema_history IN ACCESS SHARE MODE");
      validateFlyway(jdbc.queryForList("SELECT version,script,checksum,success,type FROM public.flyway_schema_history ORDER BY installed_rank"));
    } else {
      AuthIdentityDdlGateCatalog.require(Boolean.TRUE.equals(jdbc.queryForObject(
          "SELECT to_regclass('public.flyway_schema_history') IS NULL", Boolean.class)));
      jdbc.execute("LOCK TABLE ONLY public.schema_migrations IN ACCESS SHARE MODE");
      validateCanonical(jdbc.queryForList("SELECT version,dirty FROM public.schema_migrations"));
    }
  }
  static void validateFlyway(List<Map<String,Object>> rows) {
    AuthIdentityDdlGateCatalog.require(matchesFlyway(rows,PROFILE) || matchesFlyway(rows,LINKED_OAUTH));
  }
  private static boolean matchesFlyway(List<Map<String,Object>> rows,JsonNode profile) {
    var expected = new HashMap<String,JsonNode>();
    profile.path("flyway").forEach(row -> expected.put(row.path("version").asText(), row));
    if(rows.size()!=expected.size())return false;
    for (var row : rows) {
      var wanted = expected.remove(row.get("version"));
      if(!(wanted != null && "SQL".equals(row.get("type"))
          && Boolean.TRUE.equals(row.get("success")) && row.get("checksum") instanceof Number
          && wanted.path("checksum").asInt() == ((Number)row.get("checksum")).intValue()
          && wanted.path("script").asText().equals(row.get("script"))))return false;
    }
    return expected.isEmpty();
  }
  static void validateCanonical(List<Map<String,Object>> rows) {
    AuthIdentityDdlGateCatalog.require(rows.size() == 1 && rows.getFirst().get("version") instanceof Number
        && (((Number)rows.getFirst().get("version")).longValue() == PROFILE.path("canonicalVersion").asLong()
          || ((Number)rows.getFirst().get("version")).longValue() == LINKED_OAUTH.path("canonicalVersion").asLong())
        && Boolean.FALSE.equals(rows.getFirst().get("dirty")));
    // A canonical marker is not a checksum or reconstructed Flyway history.
    // Gate and owned V28 object metadata are independently validated in this Tx.
  }
}
