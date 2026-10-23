package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.regex.Pattern;
import org.junit.jupiter.api.Test;

/** Full loader catalogs, including SDK revisions added after the shared master base. */
class AuthMigrationCatalogContractTest {
  static Path repositoryRoot() {
    return GuestConversionDurabilityMigrationContractTest.repositoryRoot();
  }

  static Path flywayDirectory() {
    return repositoryRoot().resolve("src/backend/auth/src/main/resources/db/migration");
  }

  static Path golangDirectory() {
    return repositoryRoot().resolve("src/backend/migrations/auth_db");
  }

  static Map<Integer, Path> catalog(Path directory, Pattern pattern) throws Exception {
    var catalog = new LinkedHashMap<Integer, Path>();
    try (var files = Files.list(directory)) {
      for (var file : files.sorted().toList()) {
        var matcher = pattern.matcher(file.getFileName().toString());
        if (!matcher.matches()) continue;
        int version = Integer.parseInt(matcher.group(1));
        assertThat(catalog.putIfAbsent(version, file))
            .as("migration version %s must have exactly one source in %s", version, directory)
            .isNull();
      }
    }
    return catalog;
  }

  @Test
  void flywayHasOneSourcePerVersionAndKeepsCanonicalRefreshAt15() throws Exception {
    var sources = catalog(flywayDirectory(), Pattern.compile("V(\\d+)__.*\\.sql"));
    assertThat(sources).hasSize(28);
    for (int version = 1; version <= 26; version++) assertThat(sources).containsKey(version);
    assertThat(sources).containsKeys(28,31).doesNotContainKeys(27,29,30);
    assertThat(sources.get(15).getFileName().toString()).isEqualTo("V15__refresh_tokens_profile_id.sql");
  }

  @Test
  void golangHasOneUpAndDownPerVersionAndKeepsCanonicalRefreshAt16() throws Exception {
    var up = catalog(golangDirectory(), Pattern.compile("(\\d+)_.*\\.up\\.sql"));
    var down = catalog(golangDirectory(), Pattern.compile("(\\d+)_.*\\.down\\.sql"));
    assertThat(up).hasSize(29);
    assertThat(down).hasSize(29);
    var versions = new java.util.ArrayList<Integer>();
    for (int version = 1; version <= 27; version++) versions.add(version);
    versions.addAll(java.util.List.of(29,32));
    assertThat(up).doesNotContainKeys(28,30,31);
    for (int version : versions) {
      assertThat(up).containsKey(version);
      assertThat(down).containsKey(version);
      assertThat(up.get(version).getFileName().toString().replace(".up.sql", ""))
          .isEqualTo(down.get(version).getFileName().toString().replace(".down.sql", ""));
    }
    assertThat(up.get(16).getFileName().toString()).isEqualTo("000016_refresh_tokens_profile_id.up.sql");
  }

  @Test
  void bothSupportedCatalogsContainTheSameEntireOrderedSdkDdl() throws Exception {
    var flyway = catalog(flywayDirectory(), Pattern.compile("V(\\d+)__.*\\.sql"));
    var golang = catalog(golangDirectory(), Pattern.compile("(\\d+)_.*\\.up\\.sql"));
    for (int version = 16; version <= 26; version++) {
      assertThat(flyway).containsKey(version);
      assertThat(golang).containsKey(version + 1);
      assertThat(Files.readString(golang.get(version + 1), StandardCharsets.UTF_8).replace("\r\n", "\n"))
          .as("SDK DDL must match at Flyway %s and golang-migrate %s", version, version + 1)
          .isEqualTo(Files.readString(flyway.get(version), StandardCharsets.UTF_8).replace("\r\n", "\n"));
    }
  }

  @Test void identityPairsPreserveOwnedBytesAndCanonicalTransactionEnvelope() throws Exception {
    var flyway = catalog(flywayDirectory(), Pattern.compile("V(\\d+)__.*\\.sql"));
    var canonical = catalog(golangDirectory(), Pattern.compile("(\\d+)_.*\\.up\\.sql"));
    String owner = Files.readString(flyway.get(28)).replace("\r\n", "\n");
    assertThat(Files.readString(canonical.get(29)).replace("\r\n", "\n")).isEqualTo(owner);
    String gate = Files.readString(flyway.get(31)).replace("\r\n", "\n");
    assertThat(Files.readString(canonical.get(32)).replace("\r\n", "\n")).isEqualTo("BEGIN;\n" + gate + "COMMIT;\n");
    assertThat(gate.indexOf("LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS EXCLUSIVE MODE"))
        .isLessThan(gate.indexOf("CREATE FUNCTION"));
    assertThat(gate).contains("SECURITY INVOKER", "ACCESS EXCLUSIVE MODE NOWAIT", "CREATE EVENT TRIGGER");
    assertThat(gate).doesNotContain("SECURITY DEFINER", "pg_proc SET", "flyway_schema_history", "schema_migrations");
    assertThat(Files.readString(golangDirectory().resolve("000032_auth_identity_ddl_barrier.down.sql")))
        .contains("ERRCODE='55000'").doesNotContain("DROP EVENT TRIGGER", "DROP TABLE");
  }

  public static void main(String[] args) throws Exception {
    if (!System.getProperty("os.name").equals("Linux")) throw new AssertionError("Linux required");
    var suite = new AuthMigrationCatalogContractTest();
    suite.flywayHasOneSourcePerVersionAndKeepsCanonicalRefreshAt15();
    suite.golangHasOneUpAndDownPerVersionAndKeepsCanonicalRefreshAt16();
    suite.bothSupportedCatalogsContainTheSameEntireOrderedSdkDdl();
    suite.identityPairsPreserveOwnedBytesAndCanonicalTransactionEnvelope();
    System.out.println("PASS 4 migration source-catalog contracts; no loader or SQL execution");
  }
}
