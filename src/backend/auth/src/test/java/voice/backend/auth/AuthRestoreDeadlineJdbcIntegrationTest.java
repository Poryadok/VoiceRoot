package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import java.sql.Connection;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.condition.EnabledIfEnvironmentVariable;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;
import voice.backend.auth.repository.JdbcAccountRepository;

/** Real native-catalog proof; execute only in the owned Linux PostgreSQL runner. */
@EnabledIfEnvironmentVariable(named = "AUTH_RESTORE_DEADLINE_TEST_JDBC_URL", matches = ".+")
class AuthRestoreDeadlineJdbcIntegrationTest {
  private static DriverManagerDataSource source;

  @BeforeAll
  static void migrate() {
    assertThat(System.getProperty("os.name")).isEqualTo("Linux");
    source = new DriverManagerDataSource(
        System.getenv("AUTH_RESTORE_DEADLINE_TEST_JDBC_URL"), "postgres", "local-proof");
    Flyway.configure().dataSource(source).locations("filesystem:src/main/resources/db/migration")
        .load().migrate();
  }

  private static JdbcAccountRepository repository() {
    return new JdbcAccountRepository(new NamedParameterJdbcTemplate(source));
  }

  private static JdbcAccountRepository repository(Connection connection) {
    return new JdbcAccountRepository(new NamedParameterJdbcTemplate(
        new SingleConnectionDataSource(connection, true)));
  }

  private static UUID account() {
    return repository().create(UUID.randomUUID() + "@example.invalid", null, "fixture", "regular").id();
  }

  @Test
  void restoreStrictlyBeforeDeadlineStillSucceeds() {
    UUID account = account();
    new JdbcTemplate(source).update("UPDATE accounts SET status='deleted', "
        + "deleted_at=clock_timestamp()-INTERVAL '29 days' WHERE id=?", account);
    assertThat(repository().restoreDeleted(account)).isTrue();
    var restored = repository().findById(account.toString()).orElseThrow();
    assertThat(restored.status()).isEqualTo("active");
    assertThat(restored.deletedAt()).isNull();
  }

  @Test
  void transactionStartAtDeadlineCannotAuthorizeLaterRestore() throws Exception {
    UUID account = account();
    try (Connection connection = source.getConnection()) {
      connection.setAutoCommit(false);
      var transaction = new JdbcTemplate(new SingleConnectionDataSource(connection, true));
      Instant started = transaction.queryForObject("SELECT CURRENT_TIMESTAMP", Timestamp.class).toInstant();
      new JdbcTemplate(source).update("UPDATE accounts SET status='deleted', deleted_at=? WHERE id=?",
          Timestamp.from(started.minusSeconds(30L * 24 * 60 * 60)), account);
      assertThat(repository(connection).restoreDeleted(account)).isFalse();
      connection.commit();
    }
    assertThat(repository().findById(account.toString()).orElseThrow().status()).isEqualTo("deleted");
  }

  @Test
  void restoreWaitingOnExactAccountLockCannotCrossDeadline() throws Exception {
    UUID account = account();
    var jdbc = new JdbcTemplate(source);
    jdbc.update("UPDATE accounts SET status='deleted', "
        + "deleted_at=clock_timestamp()-INTERVAL '30 days'+INTERVAL '15 seconds' WHERE id=?", account);
    Instant deleted = repository().findById(account.toString()).orElseThrow().deletedAt();
    Instant deadline = deleted.plusSeconds(30L * 24 * 60 * 60);
    var worker = Executors.newSingleThreadExecutor();
    try (Connection blocker = source.getConnection()) {
      blocker.setAutoCommit(false);
      var locked = new JdbcTemplate(new SingleConnectionDataSource(blocker, true));
      locked.queryForObject("SELECT id FROM accounts WHERE id=? FOR UPDATE", UUID.class, account);
      int blockerPid = locked.queryForObject("SELECT pg_backend_pid()", Integer.class);
      var ready = new CountDownLatch(1);
      var restorePid = new AtomicInteger();
      var restored = worker.submit(() -> {
        try (Connection connection = source.getConnection()) {
          connection.setAutoCommit(false);
          var transaction = new JdbcTemplate(new SingleConnectionDataSource(connection, true));
          restorePid.set(transaction.queryForObject("SELECT pg_backend_pid()", Integer.class));
          Instant started = transaction.queryForObject("SELECT CURRENT_TIMESTAMP", Timestamp.class).toInstant();
          assertThat(started).isBefore(deadline);
          ready.countDown();
          boolean result = repository(connection).restoreDeleted(account);
          connection.commit();
          return result;
        }
      });
      assertThat(ready.await(5, TimeUnit.SECONDS)).isTrue();
      long waitLimit = System.nanoTime() + TimeUnit.SECONDS.toNanos(5);
      boolean actuallyBlocked = false;
      while (System.nanoTime() < waitLimit) {
        actuallyBlocked = Boolean.TRUE.equals(locked.queryForObject(
            "SELECT ?=ANY(pg_blocking_pids(?))", Boolean.class, blockerPid, restorePid.get()));
        if (actuallyBlocked) break;
        Thread.sleep(10);
      }
      assertThat(actuallyBlocked).as("restore must wait on this account row, not an idle connection").isTrue();
      locked.execute("SELECT pg_sleep_until('" + deadline + "'::timestamptz)");
      assertThat(locked.queryForObject("SELECT clock_timestamp()>=?", Boolean.class,
          Timestamp.from(deadline))).isTrue();
      blocker.commit();
      assertThat(restored.get(10, TimeUnit.SECONDS)).isFalse();
    } finally {
      worker.shutdownNow();
      assertThat(worker.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
    var unchanged = repository().findById(account.toString()).orElseThrow();
    assertThat(unchanged.status()).isEqualTo("deleted");
    assertThat(unchanged.deletedAt()).isEqualTo(deleted);
  }
}
