package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.Map;
import java.util.UUID;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.testcontainers.containers.BindMode;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.Network;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.containers.startupcheck.OneShotStartupCheckStrategy;
import org.testcontainers.utility.DockerImageName;

/** Actual default Flyway and pinned operational loader; no manual sorted-SQL substitute. */
class AuthMigrationCatalogJdbcIntegrationTest {
  @TempDir Path temporary;

  private static PostgreSQLContainer<?> postgres(Network network) {
    return new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
        .withNetwork(network).withNetworkAliases("auth-catalog-postgres")
        .withDatabaseName("auth_db").withUsername("voice").withPassword("voice")
        .withLabel("voice.task", "auth-migration-catalog-20261002").withReuse(false);
  }

  private static JdbcTemplate jdbc(PostgreSQLContainer<?> postgres) {
    return new JdbcTemplate(new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword()));
  }

  private static Flyway flyway(PostgreSQLContainer<?> postgres, String location) {
    return Flyway.configure().dataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())
        .locations(location).load();
  }

  private static void migrate(PostgreSQLContainer<?> postgres, Network network, Path directory, String... command) {
    var args = new java.util.ArrayList<String>();
    args.addAll(java.util.List.of("-path", "/migrations", "-database",
        "postgres://voice:voice@auth-catalog-postgres:5432/auth_db?sslmode=disable"));
    args.addAll(java.util.List.of(command));
    try (var loader = new GenericContainer<>(DockerImageName.parse("migrate/migrate:v4.18.1"))
        .withNetwork(network).withFileSystemBind(directory.toAbsolutePath().toString(), "/migrations", BindMode.READ_ONLY)
        .withCommand(args.toArray(String[]::new))
        .withStartupCheckStrategy(new OneShotStartupCheckStrategy().withTimeout(Duration.ofSeconds(45)))
        .withLabel("voice.task", "auth-migration-catalog-20261002")) {
      loader.start();
    }
  }

  private static String authorizationCatalog(JdbcTemplate jdbc) {
    // Never print values of identity/provider/token/proof/receipt rows.
    return jdbc.queryForObject("""
        SELECT jsonb_agg(jsonb_build_array(c.relname,a.attname,format_type(a.atttypid,a.atttypmod),
                 a.attnotnull,pg_get_expr(d.adbin,d.adrelid)) ORDER BY c.relname,a.attnum)::text
        FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
        JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
        LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum
        WHERE n.nspname='public' AND c.relkind='r'
          AND (c.relname LIKE 'sdk_%' OR c.relname='refresh_tokens' AND a.attname='profile_id')
        """, String.class);
  }

  @Test
  void freshFlywayAndPinnedGolangMigrateLoadTheEntireSdkCatalog() throws Exception {
    String flywayCatalog;
    try (var network = Network.newNetwork(); var postgres = postgres(network)) {
      postgres.start();
      var loader = flyway(postgres, "classpath:db/migration");
      assertThat(loader.migrate().migrationsExecuted).isEqualTo(26);
      assertThat(loader.info().current().getVersion().getVersion()).isEqualTo("26");
      flywayCatalog = authorizationCatalog(jdbc(postgres));
      assertThat(loader.migrate().migrationsExecuted).isZero();
    }
    try (var network = Network.newNetwork(); var postgres = postgres(network)) {
      postgres.start();
      migrate(postgres, network, AuthMigrationCatalogContractTest.golangDirectory(), "up");
      var database = jdbc(postgres);
      assertThat(database.queryForObject("SELECT version FROM schema_migrations WHERE NOT dirty", Long.class)).isEqualTo(27);
      assertThat(authorizationCatalog(database)).isEqualTo(flywayCatalog);
      migrate(postgres, network, AuthMigrationCatalogContractTest.golangDirectory(), "up");
      try(var reader=new voice.backend.auth.authoritysource.AuthAuthorityReader(
          new DriverManagerDataSource(postgres.getJdbcUrl(),postgres.getUsername(),postgres.getPassword()))) {
        reader.checkSchema();
        assertThat(reader.snapshot(java.util.List.of()).revision()).isEqualTo(1);
        database.execute("UPDATE schema_migrations SET dirty=true");
        assertThatThrownBy(reader::checkSchema).isInstanceOf(IllegalStateException.class);
      }
    }
  }

  private Path legacyFlyway() throws Exception {
    var legacy = Files.createDirectory(temporary.resolve("legacy-flyway"));
    try (var sources = Files.list(AuthMigrationCatalogContractTest.flywayDirectory())) {
      for (var source : sources.toList()) {
        String name = source.getFileName().toString();
        int version = Integer.parseInt(name.substring(1, name.indexOf("__")));
        if (version > 25) continue; // Preserve the actual old SDK-only history.
        if (version == 15) continue; // Historical SDK-only V15 omitted master's refresh addition.
        String target = version < 15 ? name : "V" + (version - 1) + name.substring(name.indexOf("__"));
        Files.copy(source, legacy.resolve(target));
      }
    }
    return legacy;
  }

  private static Map<String, Object> seedEvidence(JdbcTemplate jdbc) {
    UUID account = UUID.randomUUID(), actor = UUID.randomUUID(), application = UUID.randomUUID(), environment = UUID.randomUUID();
    jdbc.update("INSERT INTO sdk_identities(account_id,actor_id,application_id,environment_id,issuer,provider_subject,created_at) "
        + "VALUES(?,?,?,?,'fixture','private-fixture-subject',clock_timestamp())", account, actor, application, environment);
    UUID device = UUID.randomUUID();
    jdbc.update("INSERT INTO sdk_devices(device_id,account_id,thumbprint,public_jwk) VALUES(?,?,'fixture-thumbprint','fixture-public-key')", device, account);
    UUID operation = UUID.randomUUID();
    jdbc.update("INSERT INTO sdk_conversion_operations(operation_id,mode,source_account_id,device_id,public_jwk,application_id,environment_id,source_generation,binding_id,idempotency_key,request_hash,created_at) "
        + "VALUES(?,'new',?,?,'fixture-public-key',?,?,1,?,?,'fixture-request',clock_timestamp())",
        operation, account, device, application, environment, UUID.randomUUID(), UUID.randomUUID());
    if (hasOwnerReceipts(jdbc)) {
      jdbc.update("INSERT INTO sdk_conversion_owner_receipts(operation_id,owner,stage,request_hash,receipt_id,receipt_json,created_at) "
          + "VALUES(?,'gis','freeze',?,?,?::jsonb,clock_timestamp())", operation, "a".repeat(64), UUID.randomUUID(), "{\"fixture\":1}");
    }
    return evidence(jdbc);
  }

  private static Map<String, Object> evidence(JdbcTemplate jdbc) {
    var rows = new java.util.LinkedHashMap<String, Object>(jdbc.queryForMap("""
        SELECT (SELECT jsonb_agg(to_jsonb(i) ORDER BY account_id) FROM sdk_identities i)::text AS identities,
               (SELECT jsonb_agg(to_jsonb(d) ORDER BY device_id) FROM sdk_devices d)::text AS devices,
               (SELECT jsonb_agg(to_jsonb(o) ORDER BY operation_id) FROM sdk_conversion_operations o)::text AS operations
        """));
    if (hasOwnerReceipts(jdbc)) {
      rows.put("receipts", jdbc.queryForObject("SELECT jsonb_agg(to_jsonb(r) ORDER BY operation_id,owner,stage,generation)::text FROM sdk_conversion_owner_receipts r", String.class));
    }
    return rows;
  }

  private static boolean hasOwnerReceipts(JdbcTemplate jdbc) {
    return Boolean.TRUE.equals(jdbc.queryForObject("SELECT to_regclass('public.sdk_conversion_owner_receipts') IS NOT NULL", Boolean.class));
  }

  @Test
  void historicalSdkFlywayHistoryRefusesUpgradeWithoutChangingRowsOrHistory() throws Exception {
    try (var network = Network.newNetwork(); var postgres = postgres(network)) {
      postgres.start();
      flyway(postgres, "filesystem:" + legacyFlyway()).migrate();
      var database = jdbc(postgres);
      var before = seedEvidence(database);
      String history = database.queryForObject("SELECT jsonb_agg(to_jsonb(h) ORDER BY installed_rank)::text FROM flyway_schema_history h", String.class);
      assertThatThrownBy(() -> flyway(postgres, "classpath:db/migration").migrate())
          .isInstanceOf(org.flywaydb.core.api.exception.FlywayValidateException.class);
      assertThat(evidence(database).equals(before)).as("failed historical upgrade preserves all saved authority/receipt rows").isTrue();
      assertThat(database.queryForObject("SELECT jsonb_agg(to_jsonb(h) ORDER BY installed_rank)::text FROM flyway_schema_history h", String.class).equals(history))
          .as("validation refusal must not repair or shift migration history").isTrue();
    }
  }

  @Test
  void historicalSdkGolangVersionRefusesUpgradeAndPreservesExistingAuthorityState() throws Exception {
    var legacy = Files.createDirectory(temporary.resolve("legacy-golang"));
    try (var sources = Files.list(AuthMigrationCatalogContractTest.golangDirectory())) {
      for (var source : sources.toList()) {
        String name = source.getFileName().toString();
        int version = Integer.parseInt(name.substring(0, name.indexOf('_')));
        if (version == 16 || version > 19) continue;
        String target = version < 16 ? name : String.format("%06d", version - 1) + name.substring(name.indexOf('_'));
        Files.copy(source, legacy.resolve(target));
      }
    }
    try (var network = Network.newNetwork(); var postgres = postgres(network)) {
      postgres.start();
      migrate(postgres, network, legacy, "up");
      var database = jdbc(postgres);
      var before = seedEvidence(database);
      assertThat(database.queryForObject("SELECT version FROM schema_migrations WHERE NOT dirty", Long.class)).isEqualTo(18);
      assertThatThrownBy(() -> migrate(postgres, network, AuthMigrationCatalogContractTest.golangDirectory(), "up"))
          .isInstanceOf(org.testcontainers.containers.ContainerLaunchException.class);
      assertThat(database.queryForObject("SELECT version FROM schema_migrations WHERE dirty", Long.class)).isEqualTo(19);
      assertThat(evidence(database).equals(before)).as("historical failure preserves identity, device and conversion state").isTrue();
      assertThat(hasOwnerReceipts(database)).isFalse();
      assertThat(database.queryForObject("SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='refresh_tokens' AND column_name='profile_id')", Boolean.class))
          .as("refusal must not fabricate the canonical migration missing from legacy history").isFalse();
    }
  }

  @Test
  void sdkDowngradeRefusesWithDirtyMaintenanceStateAndRetainsAllEvidence() {
    try (var network = Network.newNetwork(); var postgres = postgres(network)) {
      postgres.start();
      migrate(postgres, network, AuthMigrationCatalogContractTest.golangDirectory(), "up");
      var database = jdbc(postgres);
      var before = seedEvidence(database);
      assertThatThrownBy(() -> migrate(postgres, network, AuthMigrationCatalogContractTest.golangDirectory(), "down", "1"))
          .isInstanceOf(org.testcontainers.containers.ContainerLaunchException.class);
      assertThat(database.queryForObject("SELECT version FROM schema_migrations WHERE dirty", Long.class)).isEqualTo(26);
      assertThat(evidence(database).equals(before)).as("refused downgrade must retain all authority and protected receipt bytes").isTrue();
    }
  }
}
