package voice.backend.auth.roleidentityauthority;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.sql.Connection;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;
import org.springframework.dao.DataAccessException;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import voice.backend.auth.repository.JdbcAccountRepository;

/** Native owner catalog, real row serialization and durable minimized facts. */
@EnabledIfEnvironmentVariable(named = "AUTH_ROLE_IDENTITY_TEST_JDBC_URL", matches = ".+")
class RoleIdentityOwnerJdbcIntegrationTest {
  private static DriverManagerDataSource source;
  private static JdbcTemplate jdbc;
  private static final byte[] KEY = new byte[32];
  static { java.util.Arrays.fill(KEY, (byte) 19); }
  private static final RoleIdentitySelectors SELECTORS =
      new RoleIdentitySelectors(1, 1, Map.of(1, KEY));

  @BeforeAll
  static void migrate() {
    assertThat(System.getProperty("os.name")).isEqualTo("Linux");
    source = new DriverManagerDataSource(
        System.getenv("AUTH_ROLE_IDENTITY_TEST_JDBC_URL"), "postgres", "local-proof");
    var locations = new java.util.ArrayList<>(List.of("filesystem:src/main/resources/db/migration"));
    String compatible = System.getenv("AUTH_ROLE_IDENTITY_COMPAT_MIGRATIONS");
    if (compatible != null) locations.add("filesystem:" + compatible);
    Flyway.configure().dataSource(source).locations(locations.toArray(String[]::new)).load().migrate();
    jdbc = new JdbcTemplate(source);
  }

  private static void keyset() {
    byte[] fingerprint;
    try {
      fingerprint = java.security.MessageDigest.getInstance("SHA-256").digest(KEY);
    } catch (java.security.NoSuchAlgorithmException impossible) {
      throw new IllegalStateException(impossible);
    }
    jdbc.queryForObject("SELECT role_identity_register_keyset(1,1,"
        + "jsonb_build_array(jsonb_build_object('version',1,'fingerprint',encode(?,'hex'))))",
        Long.class, fingerprint);
  }

  private static UUID account(JdbcTemplate connection, UUID id, UUID handle, boolean captured) {
    connection.update("INSERT INTO accounts(id,email,password_hash,type,status,role_identity_handle,"
        + "role_identity_selector_version,role_identity_account_selector,role_identity_keyset_revision) "
        + "VALUES(?,?,'fixture','regular','active',?,?,?,?)", id, id + "@example.invalid", handle,
        captured ? 1 : null, captured ? SELECTORS.activeAccount(id) : null, captured ? 1L : null);
    return id;
  }

  private static UUID account(UUID handle) {
    keyset();
    return account(jdbc, UUID.randomUUID(), handle, true);
  }

  @Test
  void differentEligibilityAtSameRevisionCannotShareJournalDigest() throws Exception {
    UUID handle = UUID.randomUUID();
    UUID id = account(handle);
    try (Connection connection = source.getConnection()) {
      connection.setAutoCommit(false);
      var branch = new JdbcTemplate(new SingleConnectionDataSource(connection, true));
      branch.queryForObject("SELECT id FROM accounts WHERE id=? FOR UPDATE", UUID.class, id);
      branch.queryForObject("SELECT owner_handle FROM role_identity_lifetimes WHERE owner_handle=? FOR UPDATE",
          UUID.class, handle);
      branch.queryForObject("SELECT through_revision FROM role_identity_owner_state FOR UPDATE", Long.class);
      var baseline = connection.setSavepoint();
      branch.update("UPDATE accounts SET status='suspended' WHERE id=?", id);
      byte[] ineligibleDigest = branch.queryForObject("SELECT head_digest FROM role_identity_owner_state", byte[].class);
      connection.rollback(baseline);
      branch.update("UPDATE accounts SET type='guest' WHERE id=?", id);
      byte[] eligibleDigest = branch.queryForObject("SELECT head_digest FROM role_identity_owner_state", byte[].class);
      assertThat(eligibleDigest).as("same revision and lifecycle state must still commit the distinct eligibility")
          .isNotEqualTo(ineligibleDigest);
      connection.rollback();
    }
  }

  @Test
  void committedJournalRetainsEligibilityForEveryLifecycleRevision() {
    UUID handle = UUID.randomUUID();
    UUID id = account(handle);
    assertThat(jdbc.queryForObject("SELECT eligible FROM role_identity_owner_journal "
        + "WHERE owner_handle=? ORDER BY revision DESC LIMIT 1", Boolean.class, handle)).isTrue();
    boolean moderation = Boolean.TRUE.equals(jdbc.queryForObject(
        "SELECT to_regprocedure('auth_reconcile_moderation(uuid,uuid,bigint,boolean)') IS NOT NULL", Boolean.class));
    if (moderation) {
      // V27 deliberately preserves unknown legacy suspension against raw clears.
      // Use its real owner projection for a reversible ban/unban fixture.
      jdbc.queryForObject("SELECT auth_reconcile_moderation(id,moderation_subject_id,1,true) FROM accounts WHERE id=?",
          Long.class, id);
    } else {
      jdbc.update("UPDATE accounts SET status='suspended' WHERE id=?", id);
    }
    assertThat(jdbc.queryForObject("SELECT eligible FROM role_identity_lifetimes WHERE owner_handle=?",
        Boolean.class, handle)).isFalse();
    assertThat(jdbc.queryForObject("SELECT eligible FROM role_identity_owner_journal "
        + "WHERE owner_handle=? ORDER BY revision DESC LIMIT 1", Boolean.class, handle)).isFalse();
    assertThat(jdbc.queryForObject("SELECT lifecycle_revision FROM role_identity_owner_journal "
        + "WHERE owner_handle=? ORDER BY revision DESC LIMIT 1", Long.class, handle)).isEqualTo(2);
    if (moderation) {
      jdbc.queryForObject("SELECT auth_reconcile_moderation(id,moderation_subject_id,2,false) FROM accounts WHERE id=?",
          Long.class, id);
    } else {
      jdbc.update("UPDATE accounts SET status='active' WHERE id=?", id);
    }
    assertThat(jdbc.queryForObject("SELECT eligible FROM role_identity_owner_journal "
        + "WHERE owner_handle=? ORDER BY revision DESC LIMIT 1", Boolean.class, handle)).isTrue();
  }

  @Test
  void repositoryCapturesBothBirthPathsBeforeEnforcedInsertCommits() {
    keyset();
    jdbc.update("UPDATE role_identity_owner_state SET enforcement_enabled=true");
    var repository = new JdbcAccountRepository(new NamedParameterJdbcTemplate(source), SELECTORS);
    var ordinary = repository.create(UUID.randomUUID() + "@example.invalid", null, "fixture", "regular");
    var pending = repository.createRegularEmailPending(UUID.randomUUID() + "@example.invalid", "fixture");
    assertThat(ordinary.type()).isEqualTo("regular");
    assertThat(pending.type()).isEqualTo("guest");
    assertThat(repository.isRegularEmailVerificationPending(pending.id())).isTrue();
    for (var created : List.of(ordinary, pending)) {
      UUID handle = jdbc.queryForObject("SELECT role_identity_handle FROM accounts WHERE id=?", UUID.class, created.id());
      assertThat(handle).isNotEqualTo(created.id());
      assertThat(jdbc.queryForObject("SELECT owner_handle FROM role_identity_account_selectors "
          + "WHERE key_version=1 AND selector=?", UUID.class, SELECTORS.activeAccount(created.id())))
          .isEqualTo(handle);
      assertThat(jdbc.queryForObject("SELECT selector_complete FROM role_identity_lifetimes WHERE owner_handle=?",
          Boolean.class, handle)).isTrue();
    }
  }

  @Test
  void birthAndCommittedJournalRollbackTogetherWithoutSequenceGap() throws Exception {
    UUID handle = UUID.randomUUID();
    UUID id = account(handle);
    assertThatThrownBy(() -> jdbc.update("INSERT INTO accounts(email,password_hash,type,status,role_identity_selector_version) "
        + "VALUES(?,'fixture','regular','active',1)", UUID.randomUUID() + "@example.invalid"))
        .isInstanceOf(DataAccessException.class);
    assertThat(jdbc.queryForObject("SELECT owner_handle FROM role_identity_account_selectors "
        + "WHERE key_version=1 AND selector=?", UUID.class, SELECTORS.activeAccount(id))).isEqualTo(handle);
    assertThat(jdbc.queryForObject("SELECT lifecycle_revision FROM role_identity_lifetimes WHERE owner_handle=?",
        Long.class, handle)).isEqualTo(1);
    long before = jdbc.queryForObject("SELECT through_revision FROM role_identity_owner_state", Long.class);
    UUID rolledBack = UUID.randomUUID();
    try (Connection connection = source.getConnection()) {
      connection.setAutoCommit(false);
      account(new JdbcTemplate(new SingleConnectionDataSource(connection, true)), UUID.randomUUID(), rolledBack, true);
      connection.rollback();
    }
    assertThat(jdbc.queryForObject("SELECT count(*) FROM role_identity_lifetimes WHERE owner_handle=?",
        Long.class, rolledBack)).isZero();
    assertThat(jdbc.queryForObject("SELECT through_revision FROM role_identity_owner_state", Long.class)).isEqualTo(before);
    assertThat(jdbc.queryForObject("SELECT count(*) FROM role_identity_owner_journal", Long.class)).isEqualTo(before);
  }

  @Test
  void repeatedDeleteCannotExtendCycleAndTimelyRestoreOpensANewCycleLater() {
    UUID handle = UUID.randomUUID();
    UUID id = account(handle);
    jdbc.update("UPDATE accounts SET status='deleted',deleted_at=clock_timestamp()-INTERVAL '29 days' WHERE id=?", id);
    UUID first = jdbc.queryForObject("SELECT deletion_cycle FROM role_identity_lifetimes WHERE owner_handle=?", UUID.class, handle);
    assertThatThrownBy(() -> jdbc.update("UPDATE accounts SET deleted_at=deleted_at+INTERVAL '1 day' WHERE id=?", id))
        .isInstanceOf(DataAccessException.class);
    jdbc.update("UPDATE accounts SET status='active',deleted_at=NULL WHERE id=?", id);
    assertThat(jdbc.queryForObject("SELECT state FROM role_identity_lifetimes WHERE owner_handle=?", String.class, handle)).isEqualTo("ACTIVE");
    jdbc.update("UPDATE accounts SET status='deleted',deleted_at=clock_timestamp() WHERE id=?", id);
    assertThat(jdbc.queryForObject("SELECT deletion_cycle FROM role_identity_lifetimes WHERE owner_handle=?", UUID.class, handle)).isNotEqualTo(first);
  }

  @Test
  void rawRestoreBlockedOnAccountCannotCrossDeadline() throws Exception {
    UUID handle = UUID.randomUUID();
    UUID id = account(handle);
    jdbc.update("UPDATE accounts SET status='deleted',deleted_at=clock_timestamp()-INTERVAL '30 days'+INTERVAL '8 seconds' WHERE id=?", id);
    Instant deadline = jdbc.queryForObject("SELECT erase_at FROM role_identity_lifetimes WHERE owner_handle=?", Timestamp.class, handle).toInstant();
    var worker = Executors.newSingleThreadExecutor();
    try (Connection blocker = source.getConnection()) {
      blocker.setAutoCommit(false);
      var locked = new JdbcTemplate(new SingleConnectionDataSource(blocker, true));
      locked.queryForObject("SELECT id FROM accounts WHERE id=? FOR UPDATE", UUID.class, id);
      int blockerPid = locked.queryForObject("SELECT pg_backend_pid()", Integer.class);
      var ready = new CountDownLatch(1);
      var pid = new AtomicInteger();
      var denied = worker.submit(() -> {
        try (Connection connection = source.getConnection()) {
          connection.setAutoCommit(false);
          var restore = new JdbcTemplate(new SingleConnectionDataSource(connection, true));
          pid.set(restore.queryForObject("SELECT pg_backend_pid()", Integer.class));
          assertThat(restore.queryForObject("SELECT CURRENT_TIMESTAMP", Timestamp.class).toInstant()).isBefore(deadline);
          ready.countDown();
          try {
            restore.update("UPDATE accounts SET status='active',deleted_at=NULL WHERE id=?", id);
            connection.commit();
            return false;
          } catch (DataAccessException expected) {
            connection.rollback();
            return true;
          }
        }
      });
      assertThat(ready.await(5, TimeUnit.SECONDS)).isTrue();
      long limit = System.nanoTime() + TimeUnit.SECONDS.toNanos(5);
      boolean blocked = false;
      while (System.nanoTime() < limit) {
        blocked = Boolean.TRUE.equals(locked.queryForObject("SELECT ?=ANY(pg_blocking_pids(?))", Boolean.class, blockerPid, pid.get()));
        if (blocked) break;
        Thread.sleep(10);
      }
      assertThat(blocked).as("actual raw restore waits on the exact account row").isTrue();
      locked.execute("SELECT pg_sleep_until('" + deadline + "'::timestamptz)");
      blocker.commit();
      assertThat(denied.get(10, TimeUnit.SECONDS)).isTrue();
    } finally {
      worker.shutdownNow();
      assertThat(worker.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
    jdbc.queryForObject("SELECT role_identity_materialize_handle(?)", Long.class, handle);
    assertThat(jdbc.queryForObject("SELECT state FROM role_identity_lifetimes WHERE owner_handle=?", String.class, handle)).isEqualTo("TERMINAL");
    assertThat(jdbc.queryForObject("SELECT status FROM accounts WHERE id=?", String.class, id)).isEqualTo("deleted");
  }

  @Test
  void dueRawDeletionRetainsOnlyOpaqueFenceAndPrecomputedSelector() {
    UUID handle = UUID.randomUUID();
    UUID id = account(handle);
    jdbc.update("UPDATE accounts SET status='deleted',deleted_at=clock_timestamp()-INTERVAL '31 days' WHERE id=?", id);
    jdbc.update("DELETE FROM accounts WHERE id=?", id);
    assertThat(jdbc.queryForObject("SELECT state FROM role_identity_lifetimes WHERE owner_handle=?", String.class, handle)).isEqualTo("TERMINAL");
    assertThat(jdbc.queryForObject("SELECT owner_handle FROM role_identity_account_selectors WHERE key_version=1 AND selector=?",
        UUID.class, SELECTORS.activeAccount(id))).isEqualTo(handle);
    assertThat(jdbc.queryForObject("SELECT count(*) FROM information_schema.columns WHERE table_name LIKE 'role_identity_%' "
        + "AND column_name IN ('account_id','profile_id','request_hash','request_body')", Long.class)).isZero();
    assertThatThrownBy(() -> account(jdbc, id, handle, true)).isInstanceOf(DataAccessException.class);
  }

  @Test
  void legacyMissingSelectorNeverDelaysDueRawDeletionOrPretendsComplete() {
    keyset();
    jdbc.update("UPDATE role_identity_owner_state SET enforcement_enabled=false");
    UUID handle = UUID.randomUUID();
    UUID id = account(jdbc, UUID.randomUUID(), handle, false);
    jdbc.update("UPDATE accounts SET status='deleted',deleted_at=clock_timestamp()-INTERVAL '31 days' WHERE id=?", id);
    jdbc.update("UPDATE role_identity_owner_state SET enforcement_enabled=true");
    jdbc.update("DELETE FROM accounts WHERE id=?", id);
    assertThat(jdbc.queryForObject("SELECT count(*) FROM accounts WHERE id=?", Long.class, id)).isZero();
    assertThat(jdbc.queryForObject("SELECT selector_complete FROM role_identity_lifetimes WHERE owner_handle=?", Boolean.class, handle)).isFalse();
    assertThat(jdbc.queryForObject("SELECT state FROM role_identity_lifetimes WHERE owner_handle=?", String.class, handle)).isEqualTo("TERMINAL");
    assertThat(jdbc.queryForObject("SELECT count(*) FROM role_identity_account_selectors WHERE owner_handle=?", Long.class, handle)).isZero();
  }
}
