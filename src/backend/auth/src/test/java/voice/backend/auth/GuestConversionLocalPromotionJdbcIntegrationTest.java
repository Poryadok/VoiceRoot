package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.time.Instant;
import java.util.ArrayList;
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
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.GuestConversionAdvanceResult;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.repository.JdbcAccountRepository;
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
    accounts = new JdbcAccountRepository(new NamedParameterJdbcTemplate(source));
    transactions = new TransactionTemplate(new DataSourceTransactionManager(source));
  }

  @Test
  void appliedAdvanceCommitsGuestToRegularUsingTheLeasedOperationExactly() {
    Account guest = guest();
    GuestConversionOperation operation = operation(guest.id());
    RecordingAdvances advances = new RecordingAdvances(GuestConversionAdvanceResult.APPLIED);

    GuestConversionAdvanceResult result =
        new TransactionalGuestConversionLocalPromotion(transactions, accounts, advances)
            .promoteAndAdvance(operation, now());

    assertThat(result).isEqualTo(GuestConversionAdvanceResult.APPLIED);
    assertThat(accounts.findById(guest.id().toString()).orElseThrow().type()).isEqualTo("regular");
    assertThat(advances.calls)
        .containsExactly(new AdvanceCall(operation.operationId(), GuestConversionState.PENDING_USER, operation.lockedUntil(), now()));
  }

  @Test
  void leaseLostOrMissingAdvanceRollsBackTheLocalPromotion() {
    for (GuestConversionAdvanceResult result :
        List.of(GuestConversionAdvanceResult.LEASE_LOST, GuestConversionAdvanceResult.NOT_FOUND)) {
      Account guest = guest();
      GuestConversionOperation operation = operation(guest.id());
      RecordingAdvances advances = new RecordingAdvances(result);

      assertThat(
              new TransactionalGuestConversionLocalPromotion(transactions, accounts, advances)
                  .promoteAndAdvance(operation, now()))
          .isEqualTo(result);
      assertThat(accounts.findById(guest.id().toString()).orElseThrow().type()).isEqualTo("guest");
      assertThat(advances.calls)
          .containsExactly(new AdvanceCall(operation.operationId(), GuestConversionState.PENDING_USER, operation.lockedUntil(), now()));
    }
  }

  @Test
  void alreadyAppliedIsRecoveryOnlyWhenAuthIsAlreadyRegular() {
    Account regular = accounts.create("regular-" + UUID.randomUUID() + "@example.com", null, "hash", "regular");
    RecordingAdvances advances = new RecordingAdvances(GuestConversionAdvanceResult.ALREADY_APPLIED);

    assertThat(
            new TransactionalGuestConversionLocalPromotion(transactions, accounts, advances)
                .promoteAndAdvance(operation(regular.id()), now()))
        .isEqualTo(GuestConversionAdvanceResult.ALREADY_APPLIED);
    assertThat(accounts.findById(regular.id().toString()).orElseThrow().type()).isEqualTo("regular");

    Account guest = guest();
    assertThatThrownBy(
            () ->
                new TransactionalGuestConversionLocalPromotion(transactions, accounts, advances)
                    .promoteAndAdvance(operation(guest.id()), now()))
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

  private static final class RecordingAdvances implements GuestConversionOperationRepository {
    private final GuestConversionAdvanceResult result;
    private final List<AdvanceCall> calls = new ArrayList<>();
    private RecordingAdvances(GuestConversionAdvanceResult result) { this.result = result; }
    @Override public GuestConversionOperation createOrResume(UUID accountId, UUID otpCodeId, Instant now) { throw new UnsupportedOperationException(); }
    @Override public List<GuestConversionOperation> leaseDue(int batchSize, Instant now, Instant until) { return List.of(); }
    @Override public GuestConversionAdvanceResult advance(UUID id, GuestConversionState state, Instant lease, Instant now) {
      calls.add(new AdvanceCall(id, state, lease, now));
      return result;
    }
    @Override public Optional<GuestConversionOperation> recordFailure(UUID id, Instant lease, String error, Instant retry, Instant now) { throw new UnsupportedOperationException(); }
  }

  private record AdvanceCall(UUID operationId, GuestConversionState state, Instant lease, Instant now) {}
}
