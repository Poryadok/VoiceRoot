package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.Callable;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
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
import voice.backend.auth.repository.AccountDeletionAdvanceResult;
import voice.backend.auth.repository.AccountDeletionOperation;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.AccountDeletionState;
import voice.backend.auth.repository.JdbcAccountDeletionOperationRepository;

/** PostgreSQL proof that separate Auth instances cannot both own one deletion operation lease. */
@Testcontainers(disabledWithoutDocker = true)
class AccountDeletionOperationRepositoryJdbcIntegrationTest {
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
  void twoRepositoryInstances_claimExactlyOnceAndFenceTheFloorTransition() throws Exception {
    AccountDeletionOperationRepository first = repository();
    AccountDeletionOperationRepository second = repository();
    UUID accountId = UUID.randomUUID();
    seedAccount(accountId);
    Instant now = Instant.now().truncatedTo(ChronoUnit.MICROS);
    AccountDeletionOperation operation =
        first.createOrResume(UUID.randomUUID(), accountId, 7, "token-verifier", now);
    Instant leaseUntil = now.plus(2, ChronoUnit.MINUTES);

    CyclicBarrier start = new CyclicBarrier(2);
    ExecutorService executor = Executors.newFixedThreadPool(2);
    try {
      List<Optional<AccountDeletionOperation>> claims =
          executor
              .invokeAll(
                  List.of(
                      (Callable<Optional<AccountDeletionOperation>>)
                          () -> claimAfterBarrier(first, operation.operationId(), now, leaseUntil, start),
                      (Callable<Optional<AccountDeletionOperation>>)
                          () -> claimAfterBarrier(second, operation.operationId(), now, leaseUntil, start)),
                  30,
                  TimeUnit.SECONDS)
              .stream()
              .map(
                  future -> {
                    try {
                      return future.get();
                    } catch (Exception exception) {
                      throw new AssertionError("concurrent exact lease failed", exception);
                    }
                  })
              .toList();

      assertThat(claims).filteredOn(Optional::isPresent).hasSize(1);
      AccountDeletionOperation owner = claims.stream().flatMap(Optional::stream).findFirst().orElseThrow();
      assertThat(owner.lockedUntil()).isEqualTo(leaseUntil);
      assertThat(second.markFloorRecorded(operation.operationId(), leaseUntil, now.plusSeconds(1)))
          .isEqualTo(AccountDeletionAdvanceResult.APPLIED);
      assertThat(first.markFloorRecorded(operation.operationId(), leaseUntil, now.plusSeconds(1)))
          .isEqualTo(AccountDeletionAdvanceResult.ALREADY_APPLIED);
      assertThat(first.findByAccountAndEpoch(accountId, 7).orElseThrow().state())
          .isEqualTo(AccountDeletionState.PENDING_EVENT);
    } finally {
      executor.shutdownNow();
      assertThat(executor.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
      jdbc().update("DELETE FROM account_deletion_operations WHERE account_id = :accountId",
          new MapSqlParameterSource("accountId", accountId));
      jdbc().update("DELETE FROM accounts WHERE id = :accountId", new MapSqlParameterSource("accountId", accountId));
    }
  }

  private static Optional<AccountDeletionOperation> claimAfterBarrier(
      AccountDeletionOperationRepository repository,
      UUID operationId,
      Instant now,
      Instant leaseUntil,
      CyclicBarrier start) throws Exception {
    start.await(10, TimeUnit.SECONDS);
    return repository.lease(operationId, AccountDeletionState.PENDING_FLOOR, now, leaseUntil);
  }

  private AccountDeletionOperationRepository repository() {
    return new JdbcAccountDeletionOperationRepository(jdbc());
  }

  private void seedAccount(UUID accountId) {
    jdbc().update(
        """
        INSERT INTO accounts (id, password_hash, type, status)
        VALUES (:accountId, 'test-hash', 'regular', 'deleted')
        """,
        new MapSqlParameterSource("accountId", accountId));
  }

  private NamedParameterJdbcTemplate jdbc() {
    return new NamedParameterJdbcTemplate(
        new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword()));
  }
}
