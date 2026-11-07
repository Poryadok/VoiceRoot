package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.sql.Connection;
import java.sql.SQLException;
import java.time.Duration;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.testcontainers.containers.PostgreSQLContainer;

/** Real PostgreSQL serialization and permanent, identity-free fence storage. */
class AuthTerminalOperationFenceMigrationJdbcIntegrationTest {
  static final PostgreSQLContainer<?> POSTGRES = new PostgreSQLContainer<>("postgres:16-alpine")
      .withReuse(false).withLabel("voice.task", "be019-terminal-fence-schema");
  static DriverManagerDataSource source;
  static JdbcTemplate jdbc;

  @BeforeAll static void migrate() {
    POSTGRES.start();
    source = new DriverManagerDataSource(POSTGRES.getJdbcUrl(), POSTGRES.getUsername(), POSTGRES.getPassword());
    jdbc = new JdbcTemplate(source);
    Flyway.configure().dataSource(source).locations("classpath:db/migration").load().migrate();
  }
  @AfterAll static void close() { POSTGRES.stop(); }
  @BeforeEach void installed() {
    assertThat(jdbc.queryForObject("SELECT to_regclass('public.auth_terminal_operation_fences') IS NOT NULL", Boolean.class))
        .as("terminal operation fences must be installed by the default loader").isTrue();
  }

  @Test void permanentFenceContainsOnlyOpaqueColumnsAndNeverMutates() throws Exception {
    assertThat(jdbc.queryForList("SELECT column_name FROM information_schema.columns WHERE table_schema='public' AND table_name='auth_terminal_operation_fences' ORDER BY ordinal_position", String.class))
        .containsExactly("family", "operation_id", "owner_handle", "deletion_cycle_id", "completed_at");
    assertThat(jdbc.queryForObject("SELECT count(*) FROM pg_constraint WHERE conrelid='public.auth_terminal_operation_fences'::regclass AND contype='f'", Integer.class)).isZero();
    UUID operation = UUID.randomUUID();
    try (var connection = source.getConnection()) {
      fence(connection, "ownership_transfer", operation);
      fence(connection, "sdk_conversion", operation); // Families are independent namespaces.
      assertThatThrownBy(() -> fence(connection, "unknown", operation)).isInstanceOf(SQLException.class);
    }
    for (String sql : List.of("UPDATE auth_terminal_operation_fences SET completed_at=clock_timestamp()",
        "DELETE FROM auth_terminal_operation_fences", "TRUNCATE auth_terminal_operation_fences")) {
      assertThatThrownBy(() -> jdbc.execute(sql)).isInstanceOf(org.springframework.dao.DataAccessException.class);
    }
    assertThat(jdbc.queryForObject("SELECT count(*) FROM auth_terminal_operation_fences WHERE operation_id=?", Integer.class, operation)).isEqualTo(2);
  }

  @Test void everyRawOperationKeyRejectsRecreationBeforeAnyIdentityPayloadIsAccepted() throws Exception {
    var mappings = List.of(
        "ownership_transfer ownership_transfer_proofs operation_id",
        "sdk_authorization sdk_authorizations request_id",
        "sdk_binding sdk_authorizations game_binding_operation_id",
        "sdk_authorization sdk_linked_sessions request_id",
        "sdk_authorization sdk_game_binding_handoff_issuances authorization_request_id",
        "sdk_binding sdk_game_binding_handoff_issuances operation_id",
        "sdk_authorization sdk_game_binding_handoff_claims authorization_request_id",
        "sdk_binding sdk_game_binding_handoff_claims operation_id",
        "sdk_authorization sdk_game_message_grants authorization_request_id",
        "sdk_conversion sdk_conversion_operations operation_id",
        "sdk_conversion sdk_conversion_owner_receipts operation_id",
        "sdk_conversion sdk_registration_intents operation_id",
        "sdk_registration sdk_registration_intents intent_id",
        "sdk_execution sdk_game_message_execution_permits operation_id");
    try (var connection = source.getConnection()) {
      for (String mapping : mappings) {
        String[] parts = mapping.split(" ");
        UUID operation = UUID.randomUUID();
        fence(connection, parts[0], operation);
        try (var insert = connection.prepareStatement("INSERT INTO " + parts[1] + "(" + parts[2] + ") VALUES(?)")) {
          insert.setObject(1, operation);
          assertThatThrownBy(insert::executeUpdate).isInstanceOf(SQLException.class)
              .hasMessageContaining("terminal_operation_erased");
        }
      }
    }
  }

  @Test void fencedRowsCannotBeUpdatedOrMovedToAnUnfencedOperation() throws Exception {
    UUID operation = UUID.randomUUID();
    try (var connection = source.getConnection()) {
      proof(connection, operation);
      fence(connection, "ownership_transfer", operation);
    }
    for (String assignment : List.of("consumed_at=clock_timestamp()", "operation_id='" + UUID.randomUUID() + "'")) {
      assertThatThrownBy(() -> jdbc.update("UPDATE ownership_transfer_proofs SET " + assignment + " WHERE operation_id=?", operation))
          .isInstanceOf(org.springframework.dao.DataAccessException.class).hasMessageContaining("terminal_operation_erased");
    }
    jdbc.update("DELETE FROM ownership_transfer_proofs WHERE operation_id=?", operation);
  }

  @Test void fenceCommitRejectsWaitingRawInsertAndFenceRollbackAllowsIt() throws Exception {
    for (boolean commit : List.of(true, false)) {
      UUID operation = UUID.randomUUID();
      try (var owner = source.getConnection(); var writer = source.getConnection(); var executor = Executors.newSingleThreadExecutor()) {
        owner.setAutoCommit(false);
        fence(owner, "ownership_transfer", operation);
        int pid = pid(writer);
        var pending = executor.submit(() -> {
          try { proof(writer, operation); return "inserted"; }
          catch (SQLException denied) { return denied.getSQLState() + ":" + denied.getMessage(); }
        });
        awaitAdvisoryWait(pid);
        if (commit) owner.commit(); else owner.rollback();
        String result = pending.get(5, TimeUnit.SECONDS);
        if (commit) assertThat(result).contains("55000", "terminal_operation_erased");
        else assertThat(result).isEqualTo("inserted");
        assertThat(jdbc.queryForObject("SELECT count(*) FROM ownership_transfer_proofs WHERE operation_id=?", Integer.class, operation)).isEqualTo(commit ? 0 : 1);
      }
    }
  }

  @Test void rawInsertWinsBeforeFenceAndCleanupSeesCommitOrRollback() throws Exception {
    for (boolean commit : List.of(true, false)) {
      UUID operation = UUID.randomUUID();
      try (var writer = source.getConnection(); var cleaner = source.getConnection(); var executor = Executors.newSingleThreadExecutor()) {
        writer.setAutoCommit(false); cleaner.setAutoCommit(false);
        proof(writer, operation);
        int pid = pid(cleaner);
        var pending = executor.submit(() -> {
          fence(cleaner, "ownership_transfer", operation);
          try (var delete = cleaner.prepareStatement("DELETE FROM ownership_transfer_proofs WHERE operation_id=?")) {
            delete.setObject(1, operation);
            int erased = delete.executeUpdate();
            cleaner.commit();
            return erased;
          }
        });
        awaitAdvisoryWait(pid);
        if (commit) writer.commit(); else writer.rollback();
        assertThat(pending.get(5, TimeUnit.SECONDS)).isEqualTo(commit ? 1 : 0);
        assertThat(jdbc.queryForObject("SELECT count(*) FROM ownership_transfer_proofs WHERE operation_id=?", Integer.class, operation)).isZero();
        assertThat(jdbc.queryForObject("SELECT count(*) FROM auth_terminal_operation_fences WHERE family='ownership_transfer' AND operation_id=?", Integer.class, operation)).isEqualTo(1);
      }
    }
  }

  @Test void repeatableReadWritersCannotUseAStaleFenceSnapshot() throws Exception {
    try (var connection = source.getConnection()) {
      connection.setTransactionIsolation(Connection.TRANSACTION_REPEATABLE_READ);
      connection.setAutoCommit(false);
      assertThatThrownBy(() -> proof(connection, UUID.randomUUID())).isInstanceOf(SQLException.class)
          .hasMessageContaining("terminal_operation_requires_read_committed");
      connection.rollback();
      assertThatThrownBy(() -> fence(connection, "ownership_transfer", UUID.randomUUID())).isInstanceOf(SQLException.class)
          .hasMessageContaining("terminal_operation_requires_read_committed");
      connection.rollback();
    }
  }

  static void fence(Connection connection, String family, UUID operation) throws SQLException {
    try (var insert = connection.prepareStatement("INSERT INTO auth_terminal_operation_fences(family,operation_id,owner_handle,deletion_cycle_id,completed_at) VALUES(?,?,?,?,clock_timestamp())")) {
      insert.setString(1, family); insert.setObject(2, operation);
      insert.setObject(3, UUID.randomUUID()); insert.setObject(4, UUID.randomUUID());
      insert.executeUpdate();
    }
  }
  static void proof(Connection connection, UUID operation) throws SQLException {
    try (var insert = connection.prepareStatement("INSERT INTO ownership_transfer_proofs(operation_id,account_id,profile_id,space_id,new_owner_profile_id,session_epoch,proof_hash,security_revision,verified_factors,expires_at,receipt_id) VALUES(?,?,?,?,?,1,?,1,'password',clock_timestamp()+interval '5 minutes',?)")) {
      for (int index = 1; index <= 5; index++) insert.setObject(index, index == 1 ? operation : UUID.randomUUID());
      insert.setString(6, UUID.randomUUID().toString().replace("-", "") + UUID.randomUUID().toString().replace("-", ""));
      insert.setObject(7, UUID.randomUUID()); insert.executeUpdate();
    }
  }
  static int pid(Connection connection) throws SQLException {
    try (var statement = connection.createStatement()) { statement.execute("SET statement_timeout='10s'"); }
    try (var statement = connection.createStatement(); var result = statement.executeQuery("SELECT pg_backend_pid()")) {
      result.next(); return result.getInt(1);
    }
  }
  static void awaitAdvisoryWait(int pid) throws Exception {
    long deadline = System.nanoTime() + Duration.ofSeconds(5).toNanos();
    while (System.nanoTime() < deadline) {
      if (Boolean.TRUE.equals(jdbc.queryForObject("SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=? AND locktype='advisory' AND NOT granted)", Boolean.class, pid))) return;
      Thread.sleep(10);
    }
    throw new AssertionError("operation writer did not wait on the shared advisory fence");
  }
}
