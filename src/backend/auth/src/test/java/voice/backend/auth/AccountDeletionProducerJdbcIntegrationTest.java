package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.when;

import java.sql.Connection;
import java.sql.Timestamp;
import java.time.Duration;
import java.time.Instant;
import java.util.UUID;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import javax.sql.DataSource;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.boot.test.mock.mockito.MockBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Import;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.springframework.transaction.PlatformTransactionManager;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.repository.Account;
import voice.backend.auth.repository.AccountDeletionOperationRepository;
import voice.backend.auth.repository.AccountRepository;
import voice.backend.auth.security.TokenBlacklist;
import voice.backend.auth.service.AccountDeletionOperationStarter;
import voice.backend.auth.service.AuthService;
import voice.backend.auth.service.RegisterCommand;
import voice.backend.auth.sessionepoch.SessionEpochFloorStore;
import voice.backend.auth.support.JdbcUserContractTestConfiguration;

/** The producer schedule is a post-lock database fact, never a caller clock hint. */
@SpringBootTest
@ActiveProfiles("integration")
@Testcontainers(disabledWithoutDocker = true)
@Import({JdbcUserContractTestConfiguration.class, AccountDeletionProducerJdbcIntegrationTest.Transactions.class})
class AccountDeletionProducerJdbcIntegrationTest {
  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db")
          .withUsername("voice")
          .withPassword("voice");

  @DynamicPropertySource
  static void databaseProperties(DynamicPropertyRegistry registry) {
    registry.add("voice.auth.jdbc.url", postgres::getJdbcUrl);
    registry.add("spring.datasource.username", postgres::getUsername);
    registry.add("spring.datasource.password", postgres::getPassword);
    registry.add("spring.flyway.user", postgres::getUsername);
    registry.add("spring.flyway.password", postgres::getPassword);
  }

  @Autowired AuthService authService;
  @Autowired AccountRepository accounts;
  @Autowired AccountDeletionOperationStarter starter;
  @Autowired AccountDeletionOperationRepository operations;
  @Autowired DataSource dataSource;
  @MockBean SessionEpochFloorStore floors;
  @MockBean TokenBlacklist blacklist;

  @BeforeEach
  void durableFloorIsAvailable() {
    when(floors.recordAtLeast(any(), anyLong())).thenAnswer(call -> call.getArgument(1));
  }

  @Test
  void scheduleUsesDatabaseClockAfterTheAccountRowLock() throws Exception {
    Account account = account();
    var executor = Executors.newSingleThreadExecutor();
    try (Connection blocker = dataSource.getConnection()) {
      blocker.setAutoCommit(false);
      try (var statement = blocker.prepareStatement("SELECT id FROM accounts WHERE id = ? FOR UPDATE")) {
        statement.setObject(1, account.id());
        statement.executeQuery().close();
      }
      Instant skewedCallerTime = databaseTime().plus(Duration.ofDays(7));
      var deletion = executor.submit(() -> starter.startOrResume(
          account, UUID.randomUUID(), "producer-clock-token-hash", skewedCallerTime));
      awaitAccountRowWait();
      Instant releasedAfter;
      try (var statement = blocker.createStatement();
          var result = statement.executeQuery("SELECT clock_timestamp()")) {
        assertThat(result.next()).isTrue();
        releasedAfter = result.getTimestamp(1).toInstant();
      }
      blocker.commit();
      var started = deletion.get(10, TimeUnit.SECONDS);
      Instant finishedBefore = databaseTime();

      assertThat(started.account().deletedAt()).isBetween(releasedAfter, finishedBefore);
      assertThat(started.operation().createdAt()).isEqualTo(started.account().deletedAt());
      assertThat(started.operation().createdAt()).isNotEqualTo(skewedCallerTime);
    } finally {
      executor.shutdownNow();
      assertThat(executor.awaitTermination(10, TimeUnit.SECONDS)).isTrue();
    }
  }

  @Test
  void retryPreservesTheOriginalScheduleAndOperation() {
    Account account = account();
    var first = starter.startOrResume(
        account, UUID.randomUUID(), "first-producer-token-hash", databaseTime());
    var retried = starter.startOrResume(
        first.account(), UUID.randomUUID(), "different-proposed-token-hash",
        databaseTime().plus(Duration.ofDays(7)));

    assertThat(retried.account().deletedAt()).isEqualTo(first.account().deletedAt());
    assertThat(retried.operation().operationId()).isEqualTo(first.operation().operationId());
    assertThat(retried.operation().createdAt()).isEqualTo(first.operation().createdAt());
    assertThat(retried.operation().restoreTokenHash()).isEqualTo(first.operation().restoreTokenHash());
    assertThat(operations.findByAccountAndEpoch(account.id(), first.account().sessionEpoch()))
        .contains(retried.operation());
  }

  private Account account() {
    String email = "producer-" + UUID.randomUUID() + "@example.com";
    authService.register(new RegisterCommand(
        email, null, "Correct horse battery staple", false, "{}"));
    return accounts.findByEmail(email).orElseThrow();
  }

  private Instant databaseTime() {
    return new JdbcTemplate(dataSource)
        .queryForObject("SELECT clock_timestamp()", Timestamp.class).toInstant();
  }

  private void awaitAccountRowWait() throws InterruptedException {
    long deadline = System.nanoTime() + TimeUnit.SECONDS.toNanos(10);
    var jdbc = new JdbcTemplate(dataSource);
    while (System.nanoTime() < deadline) {
      Integer waiting = jdbc.queryForObject(
          "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() "
              + "AND wait_event_type = 'Lock' AND query LIKE '%accounts%'", Integer.class);
      if (waiting != null && waiting > 0) return;
      Thread.sleep(10);
    }
    throw new AssertionError("deletion did not wait on the held account row");
  }

  @TestConfiguration(proxyBeanMethods = false)
  static class Transactions {
    @Bean
    PlatformTransactionManager transactionManager(DataSource source) {
      return new DataSourceTransactionManager(source);
    }
  }
}
