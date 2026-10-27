package voice.backend.auth.roleidentityauthority;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.nio.charset.StandardCharsets;
import java.util.*;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;

/** Pure decision and JDBC-double tests; no PostgreSQL connection or SQL execution. */
final class AuthIdentityDdlGateSourceTest {
  static final ObjectMapper JSON = new ObjectMapper();
  static final String OWNER = "trusted_auth_schema_owner";
  static ObjectNode valid() throws Exception {
    var node = JSON.createObjectNode();
    for (String key : List.of("barrier_owner","function_owner","event_owner")) node.put(key,OWNER);
    for (String key : List.of("relation_ok","acl_ok","function_ok","binding_ok","major_ok","runtime_ok")) node.put(key,true);
    node.put("event_enabled","O");
    node.set("event_tags",AuthIdentityDdlGateCatalog.EXPECTED.path("tags").deepCopy());
    try (var stream = AuthIdentityDdlGateSourceTest.class.getResourceAsStream("/db/migration/V31__auth_identity_ddl_barrier.sql")) {
      String sql = new String(stream.readAllBytes(),StandardCharsets.UTF_8);
      node.put("function_source",sql.substring(sql.indexOf("AS $$")+5,sql.indexOf("$$;")));
    }
    return node;
  }
  static void deny(Runnable check) {
    try { check.run(); } catch (IllegalStateException expected) { return; }
    throw new AssertionError("unsafe metadata accepted");
  }
  @Test void exactGateAcceptedAndEveryProtectionRequired() throws Exception {
    var good = valid();
    AuthIdentityDdlGateCatalog.validateMetadata(good,OWNER,false);
    for (String key : List.of("relation_ok","acl_ok","function_ok","binding_ok","major_ok","runtime_ok")) {
      var changed = good.deepCopy().put(key,false);
      deny(() -> AuthIdentityDdlGateCatalog.validateMetadata(changed,OWNER,false));
    }
    for (String key : List.of("barrier_owner","function_owner","event_owner")) {
      var changed = good.deepCopy().put(key,"other_owner");
      deny(() -> AuthIdentityDdlGateCatalog.validateMetadata(changed,OWNER,false));
    }
    for (String key : List.of("function_source","event_enabled")) {
      var changed = good.deepCopy().put(key,key.equals("event_enabled")?"D":"BEGIN RETURN; END ");
      deny(() -> AuthIdentityDdlGateCatalog.validateMetadata(changed,OWNER,false));
    }
    var missing = good.deepCopy(); missing.withArray("event_tags").remove(0);
    deny(() -> AuthIdentityDdlGateCatalog.validateMetadata(missing,OWNER,false));
    var duplicate = good.deepCopy(); duplicate.withArray("event_tags").add(duplicate.path("event_tags").get(0));
    deny(() -> AuthIdentityDdlGateCatalog.validateMetadata(duplicate,OWNER,false));
    deny(() -> AuthIdentityDdlGateCatalog.validateMetadata(good,null,false));
  }
  static List<Map<String,Object>> flywayRows() {
    var rows = new ArrayList<Map<String,Object>>();
    for (var expected : AuthIdentityDdlGateCatalog.json("migration-history-v1.json").path("flyway")) {
      rows.add(new HashMap<>(Map.of("version",expected.path("version").asText(),
          "script",expected.path("script").asText(),"checksum",expected.path("checksum").asInt(),"success",true,"type","SQL")));
    }
    return rows;
  }
  @Test void realLoaderProfilesNeverMixOrRepairHistory() {
    var rows = flywayRows(); AuthIdentityMigrationHistory.validateFlyway(rows);
    rows.getLast().put("checksum",0); deny(() -> AuthIdentityMigrationHistory.validateFlyway(rows));
    var dirty = flywayRows(); dirty.getLast().put("success",false); deny(() -> AuthIdentityMigrationHistory.validateFlyway(dirty));
    var wrong = flywayRows(); wrong.getLast().put("script","V31__foreign.sql"); deny(() -> AuthIdentityMigrationHistory.validateFlyway(wrong));
    var duplicate = flywayRows(); duplicate.set(duplicate.size()-1,duplicate.getFirst()); deny(() -> AuthIdentityMigrationHistory.validateFlyway(duplicate));
    var baseline = flywayRows(); baseline.getFirst().put("type","BASELINE"); deny(() -> AuthIdentityMigrationHistory.validateFlyway(baseline));
    var unknown = flywayRows(); unknown.add(Map.of("version","32","script","foreign","checksum",1,"success",true,"type","SQL"));
    deny(() -> AuthIdentityMigrationHistory.validateFlyway(unknown));
    AuthIdentityMigrationHistory.validateCanonical(List.of(Map.of("version",32L,"dirty",false)));
    AuthIdentityMigrationHistory.validateCanonical(List.of(Map.of("version",33L,"dirty",false)));
    for (long version : new long[]{29,31,36}) deny(() -> AuthIdentityMigrationHistory.validateCanonical(List.of(Map.of("version",version,"dirty",false))));
    deny(() -> AuthIdentityMigrationHistory.validateCanonical(List.of(Map.of("version",32L,"dirty",true))));
    deny(() -> AuthIdentityMigrationHistory.validateCanonical(List.of()));
    deny(() -> AuthIdentityMigrationHistory.validateCanonical(List.of(Map.of("version",32L,"dirty",false),Map.of("version",32L,"dirty",false))));
    var mixed = new HistoryJdbc(false);
    for (var loader : AuthIdentityDatabasePolicy.Loader.values()) deny(() -> AuthIdentityMigrationHistory.validate(mixed,loader));
    if (mixed.calls.size()!=2) throw new AssertionError("mixed history proceeded beyond engine exclusion");
    var canonical = new HistoryJdbc(true);
    AuthIdentityMigrationHistory.validate(canonical,AuthIdentityDatabasePolicy.Loader.CANONICAL);
    if (canonical.calls.stream().anyMatch(sql -> sql.contains("FROM public.flyway_schema_history"))) throw new AssertionError("canonical path queried Flyway rows");
    if (!canonical.calls.get(1).equals("LOCK TABLE ONLY public.schema_migrations IN ACCESS SHARE MODE")) throw new AssertionError("history lock missing");
  }
  @Test void linkedOAuthProfileIsExactAndPreservesThePriorProfile() {
    var old=flywayRows();AuthIdentityMigrationHistory.validateFlyway(old);
    var latest=new ArrayList<Map<String,Object>>();
    for(var expected:AuthIdentityDdlGateCatalog.json("migration-history-linked-oauth-v1.json").path("flyway"))
      latest.add(new HashMap<>(Map.of("version",expected.path("version").asText(),"script",expected.path("script").asText(),
          "checksum",expected.path("checksum").asInt(),"success",true,"type","SQL")));
    if(!latest.subList(0,old.size()).equals(old))throw new AssertionError("linked OAuth release changed prior accepted migration bytes");
    AuthIdentityMigrationHistory.validateFlyway(latest);
    latest.getLast().put("checksum",0);deny(()->AuthIdentityMigrationHistory.validateFlyway(latest));
  }
  @Test void subscriptionLifecycleProfilePreservesTheExactPredecessor() {
    var prior=new ArrayList<Map<String,Object>>();
    for(var expected:AuthIdentityDdlGateCatalog.json("migration-history-linked-oauth-v1.json").path("flyway"))
      prior.add(new HashMap<>(Map.of("version",expected.path("version").asText(),"script",expected.path("script").asText(),
          "checksum",expected.path("checksum").asInt(),"success",true,"type","SQL")));
    var latest=new ArrayList<Map<String,Object>>();
    for(var expected:AuthIdentityDdlGateCatalog.json("migration-history-subscription-lifecycle-v1.json").path("flyway"))
      latest.add(new HashMap<>(Map.of("version",expected.path("version").asText(),"script",expected.path("script").asText(),
          "checksum",expected.path("checksum").asInt(),"success",true,"type","SQL")));
    if(!latest.subList(0,prior.size()).equals(prior))throw new AssertionError("lifecycle release changed accepted predecessor bytes");
    AuthIdentityMigrationHistory.validateFlyway(latest);
    AuthIdentityMigrationHistory.validateCanonical(List.of(Map.of("version",34L,"dirty",false)));
    latest.getLast().put("checksum",0);deny(()->AuthIdentityMigrationHistory.validateFlyway(latest));
    deny(()->AuthIdentityMigrationHistory.validateCanonical(List.of(Map.of("version",36L,"dirty",false))));
  }
  @Test void accountLifecycleSelectsOnlyItsExactSuccessorCatalog() throws Exception {
    var prior=new ArrayList<Map<String,Object>>();
    for(var row:AuthIdentityDdlGateCatalog.json("migration-history-subscription-lifecycle-v1.json").path("flyway"))
      prior.add(new HashMap<>(Map.of("version",row.path("version").asText(),"script",row.path("script").asText(),"checksum",row.path("checksum").asInt(),"success",true,"type","SQL")));
    var latest=new ArrayList<Map<String,Object>>();
    var profile=AuthIdentityDdlGateCatalog.json("migration-history-account-lifecycle-v1.json");
    for(var row:profile.path("flyway"))
      latest.add(new HashMap<>(Map.of("version",row.path("version").asText(),"script",row.path("script").asText(),"checksum",row.path("checksum").asInt(),"success",true,"type","SQL")));
    if(!latest.subList(0,prior.size()).equals(prior))throw new AssertionError("account lifecycle changed predecessor");
    if(AuthIdentityMigrationHistory.validateFlyway(prior)!=1 || AuthIdentityMigrationHistory.validateFlyway(latest)!=2
        || AuthIdentityMigrationHistory.validateCanonical(List.of(Map.of("version",35L,"dirty",false)))!=2)
      throw new AssertionError("wrong catalog generation selected");
    byte[] sql;
    try(var input=AuthIdentityDdlGateSourceTest.class.getResourceAsStream("/db/migration/V34__account_lifecycle_source.sql")) { sql=input.readAllBytes(); }
    var last=profile.path("flyway").get(profile.path("flyway").size()-1);
    String hash=HexFormat.of().formatHex(java.security.MessageDigest.getInstance("SHA-256").digest(sql));
    if(!hash.equals(last.path("sha256").asText()))throw new AssertionError("source hash drift");
    var crc=new java.util.zip.CRC32();
    new String(sql,StandardCharsets.UTF_8).lines().forEach(line->crc.update(line.getBytes(StandardCharsets.UTF_8)));
    if((int)crc.getValue()!=last.path("checksum").asInt())throw new AssertionError("source checksum drift");
    latest.getLast().put("checksum",0);deny(()->AuthIdentityMigrationHistory.validateFlyway(latest));
  }
  static final class HistoryJdbc extends JdbcTemplate {
    final boolean exclusive; final List<String> calls=new ArrayList<>();
    HistoryJdbc(boolean exclusive){this.exclusive=exclusive;}
    @Override public <T>T queryForObject(String sql,Class<T> type){calls.add(sql);return type.cast(exclusive);}
    @Override public void execute(String sql){calls.add(sql);}
    @Override public List<Map<String,Object>> queryForList(String sql){calls.add(sql);return List.of(Map.of("version",32L,"dirty",false));}
  }
  public static void main(String[] args) throws Exception {
    if (!System.getProperty("os.name").equals("Linux")) throw new AssertionError("Linux required");
    var tests=new AuthIdentityDdlGateSourceTest();
    tests.exactGateAcceptedAndEveryProtectionRequired();
    tests.realLoaderProfilesNeverMixOrRepairHistory();
    System.out.println("PASS 2 gate/history source-unit methods (14 gate negatives, 14 history/mixed-loader negatives); no SQL executed");
  }
}
