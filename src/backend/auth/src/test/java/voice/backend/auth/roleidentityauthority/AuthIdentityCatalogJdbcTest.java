package voice.backend.auth.roleidentityauthority;

import static org.junit.jupiter.api.Assertions.*;

import java.util.Arrays;
import java.util.Map;
import java.util.UUID;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;

@EnabledIfEnvironmentVariable(named = "AUTH_ROLE_IDENTITY_TEST_JDBC_URL", matches = ".+")
class AuthIdentityCatalogJdbcTest {
  static DriverManagerDataSource source;
  static JdbcTemplate jdbc;
  static RoleIdentitySelectors keys;

  @BeforeAll static void migrate() {
    assertEquals("Linux", System.getProperty("os.name"));
    source = new DriverManagerDataSource(System.getenv("AUTH_ROLE_IDENTITY_TEST_JDBC_URL"), "postgres", "local-proof");
    var locations = new java.util.ArrayList<>(java.util.List.of("filesystem:src/main/resources/db/migration"));
    String compatible = System.getenv("AUTH_ROLE_IDENTITY_COMPAT_MIGRATIONS");
    if (compatible != null) locations.add("filesystem:" + compatible);
    Flyway.configure().dataSource(source).locations(locations.toArray(String[]::new)).load().migrate();
    jdbc = new JdbcTemplate(source);
    byte[] key = new byte[32]; Arrays.fill(key, (byte) 91);
    keys = new RoleIdentitySelectors(1, 1, Map.of(1, key));
    new AuthIdentityKeysetBootstrap(source, keys).initialize();
  }

  private static void rejectsBoth(String corrupt, String restore) {
    jdbc.execute(corrupt);
    try {
      assertThrows(RuntimeException.class, () -> new AuthIdentityOwnerStore(source, keys).resolveAccount(UUID.randomUUID()));
      assertThrows(RuntimeException.class, () -> new AuthIdentityKeysetBootstrap(source, keys).initialize());
    } finally {
      jdbc.execute(restore);
    }
  }

  @Test void disabledOwnerTriggerCannotIssueAnObservationOrEnableCapture() {
    rejectsBoth("ALTER TABLE accounts DISABLE TRIGGER auth_role_identity_account_guard",
        "ALTER TABLE accounts ENABLE TRIGGER auth_role_identity_account_guard");
  }

  @Test void alteredOwnerFunctionCannotIssueAnObservationOrEnableCapture() {
    String original = jdbc.queryForObject("SELECT pg_get_functiondef('public.role_identity_materialize_handle(uuid)'::regprocedure)", String.class);
    rejectsBoth("CREATE OR REPLACE FUNCTION public.role_identity_materialize_handle(p_handle uuid) RETURNS bigint "
        + "LANGUAGE plpgsql AS $$ BEGIN RETURN 0; END $$", original);
  }

  @Test void changedFunctionSecurityIsRejectedEvenWhenItsBodyIsUnchanged() {
    rejectsBoth("ALTER FUNCTION role_identity_materialize_handle(uuid) SECURITY DEFINER",
        "ALTER FUNCTION role_identity_materialize_handle(uuid) SECURITY INVOKER");
  }

  @Test void missingCaptureConstraintAndNullableHandleAreRejected() {
    String original = jdbc.queryForObject("SELECT pg_get_constraintdef(oid) FROM pg_constraint "
        + "WHERE conrelid='accounts'::regclass AND conname='accounts_role_identity_capture_shape'", String.class);
    rejectsBoth("ALTER TABLE accounts DROP CONSTRAINT accounts_role_identity_capture_shape",
        "ALTER TABLE accounts ADD CONSTRAINT accounts_role_identity_capture_shape " + original);
    rejectsBoth("ALTER TABLE accounts ALTER COLUMN role_identity_handle DROP NOT NULL",
        "ALTER TABLE accounts ALTER COLUMN role_identity_handle SET NOT NULL");
  }

  @Test void ownerRowSecurityAndDirtyOwnedHistoryAreRejected() {
    rejectsBoth("ALTER TABLE role_identity_lifetimes ENABLE ROW LEVEL SECURITY",
        "ALTER TABLE role_identity_lifetimes DISABLE ROW LEVEL SECURITY");
    int checksum = jdbc.queryForObject("SELECT checksum FROM flyway_schema_history WHERE version='28'", Integer.class);
    rejectsBoth("UPDATE flyway_schema_history SET checksum=checksum+1 WHERE version='28'",
        "UPDATE flyway_schema_history SET checksum=" + checksum + " WHERE version='28'");
  }

  @Test void compatibleAdditiveSchemaDoesNotBecomeAGlobalVersionGate() {
    jdbc.execute("CREATE TABLE identity_catalog_unrelated_extension (id uuid PRIMARY KEY)");
    try {
      new AuthIdentityKeysetBootstrap(source, keys).initialize();
      var result = new AuthIdentityOwnerStore(source, keys).resolveAccount(UUID.randomUUID());
      assertFalse(result.lifecycle().getOwnerCut().getComplete());
    } finally {
      jdbc.execute("DROP TABLE identity_catalog_unrelated_extension");
    }
  }

  @Test void failedDuplicateOwnedMigrationCannotHideBehindAValidRecord() {
    int rank = jdbc.queryForObject("SELECT max(installed_rank)+1 FROM flyway_schema_history", Integer.class);
    jdbc.update("INSERT INTO flyway_schema_history(installed_rank,version,description,type,script,checksum,installed_by,execution_time,success) "
        + "SELECT ?,'28',description,type,script,checksum,installed_by,execution_time,false "
        + "FROM flyway_schema_history WHERE version='28'", rank);
    try {
      assertThrows(RuntimeException.class, () -> new AuthIdentityOwnerStore(source, keys).resolveAccount(UUID.randomUUID()));
      assertThrows(RuntimeException.class, () -> new AuthIdentityKeysetBootstrap(source, keys).initialize());
    } finally {
      jdbc.update("DELETE FROM flyway_schema_history WHERE installed_rank=?", rank);
    }
  }
}
