package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

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
import org.junit.jupiter.api.BeforeAll;
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

  @Test
  void leaseDue_rejectsNonPositiveBatchSizesAndLeaseWindowsThatDoNotFollowNow() {
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
  }

  @Test
  void leaseDue_leasesOnlyEligibleRowsInDueThenCreatedOrderAndPreservesOperationData() {
    deleteAllOperations();
    GuestConversionOperationRepository repository = repository();
    Instant now = Instant.now().truncatedTo(ChronoUnit.MICROS);
    Instant leaseUntil = now.plus(2, ChronoUnit.MINUTES);
    Instant sharedDueAt = now.minus(5, ChronoUnit.MINUTES);
    GuestConversionOperation first =
        seedOperation(
            GuestConversionState.PENDING_EVENT,
            4,
            sharedDueAt,
            now.minus(1, ChronoUnit.MICROS),
            now.minus(10, ChronoUnit.MINUTES));
    GuestConversionOperation second =
        seedOperation(
            GuestConversionState.PENDING_USER,
            2,
            sharedDueAt,
            null,
            now.minus(9, ChronoUnit.MINUTES));
    GuestConversionOperation third =
        seedOperation(
            GuestConversionState.PENDING_USER,
            7,
            now.minus(1, ChronoUnit.MINUTES),
            null,
            now.minus(8, ChronoUnit.MINUTES));
    GuestConversionOperation futureDue =
        seedOperation(
            GuestConversionState.PENDING_USER,
            1,
            now.plus(1, ChronoUnit.MICROS),
            null,
            now.minus(7, ChronoUnit.MINUTES));
    GuestConversionOperation activelyLeased =
        seedOperation(
            GuestConversionState.PENDING_EVENT,
            3,
            now.minus(2, ChronoUnit.MINUTES),
            now.plus(1, ChronoUnit.MINUTES),
            now.minus(6, ChronoUnit.MINUTES));
    GuestConversionOperation completed =
        seedOperation(
            GuestConversionState.COMPLETED,
            8,
            now.minus(3, ChronoUnit.MINUTES),
            null,
            now.minus(5, ChronoUnit.MINUTES));

    List<GuestConversionOperation> leased = repository.leaseDue(10, now, leaseUntil);

    assertThat(leased)
        .extracting(GuestConversionOperation::operationId)
        .containsExactly(first.operationId(), second.operationId(), third.operationId());
    assertLeasedOperation(leased.get(0), first, leaseUntil);
    assertLeasedOperation(leased.get(1), second, leaseUntil);
    assertLeasedOperation(leased.get(2), third, leaseUntil);
    assertThat(lockedUntil(first.operationId())).isEqualTo(leaseUntil);
    assertThat(lockedUntil(second.operationId())).isEqualTo(leaseUntil);
    assertThat(lockedUntil(third.operationId())).isEqualTo(leaseUntil);
    assertThat(lockedUntil(futureDue.operationId())).isNull();
    assertThat(lockedUntil(activelyLeased.operationId())).isEqualTo(activelyLeased.lockedUntil());
    assertThat(lockedUntil(completed.operationId())).isNull();
  }

  @Test
  void leaseDue_capsTheDeterministicallyOrderedBatch() {
    deleteAllOperations();
    GuestConversionOperationRepository repository = repository();
    Instant now = Instant.now().truncatedTo(ChronoUnit.MICROS);
    Instant leaseUntil = now.plus(2, ChronoUnit.MINUTES);
    GuestConversionOperation first =
        seedOperation(
            GuestConversionState.PENDING_USER,
            0,
            now.minus(3, ChronoUnit.MINUTES),
            null,
            now.minus(3, ChronoUnit.MINUTES));
    GuestConversionOperation second =
        seedOperation(
            GuestConversionState.PENDING_EVENT,
            1,
            now.minus(2, ChronoUnit.MINUTES),
            null,
            now.minus(2, ChronoUnit.MINUTES));
    GuestConversionOperation excludedByBatchCap =
        seedOperation(
            GuestConversionState.PENDING_USER,
            2,
            now.minus(1, ChronoUnit.MINUTES),
            null,
            now.minus(1, ChronoUnit.MINUTES));

    List<GuestConversionOperation> leased = repository.leaseDue(2, now, leaseUntil);

    assertThat(leased)
        .extracting(GuestConversionOperation::operationId)
        .containsExactly(first.operationId(), second.operationId());
    assertThat(lockedUntil(first.operationId())).isEqualTo(leaseUntil);
    assertThat(lockedUntil(second.operationId())).isEqualTo(leaseUntil);
    assertThat(lockedUntil(excludedByBatchCap.operationId())).isNull();
  }

  @Test
  void leaseDue_concurrentLeasersReceiveDisjointRowsWithoutDuplicates() throws Exception {
    deleteAllOperations();
    GuestConversionOperationRepository repository = repository();
    Instant now = Instant.now().truncatedTo(ChronoUnit.MICROS);
    Instant leaseUntil = now.plus(2, ChronoUnit.MINUTES);
    List<GuestConversionOperation> dueOperations =
        List.of(
            seedOperation(
                GuestConversionState.PENDING_USER,
                0,
                now.minus(5, ChronoUnit.MINUTES),
                null,
                now.minus(5, ChronoUnit.MINUTES)),
            seedOperation(
                GuestConversionState.PENDING_EVENT,
                1,
                now.minus(4, ChronoUnit.MINUTES),
                null,
                now.minus(4, ChronoUnit.MINUTES)),
            seedOperation(
                GuestConversionState.PENDING_USER,
                2,
                now.minus(3, ChronoUnit.MINUTES),
                null,
                now.minus(3, ChronoUnit.MINUTES)),
            seedOperation(
                GuestConversionState.PENDING_EVENT,
                3,
                now.minus(2, ChronoUnit.MINUTES),
                null,
                now.minus(2, ChronoUnit.MINUTES)));
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
      assertThat(leased).extracting(GuestConversionOperation::lockedUntil).containsOnly(leaseUntil);
    } finally {
      executor.shutdownNow();
      assertThat(executor.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
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

  private GuestConversionOperation seedOperation(
      GuestConversionState state,
      int attemptCount,
      Instant nextAttemptAt,
      Instant lockedUntil,
      Instant createdAt) {
    UUID operationId = UUID.randomUUID();
    UUID accountId = UUID.randomUUID();
    UUID otpCodeId = UUID.randomUUID();
    jdbc()
        .update(
            """
            INSERT INTO guest_conversion_operations (
                operation_id, account_id, otp_code_id, state, attempt_count,
                next_attempt_at, locked_until, created_at, updated_at)
            VALUES (
                :operationId, :accountId, :otpCodeId, :state, :attemptCount,
                :nextAttemptAt, :lockedUntil, :createdAt, :updatedAt)
            """,
            new MapSqlParameterSource()
                .addValue("operationId", operationId)
                .addValue("accountId", accountId)
                .addValue("otpCodeId", otpCodeId)
                .addValue("state", state.name())
                .addValue("attemptCount", attemptCount)
                .addValue("nextAttemptAt", Timestamp.from(nextAttemptAt))
                .addValue("lockedUntil", lockedUntil == null ? null : Timestamp.from(lockedUntil))
                .addValue("createdAt", Timestamp.from(createdAt))
                .addValue("updatedAt", Timestamp.from(createdAt)));
    return new GuestConversionOperation(
        operationId,
        accountId,
        otpCodeId,
        state,
        attemptCount,
        nextAttemptAt,
        lockedUntil,
        null,
        null,
        null,
        null,
        createdAt,
        createdAt);
  }

  private void assertLeasedOperation(
      GuestConversionOperation actual, GuestConversionOperation expected, Instant leaseUntil) {
    assertThat(actual.operationId()).isEqualTo(expected.operationId());
    assertThat(actual.accountId()).isEqualTo(expected.accountId());
    assertThat(actual.otpCodeId()).isEqualTo(expected.otpCodeId());
    assertThat(actual.state()).isEqualTo(expected.state());
    assertThat(actual.attemptCount()).isEqualTo(expected.attemptCount());
    assertThat(actual.nextAttemptAt()).isEqualTo(expected.nextAttemptAt());
    assertThat(actual.lockedUntil()).isEqualTo(leaseUntil);
  }

  private Instant lockedUntil(UUID operationId) {
    return jdbc()
        .queryForObject(
            "SELECT locked_until FROM guest_conversion_operations WHERE operation_id = :operationId",
            new MapSqlParameterSource("operationId", operationId),
            Instant.class);
  }

  private NamedParameterJdbcTemplate jdbc() {
    return new NamedParameterJdbcTemplate(
        new DriverManagerDataSource(
            postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword()));
  }
}
