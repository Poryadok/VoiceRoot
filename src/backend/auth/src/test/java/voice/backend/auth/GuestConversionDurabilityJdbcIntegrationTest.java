package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.nio.file.Files;
import java.nio.file.Path;
import java.sql.Connection;
import java.sql.PreparedStatement;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;
import java.time.Instant;
import java.util.Comparator;
import java.util.List;
import java.util.UUID;
import java.util.stream.Stream;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.Test;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;

/**
 * T-049b RED PostgreSQL contract for the two supported Auth migration paths.
 *
 * <p>Each path receives a separate schema. This prevents Flyway from masking a missing
 * golang-migrate revision (or vice versa) and verifies executable DDL rather than migration text.
 */
@Testcontainers(disabledWithoutDocker = true)
class GuestConversionDurabilityJdbcIntegrationTest {
  private static final String TABLE = "guest_conversion_operations";
  private static final String FLYWAY_SCHEMA = "guest_conversion_flyway_contract";
  private static final String GOLANG_SCHEMA = "guest_conversion_golang_contract";
  private static final String GOLANG_PENDING_DOWN_SCHEMA = "guest_conversion_pending_down_contract";
  private static final String GOLANG_COMPLETED_DOWN_SCHEMA = "guest_conversion_completed_down_contract";

  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db")
          .withUsername("voice")
          .withPassword("voice");

  @Test
  void flywayPathEnforcesTheDurableOperationCatalog() throws Exception {
    migrateFlyway(FLYWAY_SCHEMA);

    assertDurableOperationCatalog(FLYWAY_SCHEMA);
  }

  @Test
  void golangMigratePathEnforcesTheSameDurableOperationCatalog() throws Exception {
    migrateGolang(GOLANG_SCHEMA);

    assertDurableOperationCatalog(GOLANG_SCHEMA);
  }

  @Test
  void golangMigrateDownRefusesToDiscardPendingGuestConversionWork() throws Exception {
    migrateGolang(GOLANG_PENDING_DOWN_SCHEMA);
    UUID pendingUserAccountId = UUID.randomUUID();
    UUID pendingEventAccountId = UUID.randomUUID();
    insertPendingUser(
        GOLANG_PENDING_DOWN_SCHEMA, UUID.randomUUID(), pendingUserAccountId, UUID.randomUUID());
    insert(
        GOLANG_PENDING_DOWN_SCHEMA,
        UUID.randomUUID(),
        pendingEventAccountId,
        UUID.randomUUID(),
        "PENDING_EVENT");

    String downMigration =
        Files.readString(
            GuestConversionDurabilityMigrationContractTest.golangMigration(
                GuestConversionDurabilityMigrationContractTest.GOLANG_DOWN_MIGRATION));
    assertThatThrownBy(() -> executeSql(GOLANG_PENDING_DOWN_SCHEMA, downMigration))
        .as("rollback must refuse while a conversion can still require User marking or event delivery")
        .isInstanceOf(SQLException.class);

    assertThat(tableExists(GOLANG_PENDING_DOWN_SCHEMA)).isTrue();
    assertThat(countOperationsForAccount(GOLANG_PENDING_DOWN_SCHEMA, pendingUserAccountId)).isEqualTo(1);
    assertThat(operationStateForAccount(GOLANG_PENDING_DOWN_SCHEMA, pendingUserAccountId))
        .isEqualTo("PENDING_USER");
    assertThat(countOperationsForAccount(GOLANG_PENDING_DOWN_SCHEMA, pendingEventAccountId)).isEqualTo(1);
    assertThat(operationStateForAccount(GOLANG_PENDING_DOWN_SCHEMA, pendingEventAccountId))
        .isEqualTo("PENDING_EVENT");
  }

  @Test
  void golangMigrateDownDropsOnlyCompletedGuestConversionWork() throws Exception {
    migrateGolang(GOLANG_COMPLETED_DOWN_SCHEMA);
    insert(
        GOLANG_COMPLETED_DOWN_SCHEMA,
        UUID.randomUUID(),
        UUID.randomUUID(),
        UUID.randomUUID(),
        "COMPLETED");

    String downMigration =
        Files.readString(
            GuestConversionDurabilityMigrationContractTest.golangMigration(
                GuestConversionDurabilityMigrationContractTest.GOLANG_DOWN_MIGRATION));
    executeSql(GOLANG_COMPLETED_DOWN_SCHEMA, downMigration);

    assertThat(tableExists(GOLANG_COMPLETED_DOWN_SCHEMA)).isFalse();
  }

  private void migrateFlyway(String schema) {
    Flyway.configure()
        .dataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())
        .schemas(schema)
        .defaultSchema(schema)
        .createSchemas(true)
        .locations(
            "filesystem:"
                + GuestConversionDurabilityMigrationContractTest.authProjectRoot()
                    .resolve("src/main/resources/db/migration"))
        .load()
        .migrate();
  }

  private void migrateGolang(String schema) throws Exception {
    createSchema(schema);
    try (Stream<Path> migrations = Files.list(
        GuestConversionDurabilityMigrationContractTest.golangMigration(".").getParent())) {
      List<Path> upMigrations =
          migrations
              .filter(path -> path.getFileName().toString().endsWith(".up.sql"))
              .sorted(Comparator.comparing(path -> path.getFileName().toString()))
              .toList();
      for (Path migration : upMigrations) {
        executeSql(schema, Files.readString(migration));
      }
    }
  }

  private void assertDurableOperationCatalog(String schema) throws Exception {
    assertThat(tableExists(schema)).isTrue();
    assertColumn(schema, "operation_id", "uuid", false, false);
    assertColumn(schema, "account_id", "uuid", false, false);
    assertColumn(schema, "otp_code_id", "uuid", false, false);
    assertColumn(schema, "state", null, false, false);
    assertColumn(schema, "attempt_count", "integer", false, true);
    assertColumn(schema, "next_attempt_at", "timestamp with time zone", false, true);
    assertColumn(schema, "locked_until", "timestamp with time zone", true, false);
    assertColumn(schema, "last_error_code", null, true, false);
    assertColumn(schema, "user_marked_at", "timestamp with time zone", true, false);
    assertColumn(schema, "auth_promoted_at", "timestamp with time zone", true, false);
    assertColumn(schema, "event_published_at", "timestamp with time zone", true, false);
    assertColumn(schema, "created_at", "timestamp with time zone", false, true);
    assertColumn(schema, "updated_at", "timestamp with time zone", false, true);

    UUID operationId = UUID.randomUUID();
    UUID accountId = UUID.randomUUID();
    UUID otpId = UUID.randomUUID();
    insertPendingUser(schema, operationId, accountId, otpId);
    insert(schema, UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), "PENDING_EVENT");
    insert(schema, UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), "COMPLETED");
    assertThatThrownBy(
            () -> insertPendingUser(schema, operationId, UUID.randomUUID(), UUID.randomUUID()))
        .as("operation_id is the immutable event identity and primary key")
        .isInstanceOf(SQLException.class);
    assertThatThrownBy(() -> insertPendingUser(schema, UUID.randomUUID(), accountId, UUID.randomUUID()))
        .as("one account must always resume its existing operation")
        .isInstanceOf(SQLException.class);
    assertThatThrownBy(() -> insertPendingUser(schema, UUID.randomUUID(), UUID.randomUUID(), otpId))
        .as("an OTP may not drive two conversion operations")
        .isInstanceOf(SQLException.class);
    assertThatThrownBy(
            () -> insert(schema, UUID.randomUUID(), UUID.randomUUID(), UUID.randomUUID(), "BROKEN"))
        .as("only the documented durable states are legal")
        .isInstanceOf(SQLException.class);
    assertThatThrownBy(
            () ->
                insert(
                    schema,
                    UUID.randomUUID(),
                    UUID.randomUUID(),
                    UUID.randomUUID(),
                    "PENDING_USER",
                    -1))
        .as("retry attempt cannot be negative")
        .isInstanceOf(SQLException.class);
    UUID defaultAccountId = UUID.randomUUID();
    Instant beforeInsert = databaseClock();
    insertWithRetryDefaults(schema, UUID.randomUUID(), defaultAccountId, UUID.randomUUID());
    Instant afterInsert = databaseClock();
    assertThat(defaultAttemptCountForAccount(schema, defaultAccountId)).isEqualTo(0);
    assertWithinDatabaseClock(defaultNextAttemptForAccount(schema, defaultAccountId), beforeInsert, afterInsert);
    assertWithinDatabaseClock(defaultCreatedAtForAccount(schema, defaultAccountId), beforeInsert, afterInsert);
    assertWithinDatabaseClock(defaultUpdatedAtForAccount(schema, defaultAccountId), beforeInsert, afterInsert);
    assertInitialNullableFieldsAreNull(schema, defaultAccountId);
  }

  private void assertColumn(
      String schema, String name, String dataType, boolean nullable, boolean mustHaveDefault)
      throws SQLException {
    try (Connection connection = connection();
        PreparedStatement statement =
            connection.prepareStatement(
                """
                SELECT data_type, is_nullable, column_default
                FROM information_schema.columns
                WHERE table_schema = ? AND table_name = ? AND column_name = ?
                """)) {
      statement.setString(1, schema);
      statement.setString(2, TABLE);
      statement.setString(3, name);
      try (ResultSet result = statement.executeQuery()) {
        assertThat(result.next()).as("column %s", name).isTrue();
        if (dataType != null) {
          assertThat(result.getString("data_type")).isEqualTo(dataType);
        }
        assertThat(result.getString("is_nullable")).isEqualTo(nullable ? "YES" : "NO");
        if (mustHaveDefault) {
          assertThat(result.getString("column_default")).as("default for %s", name).isNotBlank();
        }
      }
    }
  }

  private boolean tableExists(String schema) throws SQLException {
    try (Connection connection = connection();
        PreparedStatement statement =
            connection.prepareStatement("SELECT to_regclass(?) IS NOT NULL")) {
      statement.setString(1, schema + "." + TABLE);
      try (ResultSet result = statement.executeQuery()) {
        result.next();
        return result.getBoolean(1);
      }
    }
  }

  private void createSchema(String schema) throws SQLException {
    try (Connection connection = connection(); Statement statement = connection.createStatement()) {
      statement.execute("DROP SCHEMA IF EXISTS " + schema + " CASCADE");
      statement.execute("CREATE SCHEMA " + schema);
    }
  }

  private void insertPendingUser(String schema, UUID operationId, UUID accountId, UUID otpId)
      throws SQLException {
    insert(schema, operationId, accountId, otpId, "PENDING_USER");
  }

  private void insert(String schema, UUID operationId, UUID accountId, UUID otpId, String state)
      throws SQLException {
    insert(schema, operationId, accountId, otpId, state, 0);
  }

  private void insert(
      String schema, UUID operationId, UUID accountId, UUID otpId, String state, int attempt)
      throws SQLException {
    try (Connection connection = connection();
        PreparedStatement statement =
            connection.prepareStatement(
                """
                INSERT INTO %s.%s
                    (operation_id, account_id, otp_code_id, state, attempt_count, next_attempt_at)
                VALUES (?, ?, ?, ?, ?, ?)
                """.formatted(schema, TABLE))) {
      statement.setObject(1, operationId);
      statement.setObject(2, accountId);
      statement.setObject(3, otpId);
      statement.setString(4, state);
      statement.setInt(5, attempt);
      statement.setObject(6, Instant.now());
      statement.executeUpdate();
    }
  }

  private void insertWithRetryDefaults(String schema, UUID operationId, UUID accountId, UUID otpId)
      throws SQLException {
    try (Connection connection = connection();
        PreparedStatement statement =
            connection.prepareStatement(
                """
                INSERT INTO %s.%s (operation_id, account_id, otp_code_id, state)
                VALUES (?, ?, ?, 'PENDING_USER')
                """.formatted(schema, TABLE))) {
      statement.setObject(1, operationId);
      statement.setObject(2, accountId);
      statement.setObject(3, otpId);
      statement.executeUpdate();
    }
  }

  private int countOperationsForAccount(String schema, UUID accountId) throws SQLException {
    try (Connection connection = connection();
        PreparedStatement statement =
            connection.prepareStatement(
                "SELECT COUNT(*) FROM %s.%s WHERE account_id = ?".formatted(schema, TABLE))) {
      statement.setObject(1, accountId);
      try (ResultSet result = statement.executeQuery()) {
        result.next();
        return result.getInt(1);
      }
    }
  }

  private String operationStateForAccount(String schema, UUID accountId) throws SQLException {
    try (Connection connection = connection();
        PreparedStatement statement =
            connection.prepareStatement(
                "SELECT state FROM %s.%s WHERE account_id = ?".formatted(schema, TABLE))) {
      statement.setObject(1, accountId);
      try (ResultSet result = statement.executeQuery()) {
        result.next();
        return result.getString(1);
      }
    }
  }

  private int defaultAttemptCountForAccount(String schema, UUID accountId) throws SQLException {
    try (Connection connection = connection();
        PreparedStatement statement =
            connection.prepareStatement(
                "SELECT attempt_count FROM %s.%s WHERE account_id = ?".formatted(schema, TABLE))) {
      statement.setObject(1, accountId);
      try (ResultSet result = statement.executeQuery()) {
        result.next();
        return result.getInt(1);
      }
    }
  }

  private Instant defaultNextAttemptForAccount(String schema, UUID accountId) throws SQLException {
    try (Connection connection = connection();
        PreparedStatement statement =
            connection.prepareStatement(
                "SELECT next_attempt_at FROM %s.%s WHERE account_id = ?".formatted(schema, TABLE))) {
      statement.setObject(1, accountId);
      try (ResultSet result = statement.executeQuery()) {
        result.next();
        return result.getObject(1, Instant.class);
      }
    }
  }

  private Instant defaultCreatedAtForAccount(String schema, UUID accountId) throws SQLException {
    return timestampForAccount(schema, accountId, "created_at");
  }

  private Instant defaultUpdatedAtForAccount(String schema, UUID accountId) throws SQLException {
    return timestampForAccount(schema, accountId, "updated_at");
  }

  private Instant timestampForAccount(String schema, UUID accountId, String column) throws SQLException {
    try (Connection connection = connection();
        PreparedStatement statement =
            connection.prepareStatement(
                "SELECT %s FROM %s.%s WHERE account_id = ?".formatted(column, schema, TABLE))) {
      statement.setObject(1, accountId);
      try (ResultSet result = statement.executeQuery()) {
        result.next();
        return result.getObject(1, Instant.class);
      }
    }
  }

  private void assertInitialNullableFieldsAreNull(String schema, UUID accountId) throws SQLException {
    try (Connection connection = connection();
        PreparedStatement statement =
            connection.prepareStatement(
                """
                SELECT locked_until, last_error_code,
                    user_marked_at, auth_promoted_at, event_published_at
                FROM %s.%s
                WHERE account_id = ?
                """.formatted(schema, TABLE))) {
      statement.setObject(1, accountId);
      try (ResultSet result = statement.executeQuery()) {
        result.next();
        assertThat(result.getObject("locked_until")).isNull();
        assertThat(result.getObject("last_error_code")).isNull();
        assertThat(result.getObject("user_marked_at")).isNull();
        assertThat(result.getObject("auth_promoted_at")).isNull();
        assertThat(result.getObject("event_published_at")).isNull();
      }
    }
  }

  private Instant databaseClock() throws SQLException {
    try (Connection connection = connection(); Statement statement = connection.createStatement();
        ResultSet result = statement.executeQuery("SELECT clock_timestamp()")) {
      result.next();
      return result.getObject(1, Instant.class);
    }
  }

  private void assertWithinDatabaseClock(Instant value, Instant start, Instant end) {
    assertThat(value).isAfterOrEqualTo(start).isBeforeOrEqualTo(end);
  }

  private void executeSql(String schema, String sql) throws SQLException {
    try (Connection connection = connection(); Statement statement = connection.createStatement()) {
      connection.setSchema(schema);
      statement.execute(sql);
    }
  }

  private Connection connection() throws SQLException {
    return java.sql.DriverManager.getConnection(
        postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
  }
}
