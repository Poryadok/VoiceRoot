package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatIllegalArgumentException;

import java.time.Instant;
import java.util.List;
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
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountSessionEpoch;
import voice.backend.auth.repository.JdbcAccountRepository;

/** Real PostgreSQL proof for Auth-owned session-epoch max and UUID keyset semantics. */
@Testcontainers(disabledWithoutDocker = true)
class SessionEpochRepositoryJdbcIntegrationTest {
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
  void keysetPagesCrossUnsignedUuidHighBitAndRetainSoftDeletedRows() {
    UUID zero = UUID.fromString("00000000-0000-0000-0000-000000000001");
    UUID lowerHighBit = UUID.fromString("7fffffff-ffff-ffff-ffff-ffffffffffff");
    UUID upperHighBitLowerLsb = UUID.fromString("80000000-0000-0000-0000-000000000001");
    UUID upperHighBit = UUID.fromString("80000000-0000-0000-8000-000000000000");
    UUID allBits = UUID.fromString("ffffffff-ffff-ffff-ffff-ffffffffffff");
    seedAccount(zero, 2, "active", false);
    seedAccount(lowerHighBit, 3, "active", false);
    seedAccount(upperHighBitLowerLsb, 4, "active", false);
    seedAccount(upperHighBit, 5, "deleted", true);
    seedAccount(allBits, 9, "active", false);

    JdbcAccountRepository accounts = repository();
    List<AccountSessionEpoch> first = accounts.pageSessionEpochsAfter(null, 2);
    List<AccountSessionEpoch> second = accounts.pageSessionEpochsAfter(first.getLast().accountId(), 2);
    List<AccountSessionEpoch> third = accounts.pageSessionEpochsAfter(second.getLast().accountId(), 2);
    List<AccountSessionEpoch> terminal = accounts.pageSessionEpochsAfter(third.getLast().accountId(), 2);

    assertThat(first)
        .containsExactly(new AccountSessionEpoch(zero, 2L), new AccountSessionEpoch(lowerHighBit, 3L));
    assertThat(second)
        .containsExactly(
            new AccountSessionEpoch(upperHighBitLowerLsb, 4L),
            new AccountSessionEpoch(upperHighBit, 5L));
    assertThat(third).containsExactly(new AccountSessionEpoch(allBits, 9L));
    assertThat(terminal).isEmpty();
    assertThatIllegalArgumentException().isThrownBy(() -> accounts.pageSessionEpochsAfter(null, 0));
    assertThatIllegalArgumentException().isThrownBy(() -> accounts.pageSessionEpochsAfter(null, -1));
  }

  @Test
  void advanceRetainsAllNonEpochFieldsAndRejectsInvalidOrMissingAccounts() {
    UUID accountId = UUID.randomUUID();
    Instant createdAt = Instant.parse("2026-09-06T08:00:00Z");
    Instant deletedAt = Instant.parse("2026-09-06T08:01:00Z");
    seedNondefaultAccount(accountId, createdAt, deletedAt);
    JdbcAccountRepository accounts = repository();
    Account before = accounts.findById(accountId.toString()).orElseThrow();

    assertThat(accounts.advanceSessionEpochAtLeast(accountId, 7L)).isEqualTo(7L);
    assertThat(accounts.advanceSessionEpochAtLeast(accountId, 3L)).isEqualTo(7L);
    Account after = accounts.findById(accountId.toString()).orElseThrow();
    assertThat(after)
        .extracting(
            Account::id,
            Account::email,
            Account::phone,
            Account::passwordHash,
            Account::type,
            Account::status,
            Account::totpEnabled,
            Account::createdAt,
            Account::deletedAt,
            Account::sessionEpoch)
        .containsExactly(
            before.id(),
            before.email(),
            before.phone(),
            before.passwordHash(),
            before.type(),
            before.status(),
            before.totpEnabled(),
            before.createdAt(),
            before.deletedAt(),
            7L);
    assertThat(after.totpSecret()).containsExactly(before.totpSecret());
    assertThatIllegalArgumentException().isThrownBy(() -> accounts.advanceSessionEpochAtLeast(accountId, 0));
    assertThatIllegalArgumentException().isThrownBy(() -> accounts.advanceSessionEpochAtLeast(accountId, -1));
    assertThatIllegalArgumentException()
        .isThrownBy(() -> accounts.advanceSessionEpochAtLeast(UUID.randomUUID(), 2));
  }

  @Test
  void concurrentAdvanceRequestsConvergeAtMaximumAndLowerFollowUpCannotReduceIt()
      throws Exception {
    UUID accountId = UUID.randomUUID();
    seedAccount(accountId, 1, "active", false);
    JdbcAccountRepository first = repository();
    JdbcAccountRepository second = repository();
    CyclicBarrier start = new CyclicBarrier(2);
    ExecutorService workers = Executors.newFixedThreadPool(2);
    try {
      List<Long> returned =
          workers
              .invokeAll(
                  List.of(
                      (Callable<Long>) () -> advanceAfterBarrier(first, accountId, 7, start),
                      (Callable<Long>) () -> advanceAfterBarrier(second, accountId, 13, start)),
                  30,
                  TimeUnit.SECONDS)
              .stream()
              .map(
                  future -> {
                    try {
                      return future.get();
                    } catch (Exception exception) {
                      throw new AssertionError("concurrent max advance failed", exception);
                    }
                  })
              .toList();

      assertThat(returned).allMatch(epoch -> epoch == 7L || epoch == 13L);
      assertThat(first.advanceSessionEpochAtLeast(accountId, 3)).isEqualTo(13L);
      assertThat(first.findById(accountId.toString()).orElseThrow().sessionEpoch()).isEqualTo(13L);
    } finally {
      workers.shutdownNow();
      assertThat(workers.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
      jdbc().update("DELETE FROM accounts WHERE id = :id", new MapSqlParameterSource("id", accountId));
    }
  }

  private static long advanceAfterBarrier(
      JdbcAccountRepository accounts, UUID accountId, long requestedEpoch, CyclicBarrier start)
      throws Exception {
    start.await(10, TimeUnit.SECONDS);
    return accounts.advanceSessionEpochAtLeast(accountId, requestedEpoch);
  }

  private void seedAccount(UUID accountId, long epoch, String status, boolean deleted) {
    jdbc().update(
        """
        INSERT INTO accounts (id, password_hash, type, status, session_epoch, deleted_at)
        VALUES (:id, 'test-hash', 'regular', :status, :epoch,
                CASE WHEN :deleted THEN CURRENT_TIMESTAMP ELSE NULL END)
        """,
        new MapSqlParameterSource()
            .addValue("id", accountId)
            .addValue("epoch", epoch)
            .addValue("status", status)
            .addValue("deleted", deleted));
  }

  private void seedNondefaultAccount(UUID accountId, Instant createdAt, Instant deletedAt) {
    jdbc().update(
        """
        INSERT INTO accounts
            (id, email, phone, password_hash, type, status, totp_secret, totp_enabled,
             session_epoch, created_at, deleted_at)
        VALUES
            (:id, :email, :phone, :passwordHash, 'guest', 'deleted', :totpSecret, TRUE,
             1, :createdAt, :deletedAt)
        """,
        new MapSqlParameterSource()
            .addValue("id", accountId)
            .addValue("email", "epoch-retain-" + accountId + "@example.com")
            .addValue("phone", "+15550001111")
            .addValue("passwordHash", "nondefault-hash")
            .addValue("totpSecret", new byte[] {4, 5, 6})
            .addValue("createdAt", java.sql.Timestamp.from(createdAt))
            .addValue("deletedAt", java.sql.Timestamp.from(deletedAt)));
  }

  private JdbcAccountRepository repository() {
    return new JdbcAccountRepository(jdbc());
  }

  private NamedParameterJdbcTemplate jdbc() {
    return new NamedParameterJdbcTemplate(
        new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword()));
  }
}
