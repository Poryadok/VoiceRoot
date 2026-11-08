package voice.backend.auth.sdkidentity;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.sql.Connection;
import java.time.Clock;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.transaction.support.TransactionTemplate;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;

/** Exercises the opaque JDBC read contract and the source-lock race with real PostgreSQL. */
@Testcontainers(disabledWithoutDocker = true)
class SdkTerminalFenceJdbcIntegrationTest {
  @Container static final PostgreSQLContainer<?> postgres = new PostgreSQLContainer<>("postgres:16-alpine");
  private DriverManagerDataSource database;
  private NamedParameterJdbcTemplate jdbc;
  private AuthGameBindingHandoffService service;

  @BeforeEach
  void initialize() {
    database = new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
    jdbc = new NamedParameterJdbcTemplate(database);
    // Minimal schemas intentionally expose only columns read by this consumer contract.
    jdbc.getJdbcTemplate().execute("CREATE TABLE IF NOT EXISTS auth_terminal_operation_fences "
        + "(family text NOT NULL, operation_id uuid NOT NULL, PRIMARY KEY(family,operation_id))");
    jdbc.getJdbcTemplate().execute("CREATE TABLE IF NOT EXISTS sdk_identities (account_id uuid PRIMARY KEY)");
    jdbc.getJdbcTemplate().execute("CREATE TABLE IF NOT EXISTS sdk_authorizations "
        + "(source_account_id uuid,game_binding_operation_id uuid)");
    service = new AuthGameBindingHandoffService(jdbc,
        new TransactionTemplate(new DataSourceTransactionManager(database)), null, null, null, null, null, Clock.systemUTC());
  }

  @Test
  void erasedOperationNeverNeedsRawReceiptAndOtherFamiliesDoNotCollide() {
    UUID operation = UUID.randomUUID();
    jdbc.update("INSERT INTO auth_terminal_operation_fences VALUES ('sdk_execution',:id)", Map.of("id", operation));
    assertThatThrownBy(() -> service.revoke(operation)).isInstanceOf(SdkIdentityDeniedException.class);
    jdbc.update("INSERT INTO auth_terminal_operation_fences VALUES ('sdk_binding',:id)", Map.of("id", operation));
    assertThatThrownBy(() -> service.revoke(operation)).isInstanceOf(SdkOperationErasedException.class);
    assertThatThrownBy(() -> service.complete(UUID.randomUUID(), operation, "failed", null))
        .isInstanceOf(SdkOperationErasedException.class);
  }

  @Test
  void retryWaitingOnSourceObservesFenceAfterCleanupCommitsAndPreservesLiveSource() throws Exception {
    UUID source = UUID.randomUUID();
    UUID operation = UUID.randomUUID();
    jdbc.update("INSERT INTO sdk_identities VALUES (:source)", Map.of("source", source));
    jdbc.update("INSERT INTO sdk_authorizations VALUES (:source,:operation)", Map.of("source", source, "operation", operation));
    try (Connection cleaner = database.getConnection(); var worker = Executors.newSingleThreadExecutor()) {
      cleaner.setAutoCommit(false);
      try (var lock = cleaner.prepareStatement("SELECT account_id FROM sdk_identities WHERE account_id=? FOR UPDATE")) {
        lock.setObject(1, source);
        lock.executeQuery().close();
      }
      var retry = worker.submit(() -> {
        try { service.revoke(operation); return "unexpected_success"; }
        catch (RuntimeException result) { return result.getMessage(); }
      });
      try {
        long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(10);
        boolean waiting = false;
        while (System.nanoTime() < deadline) {
          waiting = Boolean.TRUE.equals(jdbc.getJdbcTemplate().queryForObject(
              "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' "
                  + "AND query LIKE 'SELECT account_id FROM sdk_identities%')", Boolean.class));
          if (waiting) break;
          Thread.sleep(10);
        }
        assertThat(waiting).as("retry reached the shared source lock").isTrue();
        try (var fence = cleaner.prepareStatement("INSERT INTO auth_terminal_operation_fences VALUES ('sdk_binding',?)");
             var remove = cleaner.prepareStatement("DELETE FROM sdk_authorizations WHERE game_binding_operation_id=?")) {
          fence.setObject(1, operation); fence.executeUpdate();
          remove.setObject(1, operation); remove.executeUpdate();
        }
        cleaner.commit();
        assertThat(retry.get(10, TimeUnit.SECONDS)).isEqualTo("operation_erased");
        assertThat(jdbc.queryForObject("SELECT count(*) FROM sdk_identities WHERE account_id=:id",
            Map.of("id", source), Long.class)).isEqualTo(1L);
      } finally {
        cleaner.rollback();
      }
    }
  }
}
