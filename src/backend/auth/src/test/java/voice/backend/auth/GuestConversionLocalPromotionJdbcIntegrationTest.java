package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.time.Instant;
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
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.GuestConversionAdvanceResult;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.repository.JdbcAccountRepository;
import voice.backend.auth.repository.JdbcGuestConversionOperationRepository;
import voice.backend.auth.service.TransactionalGuestConversionLocalPromotion;

@Testcontainers(disabledWithoutDocker = true)
class GuestConversionLocalPromotionJdbcIntegrationTest {
  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db")
          .withUsername("voice")
          .withPassword("voice");

  private JdbcAccountRepository accounts;
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
    DriverManagerDataSource source =
        new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword());
    NamedParameterJdbcTemplate jdbc = new NamedParameterJdbcTemplate(source);
    accounts = new JdbcAccountRepository(jdbc);
    operations = new JdbcGuestConversionOperationRepository(jdbc);
    transactions = new TransactionTemplate(new DataSourceTransactionManager(source));
    jdbc.getJdbcTemplate().update("DELETE FROM guest_conversion_operations");
  }

  @Test
  void appliedAdvanceCommitsGuestToRegularUsingTheLeasedOperationExactly() {
    Account guest = guest();
    GuestConversionOperation operation = leasedOperation(guest.id());

    GuestConversionAdvanceResult result =
        new TransactionalGuestConversionLocalPromotion(transactions, accounts, operations)
            .promoteAndAdvance(operation, now());

    assertThat(result).isEqualTo(GuestConversionAdvanceResult.APPLIED);
    assertThat(accounts.findById(guest.id().toString()).orElseThrow().type()).isEqualTo("regular");
    assertThat(
            operations.advance(
                operation.operationId(), GuestConversionState.PENDING_USER, operation.lockedUntil(), now()))
        .isEqualTo(GuestConversionAdvanceResult.ALREADY_APPLIED);
  }

  @Test
  void promotionPreservesThePersistedSessionEpochAcrossTheMappedJdbcRow() {
    Account guest = guest();
    assertThat(accounts.incrementSessionEpoch(guest.id())).isEqualTo(2L);
    GuestConversionOperation operation = leasedOperation(guest.id());

    assertThat(
            new TransactionalGuestConversionLocalPromotion(transactions, accounts, operations)
                .promoteAndAdvance(operation, now()))
        .isEqualTo(GuestConversionAdvanceResult.APPLIED);
    assertThat(accounts.findById(guest.id().toString()).orElseThrow().sessionEpoch()).isEqualTo(2L);
  }

  @Test
  void leaseLostOrMissingAdvanceRollsBackTheLocalPromotion() {
    Account staleGuest = guest();
    GuestConversionOperation leased = leasedOperation(staleGuest.id());
    GuestConversionOperation staleLease = withLease(leased, leased.lockedUntil().plusSeconds(1));
    assertThat(
            new TransactionalGuestConversionLocalPromotion(transactions, accounts, operations)
                .promoteAndAdvance(staleLease, now()))
        .isEqualTo(GuestConversionAdvanceResult.LEASE_LOST);
    assertThat(accounts.findById(staleGuest.id().toString()).orElseThrow().type()).isEqualTo("guest");

    Account missingGuest = guest();
    GuestConversionOperation missing = operation(missingGuest.id());
    assertThat(
            new TransactionalGuestConversionLocalPromotion(transactions, accounts, operations)
                .promoteAndAdvance(missing, now()))
        .isEqualTo(GuestConversionAdvanceResult.NOT_FOUND);
    assertThat(accounts.findById(missingGuest.id().toString()).orElseThrow().type()).isEqualTo("guest");
  }

  @Test
  void alreadyAppliedIsRecoveryOnlyWhenAuthIsAlreadyRegular() {
    Account regular = accounts.create("regular-" + UUID.randomUUID() + "@example.com", null, "hash", "regular");
    GuestConversionOperation regularOperation = leasedOperation(regular.id());
    assertThat(
            operations.advance(
                regularOperation.operationId(), GuestConversionState.PENDING_USER, regularOperation.lockedUntil(), now()))
        .isEqualTo(GuestConversionAdvanceResult.APPLIED);

    assertThat(
            new TransactionalGuestConversionLocalPromotion(transactions, accounts, operations)
                .promoteAndAdvance(regularOperation, now()))
        .isEqualTo(GuestConversionAdvanceResult.ALREADY_APPLIED);
    assertThat(accounts.findById(regular.id().toString()).orElseThrow().type()).isEqualTo("regular");

    Account guest = guest();
    GuestConversionOperation guestOperation = leasedOperation(guest.id());
    assertThat(
            operations.advance(
                guestOperation.operationId(), GuestConversionState.PENDING_USER, guestOperation.lockedUntil(), now()))
        .isEqualTo(GuestConversionAdvanceResult.APPLIED);
    assertThatThrownBy(
            () ->
                new TransactionalGuestConversionLocalPromotion(transactions, accounts, operations)
                    .promoteAndAdvance(guestOperation, now()))
        .isInstanceOf(IllegalStateException.class);
    assertThat(accounts.findById(guest.id().toString()).orElseThrow().type()).isEqualTo("guest");
  }

  private Account guest() {
    return accounts.create("guest-" + UUID.randomUUID() + "@example.com", null, "hash", "guest");
  }

  private static Instant now() { return Instant.parse("2026-09-04T10:15:30Z"); }

  private static GuestConversionOperation operation(UUID accountId) {
    Instant now = now();
    return new GuestConversionOperation(
        UUID.randomUUID(), accountId, UUID.randomUUID(), GuestConversionState.PENDING_USER, 0, now,
        now.plusSeconds(60), null, null, null, null, now, now);
  }

  private GuestConversionOperation leasedOperation(UUID accountId) {
    operations.createOrResume(accountId, UUID.randomUUID(), now());
    return operations
        .leaseDue(GuestConversionState.PENDING_USER, 1, now(), now().plusSeconds(60))
        .getFirst();
  }

  private static GuestConversionOperation withLease(GuestConversionOperation operation, Instant lease) {
    return new GuestConversionOperation(
        operation.operationId(), operation.accountId(), operation.otpCodeId(), operation.state(),
        operation.attemptCount(), operation.nextAttemptAt(), lease, operation.lastErrorCode(),
        operation.userMarkedAt(), operation.authPromotedAt(), operation.eventPublishedAt(),
        operation.createdAt(), operation.updatedAt());
  }
}
