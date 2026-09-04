package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.springframework.transaction.support.TransactionTemplate;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.repository.GuestConversionAdvanceResult;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.repository.JdbcGuestConversionOperationRepository;
import voice.backend.auth.repository.JdbcOtpCodeRepository;
import voice.backend.auth.repository.OtpCodeRecord;
import voice.backend.auth.service.GuestConversionOtpAcceptance;
import voice.backend.auth.service.TransactionalGuestConversionOtpAcceptance;

@Testcontainers(disabledWithoutDocker = true)
class GuestConversionOtpAcceptanceJdbcIntegrationTest {
  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db")
          .withUsername("voice")
          .withPassword("voice");

  private NamedParameterJdbcTemplate jdbc;
  private JdbcOtpCodeRepository otpCodes;
  private JdbcGuestConversionOperationRepository operations;
  private TransactionTemplate transactions;

  @BeforeAll
  static void migrate() {
    Flyway.configure()
        .dataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())
        .locations(
            "filesystem:"
                + GuestConversionDurabilityMigrationContractTest.authProjectRoot()
                    .resolve("src/main/resources/db/migration"))
        .load()
        .migrate();
  }

  @BeforeEach
  void setUp() {
    DriverManagerDataSource dataSource =
        new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
    jdbc = new NamedParameterJdbcTemplate(dataSource);
    otpCodes = new JdbcOtpCodeRepository(jdbc);
    operations = new JdbcGuestConversionOperationRepository(jdbc);
    transactions = new TransactionTemplate(new DataSourceTransactionManager(dataSource));
    jdbc.getJdbcTemplate().update("DELETE FROM guest_conversion_operations");
    jdbc.getJdbcTemplate().update("DELETE FROM otp_codes");
  }

  @Test
  void successfulAcceptanceConsumesExactOtpAndCreatesOneDurableOperationInTheSameAuthTransaction() {
    UUID accountId = UUID.randomUUID();
    Instant now = Instant.parse("2026-09-04T10:15:30Z").truncatedTo(ChronoUnit.MICROS);
    OtpCodeRecord otp = otpCodes.create(accountId, "a1", "email_verify", now.plusSeconds(600), now);
    GuestConversionOtpAcceptance acceptance =
        new TransactionalGuestConversionOtpAcceptance(transactions, otpCodes, operations);

    acceptance.acceptVerifiedGuestEmailOtp(accountId, otp, now);
    acceptance.acceptVerifiedGuestEmailOtp(accountId, otp, now.plusSeconds(1));

    assertThat(otpCodes.findLatestValid(accountId, "email_verify", now)).isEmpty();
    List<GuestConversionOperation> persisted = operationsFor(accountId);
    assertThat(persisted).hasSize(1);
    assertThat(persisted.getFirst().otpCodeId()).isEqualTo(otp.id());
    assertThat(persisted.getFirst().state()).isEqualTo(GuestConversionState.PENDING_USER);
  }

  @Test
  void operationCreateFailureRollsBackTheExactOtpConsumptionAndOperationRow() {
    UUID accountId = UUID.randomUUID();
    Instant now = Instant.parse("2026-09-04T10:15:30Z").truncatedTo(ChronoUnit.MICROS);
    OtpCodeRecord otp = otpCodes.create(accountId, "b2", "email_verify", now.plusSeconds(600), now);
    GuestConversionOperationRepository failureAfterInsert = new FailureAfterCreate(operations);
    GuestConversionOtpAcceptance acceptance =
        new TransactionalGuestConversionOtpAcceptance(transactions, otpCodes, failureAfterInsert);

    assertThatThrownBy(() -> acceptance.acceptVerifiedGuestEmailOtp(accountId, otp, now))
        .isInstanceOf(IllegalStateException.class)
        .hasMessage("operation persistence interrupted");

    assertThat(otpCodes.findLatestValid(accountId, "email_verify", now)).contains(otp);
    assertThat(operationsFor(accountId)).isEmpty();
  }

  private List<GuestConversionOperation> operationsFor(UUID accountId) {
    return jdbc.query(
        """
        SELECT operation_id, account_id, otp_code_id, state, attempt_count, next_attempt_at,
               locked_until, last_error_code, user_marked_at, auth_promoted_at, event_published_at,
               created_at, updated_at
        FROM guest_conversion_operations WHERE account_id = :accountId
        """,
        new org.springframework.jdbc.core.namedparam.MapSqlParameterSource("accountId", accountId),
        (rs, row) -> new GuestConversionOperation(
            rs.getObject("operation_id", UUID.class), rs.getObject("account_id", UUID.class),
            rs.getObject("otp_code_id", UUID.class), GuestConversionState.valueOf(rs.getString("state")),
            rs.getInt("attempt_count"), rs.getTimestamp("next_attempt_at").toInstant(),
            rs.getTimestamp("locked_until") == null ? null : rs.getTimestamp("locked_until").toInstant(),
            rs.getString("last_error_code"),
            rs.getTimestamp("user_marked_at") == null ? null : rs.getTimestamp("user_marked_at").toInstant(),
            rs.getTimestamp("auth_promoted_at") == null ? null : rs.getTimestamp("auth_promoted_at").toInstant(),
            rs.getTimestamp("event_published_at") == null ? null : rs.getTimestamp("event_published_at").toInstant(),
            rs.getTimestamp("created_at").toInstant(), rs.getTimestamp("updated_at").toInstant()));
  }

  private static final class FailureAfterCreate implements GuestConversionOperationRepository {
    private final GuestConversionOperationRepository delegate;
    private FailureAfterCreate(GuestConversionOperationRepository delegate) { this.delegate = delegate; }
    @Override public GuestConversionOperation createOrResume(UUID accountId, UUID otpCodeId, Instant now) {
      delegate.createOrResume(accountId, otpCodeId, now);
      throw new IllegalStateException("operation persistence interrupted");
    }
    @Override public List<GuestConversionOperation> leaseDue(int batchSize, Instant now, Instant until) { return delegate.leaseDue(batchSize, now, until); }
    @Override public GuestConversionAdvanceResult advance(UUID id, GuestConversionState state, Instant lease, Instant now) { return delegate.advance(id, state, lease, now); }
    @Override public Optional<GuestConversionOperation> recordFailure(UUID id, Instant lease, String error, Instant retry, Instant now) { return delegate.recordFailure(id, lease, error, retry, now); }
  }
}
