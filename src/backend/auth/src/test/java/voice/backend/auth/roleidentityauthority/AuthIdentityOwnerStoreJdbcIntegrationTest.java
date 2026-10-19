package voice.backend.auth.roleidentityauthority;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.sql.Timestamp;
import java.time.Instant;
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
import voice.identityerasure.v1.AccountLifecycleState;
import voice.identityerasure.v1.Eligibility;
import voice.identityerasure.v1.ErasureDisposition;

/** Real owner facts and key/transaction boundaries, independent of transport mocks. */
@EnabledIfEnvironmentVariable(named = "AUTH_ROLE_IDENTITY_TEST_JDBC_URL", matches = ".+")
class AuthIdentityOwnerStoreJdbcIntegrationTest {
  private static DriverManagerDataSource source;
  private static JdbcTemplate jdbc;
  private static final byte[] KEY = new byte[32];
  static { java.util.Arrays.fill(KEY, (byte) 19); }
  private static final RoleIdentitySelectors SELECTORS = new RoleIdentitySelectors(1, 1, Map.of(1, KEY));

  @BeforeAll
  static void migrate() {
    assertThat(System.getProperty("os.name")).isEqualTo("Linux");
    source = new DriverManagerDataSource(System.getenv("AUTH_ROLE_IDENTITY_TEST_JDBC_URL"), "postgres", "local-proof");
    var locations = new java.util.ArrayList<>(java.util.List.of("filesystem:src/main/resources/db/migration"));
    String compatible = System.getenv("AUTH_ROLE_IDENTITY_COMPAT_MIGRATIONS");
    if (compatible != null) locations.add("filesystem:" + compatible);
    Flyway.configure().dataSource(source).locations(locations.toArray(String[]::new)).load().migrate();
    jdbc = new JdbcTemplate(source);
    jdbc.queryForObject("SELECT role_identity_register_keyset(1,1,jsonb_build_array("
        + "jsonb_build_object('version',1,'fingerprint',encode(sha256(decode(repeat('13',32),'hex')),'hex'))))", Long.class);
    jdbc.update("UPDATE role_identity_owner_state SET enforcement_enabled=true");
  }

  private static UUID account() {
    return new JdbcAccountRepository(new NamedParameterJdbcTemplate(source), SELECTORS)
        .create(UUID.randomUUID() + "@example.invalid", null, "fixture", "regular").id();
  }

  private static Instant instant(com.google.protobuf.Timestamp timestamp) {
    return Instant.ofEpochSecond(timestamp.getSeconds(), timestamp.getNanos());
  }

  @Test
  void parentFactUsesExactOpaqueHandleAndCurrentEligibilityWithoutInventingErasure() {
    UUID id = account();
    var store = new AuthIdentityOwnerStore(source, SELECTORS);
    var active = store.resolveAccount(id);
    assertThat(active.accountId()).isEqualTo(id);
    assertThat(active.lifecycle().getOwnerHandle()).isEqualTo(jdbc.queryForObject(
        "SELECT role_identity_handle::text FROM accounts WHERE id=?", String.class, id));
    assertThat(active.lifecycle().getState()).isEqualTo(AccountLifecycleState.ACCOUNT_LIFECYCLE_STATE_ACTIVE);
    assertThat(active.lifecycle().getEligibility()).isEqualTo(Eligibility.ELIGIBILITY_ELIGIBLE);
    assertThat(active.lifecycle().getDisposition()).isEqualTo(ErasureDisposition.ERASURE_DISPOSITION_NOT_IRREVERSIBLE);
    assertThat(active.lifecycle().getOwnerCut().getComplete()).isTrue();
    assertThat(active.lifecycle().getOwnerCut().getOwner()).isEqualTo("auth");
    assertThat(active.lifecycle().getOwnerCut().getSelectorKeyVersionsList()).containsExactly("1");
    assertThat(active.lifecycle().getOwnerCut().getDigest().size()).isEqualTo(32);
    assertThat(instant(active.window().getValidUntil())).isAfter(instant(active.window().getObservedAt()));
    jdbc.update("UPDATE accounts SET status='suspended' WHERE id=?", id);
    var suspended = store.resolveAccount(id).lifecycle();
    assertThat(suspended.getEligibility()).isEqualTo(Eligibility.ELIGIBILITY_INELIGIBLE);
    assertThat(suspended.getDisposition()).isEqualTo(ErasureDisposition.ERASURE_DISPOSITION_NOT_IRREVERSIBLE);
    assertThat(suspended.getLifecycleRevision()).isGreaterThan(active.lifecycle().getLifecycleRevision());
  }

  @Test
  void terminalParentSurvivesRawErasureWithActualCanonicalCycleAndDeadline() {
    UUID id = account();
    UUID handle = jdbc.queryForObject("SELECT role_identity_handle FROM accounts WHERE id=?", UUID.class, id);
    jdbc.update("UPDATE accounts SET status='deleted',deleted_at=clock_timestamp()-INTERVAL '31 days' WHERE id=?", id);
    Instant deadline = jdbc.queryForObject("SELECT erase_at FROM role_identity_lifetimes WHERE owner_handle=?", Timestamp.class, handle).toInstant();
    UUID cycle = jdbc.queryForObject("SELECT deletion_cycle FROM role_identity_lifetimes WHERE owner_handle=?", UUID.class, handle);
    jdbc.update("DELETE FROM accounts WHERE id=?", id);
    var observed = new AuthIdentityOwnerStore(source, SELECTORS).resolveAccount(id);
    var terminal = observed.lifecycle();
    assertThat(terminal.getOwnerHandle()).isEqualTo(handle.toString());
    assertThat(terminal.getState()).isEqualTo(AccountLifecycleState.ACCOUNT_LIFECYCLE_STATE_TERMINAL);
    assertThat(terminal.getDisposition()).isEqualTo(ErasureDisposition.ERASURE_DISPOSITION_IRREVERSIBLE);
    assertThat(terminal.getEligibility()).isEqualTo(Eligibility.ELIGIBILITY_INELIGIBLE);
    assertThat(terminal.getDeletionCycle()).isEqualTo(cycle.toString());
    assertThat(instant(terminal.getEraseAt())).isEqualTo(deadline).isBeforeOrEqualTo(instant(observed.window().getObservedAt()));
    assertThat(terminal.getOwnerCut().getComplete()).isTrue();
  }

  @Test
  void absentWithoutHistoryIsExplicitlyIncompleteAndNeverTerminal() {
    UUID unknown = UUID.randomUUID();
    var observed = new AuthIdentityOwnerStore(source, SELECTORS).resolveAccount(unknown);
    assertThat(observed.accountId()).isEqualTo(unknown);
    assertThat(observed.lifecycle().getDisposition()).isEqualTo(ErasureDisposition.ERASURE_DISPOSITION_INCOMPLETE);
    assertThat(observed.lifecycle().getEligibility()).isEqualTo(Eligibility.ELIGIBILITY_UNAVAILABLE);
    assertThat(observed.lifecycle().getOwnerHandle()).isEmpty();
    assertThat(observed.lifecycle().getOwnerCut().getComplete()).isFalse();
  }

  @Test
  void mismatchedLoadedKeyIsUnavailableRatherThanFalseAbsent() {
    UUID id = account();
    byte[] wrongKey = new byte[32];
    java.util.Arrays.fill(wrongKey, (byte) 20);
    var wrong = new RoleIdentitySelectors(1, 1, Map.of(1, wrongKey));
    assertThatThrownBy(() -> new AuthIdentityOwnerStore(source, wrong).resolveAccount(id))
        .isInstanceOf(IllegalStateException.class).hasMessage("identity owner keyset unavailable");
  }

  @Test
  void parentAuthorityRefusesAnAmbientCallerTransaction() {
    UUID id = account();
    var caller = new TransactionTemplate(new DataSourceTransactionManager(source));
    caller.executeWithoutResult(status -> assertThatThrownBy(
        () -> new AuthIdentityOwnerStore(source, SELECTORS).resolveAccount(id))
        .isInstanceOf(IllegalStateException.class)
        .hasMessage("identity owner observation requires independent transaction"));
  }
}
