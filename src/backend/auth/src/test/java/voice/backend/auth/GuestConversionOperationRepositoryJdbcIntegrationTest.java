package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Timestamp;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.stream.IntStream;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.namedparam.MapSqlParameterSource;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.repository.JdbcGuestConversionOperationRepository;

/**
 * T-049b RED repository contract: durable guest conversion creation must be idempotent and
 * linearizable against the Auth-owned PostgreSQL schema.
 */
@Testcontainers(disabledWithoutDocker = true)
class GuestConversionOperationRepositoryJdbcIntegrationTest {
  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db")
          .withUsername("voice")
          .withPassword("voice");

  @BeforeAll
  static void migrateAuthSchema() {
    Flyway.configure()
        .dataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())
        .locations(
            "filesystem:"
                + GuestConversionDurabilityMigrationContractTest.authProjectRoot()
                    .resolve("src/main/resources/db/migration"))
        .load()
        .migrate();
  }

  @Test
  void createOrResume_preservesTheFirstOperationAndOtpWithSaneInitialMetadata() {
    GuestConversionOperationRepository repository = repository();
    UUID accountId = UUID.randomUUID();
    UUID originalOtpCodeId = UUID.randomUUID();
    Instant createdAt = Instant.now().truncatedTo(ChronoUnit.MICROS);

    GuestConversionOperation created =
        repository.createOrResume(accountId, originalOtpCodeId, createdAt);
    GuestConversionOperation resumedWithSameOtp =
        repository.createOrResume(
            accountId, originalOtpCodeId, createdAt.plus(1, ChronoUnit.MINUTES));
    GuestConversionOperation resumedWithNewOtp =
        repository.createOrResume(
            accountId, UUID.randomUUID(), createdAt.plus(5, ChronoUnit.MINUTES));

    assertThat(created.operationId()).isNotNull();
    assertThat(resumedWithSameOtp.operationId()).isEqualTo(created.operationId());
    assertThat(resumedWithNewOtp.operationId()).isEqualTo(created.operationId());
    assertThat(resumedWithNewOtp.accountId()).isEqualTo(accountId);
    assertThat(resumedWithNewOtp.otpCodeId()).isEqualTo(originalOtpCodeId);
    assertThat(resumedWithNewOtp.state()).isEqualTo(GuestConversionState.PENDING_USER);
    assertThat(resumedWithNewOtp.attemptCount()).isZero();
    assertThat(resumedWithNewOtp.nextAttemptAt()).isEqualTo(createdAt);
    assertThat(resumedWithNewOtp.lockedUntil()).isNull();
    assertThat(resumedWithNewOtp.lastErrorCode()).isNull();
    assertThat(resumedWithNewOtp.userMarkedAt()).isNull();
    assertThat(resumedWithNewOtp.authPromotedAt()).isNull();
    assertThat(resumedWithNewOtp.eventPublishedAt()).isNull();
    assertThat(resumedWithNewOtp.createdAt()).isEqualTo(createdAt);
    assertThat(resumedWithNewOtp.updatedAt()).isEqualTo(createdAt);
    assertThat(operationCount(accountId)).isEqualTo(1);
    assertThat(operationIdForAccount(accountId)).isEqualTo(created.operationId());
  }

  @Test
  void createOrResume_concurrentCallsForOneAccountPersistOneSharedOperation() throws Exception {
    GuestConversionOperationRepository repository = repository();
    UUID accountId = UUID.randomUUID();
    UUID otpCodeId = UUID.randomUUID();
    Instant now = Instant.now().truncatedTo(ChronoUnit.MICROS);
    int callers = 8;
    CyclicBarrier start = new CyclicBarrier(callers);
    ExecutorService executor = Executors.newFixedThreadPool(callers);
    try {
      List<GuestConversionOperation> operations =
          executor
              .invokeAll(
                  IntStream.range(0, callers)
                      .<java.util.concurrent.Callable<GuestConversionOperation>>mapToObj(
                          ignored ->
                              () -> {
                                start.await(10, TimeUnit.SECONDS);
                                return repository.createOrResume(accountId, otpCodeId, now);
                              })
                      .toList(),
                  30,
                  TimeUnit.SECONDS)
              .stream()
              .map(
                  future -> {
                    try {
                      return future.get();
                    } catch (Exception ex) {
                      throw new AssertionError("concurrent createOrResume call failed", ex);
                    }
                  })
              .toList();

      assertThat(operations).hasSize(callers);
      UUID commonOperationId = operations.getFirst().operationId();
      assertThat(commonOperationId).isNotNull();
      assertThat(operations)
          .extracting(GuestConversionOperation::operationId)
          .containsOnly(commonOperationId);
      assertThat(operations).extracting(GuestConversionOperation::otpCodeId).containsOnly(otpCodeId);
      assertThat(operationCount(accountId)).isEqualTo(1);
      assertThat(operationIdForAccount(accountId)).isEqualTo(commonOperationId);
    } finally {
      executor.shutdownNow();
      assertThat(executor.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  @Test
  void createOrResume_rejectsOtpReusedByAnotherAccountWithoutCreatingAnotherRow() {
    GuestConversionOperationRepository repository = repository();
    UUID otpCodeId = UUID.randomUUID();
    UUID firstAccountId = UUID.randomUUID();
    UUID secondAccountId = UUID.randomUUID();
    Instant now = Instant.now().truncatedTo(ChronoUnit.MICROS);
    repository.createOrResume(firstAccountId, otpCodeId, now);

    assertThatThrownBy(() -> repository.createOrResume(secondAccountId, otpCodeId, now))
        .isInstanceOf(IllegalArgumentException.class);

    assertThat(operationCount(firstAccountId)).isEqualTo(1);
    assertThat(operationCount(secondAccountId)).isZero();
    assertThat(operationCountForOtp(otpCodeId)).isEqualTo(1);
  }

  @Nested
  class LeaseDueContract {
    @BeforeEach
    void isolateOperations() {
      deleteAllOperations();
    }

    @AfterEach
    void removeOperationsAndLeaseDelayTrigger() {
      removeLeaseDelayTrigger();
      deleteAllOperations();
    }

    @Test
    void leaseDue_rejectsInvalidArguments() {
      GuestConversionOperationRepository repository = repository();
      Instant now = Instant.now().truncatedTo(ChronoUnit.MICROS);

      assertThatThrownBy(() -> repository.leaseDue(0, now, now.plus(1, ChronoUnit.MINUTES)))
          .isInstanceOf(IllegalArgumentException.class);
      assertThatThrownBy(() -> repository.leaseDue(-1, now, now.plus(1, ChronoUnit.MINUTES)))
          .isInstanceOf(IllegalArgumentException.class);
      assertThatThrownBy(() -> repository.leaseDue(1, now, now))
          .isInstanceOf(IllegalArgumentException.class);
      assertThatThrownBy(() -> repository.leaseDue(1, now, now.minus(1, ChronoUnit.MICROS)))
          .isInstanceOf(IllegalArgumentException.class);
      assertThatThrownBy(() -> repository.leaseDue(1, null, now.plus(1, ChronoUnit.MINUTES)))
          .isInstanceOf(NullPointerException.class);
      assertThatThrownBy(() -> repository.leaseDue(1, now, null))
          .isInstanceOf(NullPointerException.class);
    }

    @Test
    void leaseDue_leasesOnlyEligibleRowsInDueCreatedAndOperationOrderAndPreservesAllFields() {
      GuestConversionOperationRepository repository = repository();
      Instant now = Instant.now().truncatedTo(ChronoUnit.MICROS);
      Instant leaseUntil = now.plus(2, ChronoUnit.MINUTES);
      Instant sharedDueAt = now.minus(5, ChronoUnit.MINUTES);
      Instant sharedCreatedAt = now.minus(10, ChronoUnit.MINUTES);
      GuestConversionOperation later =
          seedOperation(
              operation(
                  uuid("00000000-0000-0000-0000-000000000004"),
                  GuestConversionState.PENDING_USER,
                  7,
                  now.minus(1, ChronoUnit.MINUTES),
                  null,
                  now.minus(8, ChronoUnit.MINUTES)));
      GuestConversionOperation secondAtSameDueAndCreated =
          seedOperation(
              operation(
                  uuid("00000000-0000-0000-0000-000000000002"),
                  GuestConversionState.PENDING_USER,
                  2,
                  sharedDueAt,
                  null,
                  sharedCreatedAt));
      GuestConversionOperation futureDue =
          seedOperation(
              operation(
                  uuid("00000000-0000-0000-0000-000000000007"),
                  GuestConversionState.PENDING_USER,
                  1,
                  now.plus(1, ChronoUnit.MICROS),
                  null,
                  now.minus(7, ChronoUnit.MINUTES)));
      GuestConversionOperation firstAtSameDueAndCreated =
          seedOperation(
              operation(
                  uuid("00000000-0000-0000-0000-000000000001"),
                  GuestConversionState.PENDING_EVENT,
                  4,
                  sharedDueAt,
                  now.minus(1, ChronoUnit.MICROS),
                  sharedCreatedAt));
      GuestConversionOperation activelyLeased =
          seedOperation(
              operation(
                  uuid("00000000-0000-0000-0000-000000000006"),
                  GuestConversionState.PENDING_EVENT,
                  3,
                  now.minus(2, ChronoUnit.MINUTES),
                  now.plus(1, ChronoUnit.MINUTES),
                  now.minus(6, ChronoUnit.MINUTES)));
      GuestConversionOperation exactDueAndExpiredLeaseBoundary =
          seedOperation(
              operation(
                  uuid("00000000-0000-0000-0000-000000000005"),
                  GuestConversionState.PENDING_EVENT,
                  5,
                  now,
                  now,
                  now.minus(4, ChronoUnit.MINUTES)));
      GuestConversionOperation completed =
          seedOperation(
              operation(
                  uuid("00000000-0000-0000-0000-000000000003"),
                  GuestConversionState.COMPLETED,
                  8,
                  now.minus(3, ChronoUnit.MINUTES),
                  null,
                  now.minus(5, ChronoUnit.MINUTES)));

      List<GuestConversionOperation> leased = repository.leaseDue(10, now, leaseUntil);

      assertThat(leased)
          .extracting(GuestConversionOperation::operationId)
          .containsExactly(
              firstAtSameDueAndCreated.operationId(),
              secondAtSameDueAndCreated.operationId(),
              later.operationId(),
              exactDueAndExpiredLeaseBoundary.operationId());
      assertLeasedOperation(leased.get(0), firstAtSameDueAndCreated, leaseUntil);
      assertLeasedOperation(leased.get(1), secondAtSameDueAndCreated, leaseUntil);
      assertLeasedOperation(leased.get(2), later, leaseUntil);
      assertLeasedOperation(leased.get(3), exactDueAndExpiredLeaseBoundary, leaseUntil);
      assertPersistedLease(firstAtSameDueAndCreated, leaseUntil);
      assertPersistedLease(secondAtSameDueAndCreated, leaseUntil);
      assertPersistedLease(later, leaseUntil);
      assertPersistedLease(exactDueAndExpiredLeaseBoundary, leaseUntil);
      assertThat(operationById(futureDue.operationId())).isEqualTo(futureDue);
      assertThat(operationById(activelyLeased.operationId())).isEqualTo(activelyLeased);
      assertThat(operationById(completed.operationId())).isEqualTo(completed);
    }

    @Test
    void leaseDue_capsTheDeterministicallyOrderedBatchRegardlessOfInsertOrder() {
      GuestConversionOperationRepository repository = repository();
      Instant now = Instant.now().truncatedTo(ChronoUnit.MICROS);
      Instant leaseUntil = now.plus(2, ChronoUnit.MINUTES);
      GuestConversionOperation excludedByBatchCap =
          seedOperation(
              operation(
                  uuid("00000000-0000-0000-0000-000000000013"),
                  GuestConversionState.PENDING_USER,
                  2,
                  now.minus(1, ChronoUnit.MINUTES),
                  null,
                  now.minus(1, ChronoUnit.MINUTES)));
      GuestConversionOperation second =
          seedOperation(
              operation(
                  uuid("00000000-0000-0000-0000-000000000012"),
                  GuestConversionState.PENDING_EVENT,
                  1,
                  now.minus(2, ChronoUnit.MINUTES),
                  null,
                  now.minus(2, ChronoUnit.MINUTES)));
      GuestConversionOperation first =
          seedOperation(
              operation(
                  uuid("00000000-0000-0000-0000-000000000011"),
                  GuestConversionState.PENDING_USER,
                  0,
                  now.minus(3, ChronoUnit.MINUTES),
                  null,
                  now.minus(3, ChronoUnit.MINUTES)));

      List<GuestConversionOperation> leased = repository.leaseDue(2, now, leaseUntil);

      assertThat(leased)
          .extracting(GuestConversionOperation::operationId)
          .containsExactly(first.operationId(), second.operationId());
      assertPersistedLease(first, leaseUntil);
      assertPersistedLease(second, leaseUntil);
      assertThat(operationById(excludedByBatchCap.operationId())).isEqualTo(excludedByBatchCap);
    }

    @Test
    void leaseDue_concurrentLeasersUseSkipLockedToReceiveDisjointRowsWithoutDuplicates()
        throws Exception {
      GuestConversionOperationRepository repository = repository();
      Instant now = Instant.now().truncatedTo(ChronoUnit.MICROS);
      Instant leaseUntil = now.plus(2, ChronoUnit.MINUTES);
      List<GuestConversionOperation> dueOperations =
          List.of(
              seedOperation(
                  operation(
                      uuid("00000000-0000-0000-0000-000000000021"),
                      GuestConversionState.PENDING_USER,
                      0,
                      now.minus(5, ChronoUnit.MINUTES),
                      null,
                      now.minus(5, ChronoUnit.MINUTES))),
              seedOperation(
                  operation(
                      uuid("00000000-0000-0000-0000-000000000022"),
                      GuestConversionState.PENDING_EVENT,
                      1,
                      now.minus(4, ChronoUnit.MINUTES),
                      null,
                      now.minus(4, ChronoUnit.MINUTES))),
              seedOperation(
                  operation(
                      uuid("00000000-0000-0000-0000-000000000023"),
                      GuestConversionState.PENDING_USER,
                      2,
                      now.minus(3, ChronoUnit.MINUTES),
                      null,
                      now.minus(3, ChronoUnit.MINUTES))),
              seedOperation(
                  operation(
                      uuid("00000000-0000-0000-0000-000000000024"),
                      GuestConversionState.PENDING_EVENT,
                      3,
                      now.minus(2, ChronoUnit.MINUTES),
                      null,
                      now.minus(2, ChronoUnit.MINUTES))));
      installLeaseDelayTrigger();
      int leasers = 2;
      CyclicBarrier start = new CyclicBarrier(leasers);
      ExecutorService executor = Executors.newFixedThreadPool(leasers);
      try {
        List<GuestConversionOperation> leased =
            executor
                .invokeAll(
                    IntStream.range(0, leasers)
                        .<java.util.concurrent.Callable<List<GuestConversionOperation>>>mapToObj(
                            ignored ->
                                () -> {
                                  start.await(10, TimeUnit.SECONDS);
                                  return repository.leaseDue(3, now, leaseUntil);
                                })
                        .toList(),
                    30,
                    TimeUnit.SECONDS)
                .stream()
                .flatMap(
                    future -> {
                      try {
                        return future.get().stream();
                      } catch (Exception ex) {
                        throw new AssertionError("concurrent leaseDue call failed", ex);
                      }
                    })
                .toList();

        assertThat(leased).hasSize(dueOperations.size());
        assertThat(leased)
            .extracting(GuestConversionOperation::operationId)
            .containsExactlyInAnyOrderElementsOf(
                dueOperations.stream().map(GuestConversionOperation::operationId).toList())
            .doesNotHaveDuplicates();
        assertThat(leased)
            .extracting(GuestConversionOperation::lockedUntil)
            .containsOnly(leaseUntil);
      } finally {
        executor.shutdownNow();
        assertThat(executor.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
      }
    }
  }

  private GuestConversionOperationRepository repository() {
    return new JdbcGuestConversionOperationRepository(
        new NamedParameterJdbcTemplate(
            new DriverManagerDataSource(
                postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())));
  }

  private int operationCount(UUID accountId) {
    return jdbc()
        .queryForObject(
            "SELECT COUNT(*)::int FROM guest_conversion_operations WHERE account_id = :accountId",
            java.util.Map.of("accountId", accountId),
            Integer.class);
  }

  private int operationCountForOtp(UUID otpCodeId) {
    return jdbc()
        .queryForObject(
            "SELECT COUNT(*)::int FROM guest_conversion_operations WHERE otp_code_id = :otpCodeId",
            java.util.Map.of("otpCodeId", otpCodeId),
            Integer.class);
  }

  private UUID operationIdForAccount(UUID accountId) {
    return jdbc()
        .queryForObject(
            "SELECT operation_id FROM guest_conversion_operations WHERE account_id = :accountId",
            java.util.Map.of("accountId", accountId),
            UUID.class);
  }

  private void deleteAllOperations() {
    jdbc().update("DELETE FROM guest_conversion_operations", new MapSqlParameterSource());
  }

  private GuestConversionOperation seedOperation(GuestConversionOperation operation) {
    jdbc()
        .update(
            """
            INSERT INTO guest_conversion_operations (
                operation_id, account_id, otp_code_id, state, attempt_count,
                next_attempt_at, locked_until, last_error_code, user_marked_at,
                auth_promoted_at, event_published_at, created_at, updated_at)
            VALUES (
                :operationId, :accountId, :otpCodeId, :state, :attemptCount,
                :nextAttemptAt, :lockedUntil, :lastErrorCode, :userMarkedAt,
                :authPromotedAt, :eventPublishedAt, :createdAt, :updatedAt)
            """,
            new MapSqlParameterSource()
                .addValue("operationId", operation.operationId())
                .addValue("accountId", operation.accountId())
                .addValue("otpCodeId", operation.otpCodeId())
                .addValue("state", operation.state().name())
                .addValue("attemptCount", operation.attemptCount())
                .addValue("nextAttemptAt", timestamp(operation.nextAttemptAt()))
                .addValue("lockedUntil", timestamp(operation.lockedUntil()))
                .addValue("lastErrorCode", operation.lastErrorCode())
                .addValue("userMarkedAt", timestamp(operation.userMarkedAt()))
                .addValue("authPromotedAt", timestamp(operation.authPromotedAt()))
                .addValue("eventPublishedAt", timestamp(operation.eventPublishedAt()))
                .addValue("createdAt", timestamp(operation.createdAt()))
                .addValue("updatedAt", timestamp(operation.updatedAt())));
    return operation;
  }

  private GuestConversionOperation operation(
      UUID operationId,
      GuestConversionState state,
      int attemptCount,
      Instant nextAttemptAt,
      Instant lockedUntil,
      Instant createdAt) {
    boolean markedForUserAndAuth = state != GuestConversionState.PENDING_USER;
    return new GuestConversionOperation(
        operationId,
        UUID.randomUUID(),
        UUID.randomUUID(),
        state,
        attemptCount,
        nextAttemptAt,
        lockedUntil,
        "retryable-" + attemptCount,
        markedForUserAndAuth ? createdAt.plus(1, ChronoUnit.MINUTES) : null,
        markedForUserAndAuth ? createdAt.plus(2, ChronoUnit.MINUTES) : null,
        state == GuestConversionState.COMPLETED ? createdAt.plus(3, ChronoUnit.MINUTES) : null,
        createdAt,
        createdAt.plus(4, ChronoUnit.MICROS));
  }

  private void assertLeasedOperation(
      GuestConversionOperation actual, GuestConversionOperation expected, Instant leaseUntil) {
    assertThat(actual).isEqualTo(withLease(expected, leaseUntil));
  }

  private void assertPersistedLease(GuestConversionOperation expected, Instant leaseUntil) {
    assertThat(operationById(expected.operationId())).isEqualTo(withLease(expected, leaseUntil));
  }

  private GuestConversionOperation withLease(
      GuestConversionOperation operation, Instant leaseUntil) {
    return new GuestConversionOperation(
        operation.operationId(),
        operation.accountId(),
        operation.otpCodeId(),
        operation.state(),
        operation.attemptCount(),
        operation.nextAttemptAt(),
        leaseUntil,
        operation.lastErrorCode(),
        operation.userMarkedAt(),
        operation.authPromotedAt(),
        operation.eventPublishedAt(),
        operation.createdAt(),
        operation.updatedAt());
  }

  private GuestConversionOperation operationById(UUID operationId) {
    return jdbc()
        .query(
            """
            SELECT operation_id, account_id, otp_code_id, state, attempt_count,
                   next_attempt_at, locked_until, last_error_code, user_marked_at,
                   auth_promoted_at, event_published_at, created_at, updated_at
            FROM guest_conversion_operations
            WHERE operation_id = :operationId
            """,
            new MapSqlParameterSource("operationId", operationId),
            (rs, rowNum) -> operationFromRow(rs))
        .getFirst();
  }

  private GuestConversionOperation operationFromRow(ResultSet rs) throws SQLException {
    return new GuestConversionOperation(
        rs.getObject("operation_id", UUID.class),
        rs.getObject("account_id", UUID.class),
        rs.getObject("otp_code_id", UUID.class),
        GuestConversionState.valueOf(rs.getString("state")),
        rs.getInt("attempt_count"),
        instant(rs.getTimestamp("next_attempt_at")),
        instant(rs.getTimestamp("locked_until")),
        rs.getString("last_error_code"),
        instant(rs.getTimestamp("user_marked_at")),
        instant(rs.getTimestamp("auth_promoted_at")),
        instant(rs.getTimestamp("event_published_at")),
        instant(rs.getTimestamp("created_at")),
        instant(rs.getTimestamp("updated_at")));
  }

  private void installLeaseDelayTrigger() {
    jdbc()
        .getJdbcTemplate()
        .execute(
            """
            CREATE OR REPLACE FUNCTION guest_conversion_lease_test_delay()
            RETURNS trigger
            LANGUAGE plpgsql
            AS $$
            BEGIN
              PERFORM pg_sleep(0.25);
              RETURN NEW;
            END;
            $$
            """);
    jdbc()
        .getJdbcTemplate()
        .execute(
            """
            CREATE TRIGGER guest_conversion_lease_test_delay
            BEFORE UPDATE OF locked_until ON guest_conversion_operations
            FOR EACH ROW
            WHEN (OLD.locked_until IS DISTINCT FROM NEW.locked_until)
            EXECUTE FUNCTION guest_conversion_lease_test_delay()
            """);
  }

  private void removeLeaseDelayTrigger() {
    jdbc()
        .getJdbcTemplate()
        .execute(
            "DROP TRIGGER IF EXISTS guest_conversion_lease_test_delay"
                + " ON guest_conversion_operations");
    jdbc()
        .getJdbcTemplate()
        .execute("DROP FUNCTION IF EXISTS guest_conversion_lease_test_delay()");
  }

  private static UUID uuid(String value) {
    return UUID.fromString(value);
  }

  private static Timestamp timestamp(Instant value) {
    return value == null ? null : Timestamp.from(value);
  }

  private static Instant instant(Timestamp value) {
    return value == null ? null : value.toInstant();
  }

  private NamedParameterJdbcTemplate jdbc() {
    return new NamedParameterJdbcTemplate(
        new DriverManagerDataSource(
            postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword()));
  }
}
