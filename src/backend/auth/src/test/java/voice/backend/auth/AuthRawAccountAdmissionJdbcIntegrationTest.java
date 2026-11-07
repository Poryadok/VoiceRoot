package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.awaitility.Awaitility.await;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.verifyNoInteractions;

import java.sql.Connection;
import java.sql.SQLException;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.testcontainers.containers.PostgreSQLContainer;
import voice.backend.auth.mail.MailSender;
import voice.backend.auth.repository.JdbcAccountRepository;
import voice.backend.auth.repository.JdbcOtpCodeRepository;
import voice.backend.auth.repository.OtpCodeRecord;
import voice.backend.auth.repository.RefreshTokenRepository;
import voice.backend.auth.security.BCryptPasswordHasher;
import voice.backend.auth.security.RefreshTokenCodec;
import voice.backend.auth.service.GuestConversionOtpAcceptance;
import voice.backend.auth.service.OtpService;
import voice.backend.auth.service.OtpThrottle;
import voice.backend.auth.service.SendOtpCommand;

/** Actual account deletion and delayed application writes; no fake erasure receipt. */
class AuthRawAccountAdmissionJdbcIntegrationTest {
  static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
      .withReuse(false).withLabel("voice.task", "be019-raw-admission");
  static final List<String> TABLES = List.of("refresh_tokens", "otp_codes", "backup_codes",
      "linked_identities", "e2e_key_backups", "guest_conversion_operations",
      "verification_source_sync_targets", "linked_oauth_attempts", "account_deletion_operations");
  static DriverManagerDataSource source;
  static JdbcTemplate jdbc;

  @BeforeAll static void migrate() {
    POSTGRES.start();
    source = new DriverManagerDataSource(POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword());
    jdbc = new JdbcTemplate(source);
    Flyway.configure().dataSource(source).locations("classpath:db/migration").load().migrate();
  }
  @AfterAll static void close() { POSTGRES.stop(); }

  @Test void otpPausedAfterActiveSnapshotCannotRecreatePersonalDataAfterRealDueDeletion() throws Exception {
    UUID account = account();
    String email = jdbc.queryForObject("SELECT email FROM accounts WHERE id=?", String.class, account);
    var entered = new CountDownLatch(1);
    var resume = new CountDownLatch(1);
    var named = new NamedParameterJdbcTemplate(source);
    var repository = new JdbcOtpCodeRepository(named) {
      @Override public OtpCodeRecord create(UUID id, String hash, String type, Instant expires, Instant now) {
        entered.countDown();
        try { assertThat(resume.await(10, TimeUnit.SECONDS)).isTrue(); }
        catch (InterruptedException ex) { Thread.currentThread().interrupt(); throw new AssertionError(ex); }
        return super.create(id, hash, type, expires, now);
      }
    };
    MailSender mail = mock(MailSender.class);
    var service = new OtpService(new JdbcAccountRepository(named), repository,
        mock(RefreshTokenRepository.class), new RefreshTokenCodec(), new BCryptPasswordHasher(),
        mail, mock(OtpThrottle.class), Clock.systemUTC(), mock(GuestConversionOtpAcceptance.class));
    try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
      var pending = executor.submit(() -> service.sendOtp(new SendOtpCommand(email, null, "password_reset", null), null));
      try {
        assertThat(entered.await(10, TimeUnit.SECONDS)).isTrue();
        due(account);
        jdbc.update("DELETE FROM accounts WHERE id=?", account);
        assertThat(jdbc.queryForObject("SELECT count(*) FROM accounts WHERE id=?", Integer.class, account)).isZero();
      } finally { resume.countDown(); }
      assertThatThrownBy(() -> pending.get(10, TimeUnit.SECONDS)).hasCauseInstanceOf(org.springframework.dao.DataAccessException.class)
          .rootCause().hasMessageContaining("raw_account_unavailable");
    }
    assertThat(countOtp(account)).isZero();
    verifyNoInteractions(mail);
  }

  @Test void everyRawStoreRejectsMissingAndDueOwnersBeforeAcceptingPayload() throws Exception {
    UUID missing = UUID.randomUUID(), expired = account();
    due(expired);
    try (var connection = source.getConnection()) {
      for (String table : TABLES) for (UUID owner : List.of(missing, expired)) {
        try (var insert = connection.prepareStatement("INSERT INTO " + table + "(account_id) VALUES(?)")) {
          insert.setObject(1, owner);
          assertThatThrownBy(insert::executeUpdate).isInstanceOf(SQLException.class)
              .hasMessageContaining("raw_account_unavailable");
        }
      }
    }
  }

  @Test void recoverableOwnersKeepAllNineExistingWorkflowsAndCannotMoveRawOwnership() throws Exception {
    UUID account = account(), other = account();
    jdbc.update("UPDATE accounts SET deleted_at=clock_timestamp()-interval '1 day',status='deleted' WHERE id=?", account);
    for (String sql : validInserts()) {
      jdbc.update(sql, account);
    }
    try (var connection = source.getConnection()) {
      for (String table : TABLES) {
        assertThat(jdbc.queryForObject("SELECT count(*) FROM " + table + " WHERE account_id=?", Integer.class, account)).isEqualTo(1);
        try (var update = connection.prepareStatement("UPDATE " + table + " SET account_id=? WHERE account_id=?")) {
          update.setObject(1, other); update.setObject(2, account);
          assertThatThrownBy(update::executeUpdate).isInstanceOf(SQLException.class)
              .hasMessageContaining("raw_account_owner_immutable");
        }
      }
    }
    assertThat(jdbc.queryForObject("SELECT count(*) FROM account_subscription_lifecycle_outbox WHERE account_id=?", Integer.class, account)).isEqualTo(1);
  }

  @Test void waitingInsertSeesCommittedDeletionAndAllowsRolledBackDeletion() throws Exception {
    for (boolean commit : List.of(true, false)) {
      UUID account = account();
      try (var deleting = source.getConnection(); var writer = source.getConnection();
           var executor = Executors.newVirtualThreadPerTaskExecutor()) {
        deleting.setAutoCommit(false);
        execute(deleting, "UPDATE accounts SET status='deleted',deleted_at=clock_timestamp()-interval '31 days' WHERE id=?", account);
        execute(deleting, "DELETE FROM accounts WHERE id=?", account);
        int pid = pid(writer);
        var pending = executor.submit(() -> insertOtp(writer, account));
        try { blocked(pid); } finally { if (commit) deleting.commit(); else deleting.rollback(); }
        if (commit) assertThatThrownBy(() -> pending.get(10, TimeUnit.SECONDS)).hasCauseInstanceOf(SQLException.class)
            .rootCause().hasMessageContaining("raw_account_unavailable");
        else assertThat(pending.get(10, TimeUnit.SECONDS)).isEqualTo(1);
      }
      assertThat(countOtp(account)).isEqualTo(commit ? 0 : 1);
    }
  }

  @Test void postLockClockRejectsInsertWhoseTransactionStartedBeforeDeadline() throws Exception {
    UUID account = account();
    jdbc.update("UPDATE accounts SET status='deleted',deleted_at=clock_timestamp()-interval '30 days'+interval '2 seconds' WHERE id=?", account);
    try (var blocker = source.getConnection(); var writer = source.getConnection();
         var executor = Executors.newVirtualThreadPerTaskExecutor()) {
      blocker.setAutoCommit(false);
      execute(blocker, "SELECT id FROM accounts WHERE id=? FOR UPDATE", account);
      writer.setAutoCommit(false);
      int pid = pid(writer);
      var pending = executor.submit(() -> insertOtp(writer, account));
      try {
        blocked(pid);
        await().atMost(Duration.ofSeconds(10)).until(() -> jdbc.queryForObject(
            "SELECT clock_timestamp()>=deleted_at+interval '30 days' FROM accounts WHERE id=?", Boolean.class, account));
      } finally { blocker.rollback(); }
      assertThatThrownBy(() -> pending.get(10, TimeUnit.SECONDS)).hasCauseInstanceOf(SQLException.class)
          .rootCause().hasMessageContaining("raw_account_unavailable");
      writer.rollback();
    }
    assertThat(countOtp(account)).isZero();
  }

  @Test void admittedWriterHoldsAccountUntilCommitAndCleanupThenSeesItsRow() throws Exception {
    UUID account = account();
    try (var writer = source.getConnection(); var eraser = source.getConnection();
         var executor = Executors.newVirtualThreadPerTaskExecutor()) {
      writer.setAutoCommit(false);
      insertOtp(writer, account);
      eraser.setAutoCommit(false);
      int pid = pid(eraser);
      var pending = executor.submit(() -> {
        execute(eraser, "UPDATE accounts SET status='deleted',deleted_at=clock_timestamp()-interval '31 days' WHERE id=?", account);
        int rows = execute(eraser, "DELETE FROM otp_codes WHERE account_id=?", account);
        execute(eraser, "DELETE FROM accounts WHERE id=?", account);
        eraser.commit();
        return rows;
      });
      try { blocked(pid); } finally { writer.commit(); }
      assertThat(pending.get(10, TimeUnit.SECONDS)).isEqualTo(1);
    }
    assertThat(countOtp(account)).isZero();
  }

  @Test void sameOwnerUpdatesAndDeletesDoNotAcquireAReverseAccountLock() throws Exception {
    UUID account = account();
    try (var writer = source.getConnection(); var blocker = source.getConnection()) {
      insertOtp(writer, account);
      blocker.setAutoCommit(false);
      execute(blocker, "SELECT id FROM accounts WHERE id=? FOR UPDATE", account);
      try {
        writer.createStatement().execute("SET statement_timeout='1s'");
        assertThat(execute(writer, "UPDATE otp_codes SET account_id=account_id,used_at=clock_timestamp() WHERE account_id=?", account)).isEqualTo(1);
        assertThat(execute(writer, "DELETE FROM otp_codes WHERE account_id=?", account)).isEqualTo(1);
      } finally { blocker.rollback(); }
    }
  }

  @Test void staleSnapshotIsolationCannotAdmitRawData() throws Exception {
    UUID account = account();
    try (var connection = source.getConnection()) {
      connection.setTransactionIsolation(Connection.TRANSACTION_REPEATABLE_READ);
      connection.setAutoCommit(false);
      assertThatThrownBy(() -> insertOtp(connection, account)).isInstanceOf(SQLException.class)
          .hasMessageContaining("raw_account_requires_read_committed");
      connection.rollback();
    }
  }

  static UUID account() {
    UUID id = UUID.randomUUID();
    jdbc.update("INSERT INTO accounts(id,email,password_hash,type,status) VALUES(?,?,'fixture','regular','active')", id, id+"@example.test");
    return id;
  }
  static void due(UUID id) {
    jdbc.update("UPDATE accounts SET status='deleted',deleted_at=clock_timestamp()-interval '31 days' WHERE id=?", id);
  }
  static int countOtp(UUID id) { return jdbc.queryForObject("SELECT count(*) FROM otp_codes WHERE account_id=?", Integer.class, id); }
  static int insertOtp(Connection connection, UUID id) throws SQLException {
    return execute(connection, "INSERT INTO otp_codes(account_id,code,type,expires_at) VALUES(?,'\\x01','password_reset',clock_timestamp()+interval '10 minutes')", id);
  }
  static int execute(Connection connection, String sql, UUID id) throws SQLException {
    try (var statement = connection.prepareStatement(sql)) {
      statement.setObject(1,id);
      return statement.execute() ? 0 : statement.getUpdateCount();
    }
  }
  static int pid(Connection connection) throws SQLException {
    try (var statement = connection.createStatement(); var result = statement.executeQuery("SELECT pg_backend_pid()")) { result.next(); return result.getInt(1); }
  }
  static void blocked(int pid) {
    await().atMost(Duration.ofSeconds(5)).until(() -> jdbc.queryForObject(
        "SELECT cardinality(pg_blocking_pids(?))>0", Boolean.class, pid));
  }
  static List<String> validInserts() {
    return List.of(
        "INSERT INTO refresh_tokens(account_id,token_hash,access_jti,expires_at) VALUES(?,'fixture','fixture',clock_timestamp()+interval '1 day')",
        "INSERT INTO otp_codes(account_id,code,type,expires_at) VALUES(?,'\\x01','password_reset',clock_timestamp()+interval '10 minutes')",
        "INSERT INTO backup_codes(account_id,code_hash) VALUES(?,'fixture')",
        "INSERT INTO linked_identities(account_id,platform,external_id) VALUES(?,'twitch',gen_random_uuid()::text)",
        "INSERT INTO e2e_key_backups(account_id,encrypted_blob) VALUES(?,'fixture')",
        "INSERT INTO guest_conversion_operations(account_id,operation_id,otp_code_id,state) VALUES(?,gen_random_uuid(),gen_random_uuid(),'PENDING_USER')",
        "INSERT INTO verification_source_sync_targets(account_id,profile_id,platform,source_revision,verified,badge) VALUES(?,gen_random_uuid(),'twitch',1,false,'twitch')",
        "INSERT INTO linked_oauth_attempts(account_id,id,profile_id,provider,original_jti,original_epoch,original_exp,subscription_tier,registration_revision,callback_uri,return_uri,client_surface,state_hash,return_challenge,expires_at,status) VALUES(?,gen_random_uuid(),gen_random_uuid(),'twitch','fixture',1,clock_timestamp()+interval '1 day','free','fixture','fixture','fixture','web',gen_random_uuid()::text,'fixture',clock_timestamp()+interval '1 day','pending')",
        "INSERT INTO account_deletion_operations(account_id,operation_id,session_epoch,restore_token_hash,state,next_attempt_at,created_at,updated_at) VALUES(?,gen_random_uuid(),1,'fixture','PENDING_FLOOR',clock_timestamp(),clock_timestamp(),clock_timestamp())");
  }
}
