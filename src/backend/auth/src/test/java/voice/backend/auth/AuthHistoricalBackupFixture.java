package voice.backend.auth;

import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.util.HexFormat;
import java.util.Map;
import java.util.TreeMap;
import org.flywaydb.core.Flyway;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.testcontainers.containers.PostgreSQLContainer;

/** Disposable, stopped-writer fixture only. Never an operational restore entrypoint. */
final class AuthHistoricalBackupFixture {
  private final PostgreSQLContainer<?> postgres;
  private final String catalogHash;
  private final String archiveHash;
  private final Map<String, String> rows;
  private boolean ready;

  private AuthHistoricalBackupFixture(PostgreSQLContainer<?> postgres, Path catalog) throws Exception {
    this.postgres = postgres;
    catalogHash = catalogHash(catalog);
    rows = snapshot(database("auth_db"));
    command("pg_dump", "-U", "voice", "-d", "auth_db", "-Fc", "--file=/tmp/auth-recovery.dump");
    archiveHash = archiveHash();
  }

  static AuthHistoricalBackupFixture capture(PostgreSQLContainer<?> postgres, Path catalog) throws Exception {
    // History originates in the actual target loader, never a synthesized baseline.
    var jdbc = new JdbcTemplate(new DriverManagerDataSource(url(postgres, "auth_db"), postgres.getUsername(), postgres.getPassword()));
    long expected;
    try (var paths = Files.list(catalog)) {
      expected = paths.filter(path -> path.getFileName().toString().endsWith(".sql")).count();
    }
    boolean exclusive = Boolean.TRUE.equals(jdbc.queryForObject(
        "SELECT to_regclass('public.schema_migrations') IS NULL", Boolean.class));
    var history = jdbc.queryForMap("SELECT count(*) AS total, count(*) FILTER (WHERE type='SQL' AND success AND checksum IS NOT NULL) AS sql FROM public.flyway_schema_history");
    if (!exclusive || expected == 0 || ((Number) history.get("total")).longValue() != expected
        || ((Number) history.get("sql")).longValue() != expected) {
      throw new IllegalStateException("recovery_history_not_target_sql");
    }
    loader(postgres, "auth_db", catalog).validate();
    return new AuthHistoricalBackupFixture(postgres, catalog);
  }

  JdbcTemplate restore(Path catalog) throws Exception {
    ready = false;
    if (!catalogHash.equals(catalogHash(catalog))) throw new IllegalStateException("recovery_catalog_mismatch");
    if (!archiveHash.equals(archiveHash())) throw new IllegalStateException("recovery_archive_mismatch");
    // createdb refuses an existing target; no clean, replace, repair or history UPDATE.
    command("createdb", "-U", "voice", "auth_recovery");
    command("pg_restore", "-U", "voice", "-d", "auth_recovery", "--exit-on-error", "--single-transaction", "/tmp/auth-recovery.dump");
    var restored = database("auth_recovery");
    if (!rows.equals(snapshot(restored))) throw new IllegalStateException("recovery_rows_mismatch");
    var loader = loader(postgres, "auth_recovery", catalog);
    loader.validate();
    if (loader.migrate().migrationsExecuted != 0 || !rows.equals(snapshot(restored))) {
      throw new IllegalStateException("recovery_history_mismatch");
    }
    ready = true; // Matching historical loader readiness only, never current application readiness.
    return restored;
  }

  boolean ready() { return ready; }

  private JdbcTemplate database(String name) {
    return new JdbcTemplate(new DriverManagerDataSource(url(postgres, name), postgres.getUsername(), postgres.getPassword()));
  }

  private static Flyway loader(PostgreSQLContainer<?> postgres, String name, Path catalog) {
    return Flyway.configure().dataSource(url(postgres, name), postgres.getUsername(), postgres.getPassword())
        .locations("filesystem:" + catalog.toAbsolutePath()).load();
  }

  private static String url(PostgreSQLContainer<?> postgres, String name) {
    return "jdbc:postgresql://" + postgres.getHost() + ":" + postgres.getMappedPort(5432) + "/" + name;
  }

  private static Map<String, String> snapshot(JdbcTemplate jdbc) {
    var result = new TreeMap<String, String>();
    for (String table : jdbc.queryForList("SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename", String.class)) {
      String quoted = "\"" + table.replace("\"", "\"\"") + "\"";
      result.put(table, jdbc.queryForObject("SELECT coalesce(jsonb_agg(row ORDER BY row::text),'[]'::jsonb)::text FROM (SELECT to_jsonb(t) AS row FROM public."
          + quoted + " t) data", String.class));
    }
    return result;
  }

  private static String catalogHash(Path catalog) throws Exception {
    var digest = MessageDigest.getInstance("SHA-256");
    try (var paths = Files.list(catalog)) {
      for (Path file : paths.filter(path -> path.getFileName().toString().endsWith(".sql")).sorted().toList()) {
        byte[] name = file.getFileName().toString().getBytes(java.nio.charset.StandardCharsets.UTF_8);
        byte[] content = Files.readAllBytes(file);
        digest.update(java.nio.ByteBuffer.allocate(8).putInt(name.length).putInt(content.length).array());
        digest.update(name);
        digest.update(content);
      }
    }
    return HexFormat.of().formatHex(digest.digest());
  }

  private String archiveHash() throws Exception {
    return command("sha256sum", "/tmp/auth-recovery.dump").split(" ")[0];
  }

  private String command(String... arguments) throws Exception {
    var result = postgres.execInContainer(arguments);
    if (result.getExitCode() != 0) throw new IllegalStateException("recovery_fixture_command_failed");
    return result.getStdout();
  }
}
