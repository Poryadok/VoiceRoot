package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

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
      assertThat(operations)
          .extracting(GuestConversionOperation::operationId)
          .containsOnly(operations.getFirst().operationId());
      assertThat(operations).extracting(GuestConversionOperation::otpCodeId).containsOnly(otpCodeId);
      assertThat(operationCount(accountId)).isEqualTo(1);
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

  private NamedParameterJdbcTemplate jdbc() {
    return new NamedParameterJdbcTemplate(
        new DriverManagerDataSource(
            postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword()));
  }
}
