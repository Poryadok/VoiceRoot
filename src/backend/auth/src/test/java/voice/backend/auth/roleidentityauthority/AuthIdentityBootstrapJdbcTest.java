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
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.transaction.support.TransactionTemplate;
import voice.backend.auth.repository.JdbcAccountRepository;

@EnabledIfEnvironmentVariable(named = "AUTH_ROLE_IDENTITY_TEST_JDBC_URL", matches = ".+")
class AuthIdentityBootstrapJdbcTest {
  static DriverManagerDataSource source;
  static JdbcTemplate jdbc;
  static byte[] key(byte value) { byte[] key = new byte[32]; Arrays.fill(key, value); return key; }
  static final RoleIdentitySelectors KEYS = new RoleIdentitySelectors(1, 2, Map.of(1, key((byte) 81), 2, key((byte) 82)));
  @BeforeAll static void migrate() {
    assertEquals("Linux", System.getProperty("os.name"));
    source = new DriverManagerDataSource(System.getenv("AUTH_ROLE_IDENTITY_TEST_JDBC_URL"), "postgres", "local-proof");
    var locations = new java.util.ArrayList<>(java.util.List.of("filesystem:src/main/resources/db/migration"));
    String compatible = System.getenv("AUTH_ROLE_IDENTITY_COMPAT_MIGRATIONS");
    if (compatible != null) locations.add("filesystem:" + compatible);
    Flyway.configure().dataSource(source).locations(locations.toArray(String[]::new)).load().migrate();
    jdbc = new JdbcTemplate(source);
    new AuthIdentityKeysetBootstrap(source, KEYS).initialize();
  }
  @Test void registrationIsIdempotentAndBothRepositoryBirthsCaptureBeforeEnforcement() {
    long revision = jdbc.queryForObject("SELECT through_revision FROM role_identity_owner_state", Long.class);
    new AuthIdentityKeysetBootstrap(source, KEYS).initialize();
    assertEquals(revision, jdbc.queryForObject("SELECT through_revision FROM role_identity_owner_state", Long.class));
    assertTrue(jdbc.queryForObject("SELECT enforcement_enabled FROM role_identity_owner_state", Boolean.class));
    var accounts = new JdbcAccountRepository(new NamedParameterJdbcTemplate(source), KEYS);
    var regular = accounts.create(UUID.randomUUID() + "@example.test", null, "fixture", "regular");
    var pending = accounts.createRegularEmailPending(UUID.randomUUID() + "@example.test", "fixture");
    for (var account : java.util.List.of(regular, pending)) {
      assertTrue(jdbc.queryForObject("SELECT l.selector_complete FROM accounts a JOIN role_identity_lifetimes l "
          + "ON l.owner_handle=a.role_identity_handle WHERE a.id=?", Boolean.class, account.id()));
    }
  }
  @Test void unavailableRetainedKeyOrReplacedFingerprintCannotAdvanceCatalog() {
    long revision = jdbc.queryForObject("SELECT through_revision FROM role_identity_owner_state", Long.class);
    for (var invalid : java.util.List.of(
        new RoleIdentitySelectors(2, 2, Map.of(2, key((byte) 82))),
        new RoleIdentitySelectors(2, 2, Map.of(1, key((byte) 83), 2, key((byte) 82))))) {
      assertThrows(RuntimeException.class, () -> new AuthIdentityKeysetBootstrap(source, invalid).initialize());
      assertEquals(revision, jdbc.queryForObject("SELECT through_revision FROM role_identity_owner_state", Long.class));
      assertEquals(1L, jdbc.queryForObject("SELECT keyset_revision FROM role_identity_owner_state", Long.class));
    }
    assertThrows(RuntimeException.class, () -> jdbc.update("INSERT INTO accounts(id,email,password_hash,type,status) "
        + "VALUES(?,?,'fixture','regular','active')", UUID.randomUUID(), UUID.randomUUID() + "@example.test"));
  }
  @Test void startupRegistrationCannotJoinAnAmbientSecurityTransaction() {
    var transaction = new TransactionTemplate(new DataSourceTransactionManager(source));
    transaction.executeWithoutResult(status -> assertThrows(IllegalStateException.class,
        () -> new AuthIdentityKeysetBootstrap(source, KEYS).initialize()));
  }
}
