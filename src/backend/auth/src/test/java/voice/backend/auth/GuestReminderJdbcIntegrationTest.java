package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.concurrent.Callable;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import org.flywaydb.core.Flyway;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.repository.JdbcAccountRepository;

@Testcontainers(disabledWithoutDocker = true)
class GuestReminderJdbcIntegrationTest {
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
  void concurrentJdbcClaimsHaveOneWinnerAndPreserveTheWinningTimestamp() throws Exception {
    JdbcAccountRepository setup = repository();
    var guest = setup.create(null, null, "test-password-hash", "guest");
    var regular = setup.create("reminder-jdbc@example.test", null, "test-password-hash", "regular");
    Instant now = Instant.parse("2026-09-28T12:00:00Z");
    Instant cutoff = now.minus(Duration.ofHours(24));
    var barrier = new CyclicBarrier(2);
    Callable<Boolean> claim = () -> {
      barrier.await(10, TimeUnit.SECONDS);
      return repository().claimGuestReminder(guest.id(), now, cutoff);
    };

    var pool = Executors.newFixedThreadPool(2);
    try {
      var first = pool.submit(claim);
      var second = pool.submit(claim);
      assertThat(List.of(first.get(15, TimeUnit.SECONDS), second.get(15, TimeUnit.SECONDS)))
          .containsExactlyInAnyOrder(true, false);
    } finally {
      pool.shutdownNow();
    }

    assertThat(setup.getGuestReminderLastShownAt(guest.id())).contains(now);
    assertThat(setup.claimGuestReminder(guest.id(), now.plus(Duration.ofHours(23)),
        now.minus(Duration.ofHours(1)))).isFalse();
    assertThat(setup.claimGuestReminder(regular.id(), now, cutoff)).isFalse();
  }

  private static JdbcAccountRepository repository() {
    return new JdbcAccountRepository(new NamedParameterJdbcTemplate(
        new DriverManagerDataSource(postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())));
  }
}
