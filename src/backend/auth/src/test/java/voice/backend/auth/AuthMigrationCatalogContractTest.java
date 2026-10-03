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
    assertThat(sources).hasSize(26);
    for (int version = 1; version <= 26; version++) assertThat(sources).containsKey(version);
    assertThat(sources.get(15).getFileName().toString()).isEqualTo("V15__refresh_tokens_profile_id.sql");
  }

  @Test
  void golangHasOneUpAndDownPerVersionAndKeepsCanonicalRefreshAt16() throws Exception {
    var up = catalog(golangDirectory(), Pattern.compile("(\\d+)_.*\\.up\\.sql"));
    var down = catalog(golangDirectory(), Pattern.compile("(\\d+)_.*\\.down\\.sql"));
    assertThat(up).hasSize(27);
    assertThat(down).hasSize(27);
    for (int version = 1; version <= 27; version++) {
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
}
