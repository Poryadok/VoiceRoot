package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import java.time.Instant;
import java.util.Map;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.annotation.Import;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.lifecycle.GuestAccountSweeper;
import voice.backend.auth.support.JdbcUserContractTestConfiguration;

@SpringBootTest
@ActiveProfiles("integration")
@Testcontainers(disabledWithoutDocker = true)
@Import(JdbcUserContractTestConfiguration.class)
class GuestAccountSweeperJdbcIntegrationTest {
  @Container
  static final PostgreSQLContainer<?> postgres =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_db")
          .withUsername("voice")
          .withPassword("voice");

  @Container
  static final GenericContainer<?> redis =
      new GenericContainer<>(DockerImageName.parse("redis:7-alpine")).withExposedPorts(6379);

  @DynamicPropertySource
  static void registerProps(DynamicPropertyRegistry registry) {
    registry.add("voice.auth.jdbc.url", postgres::getJdbcUrl);
    registry.add("spring.datasource.username", postgres::getUsername);
    registry.add("spring.datasource.password", postgres::getPassword);
    registry.add("spring.flyway.user", postgres::getUsername);
    registry.add("spring.flyway.password", postgres::getPassword);
    registry.add("spring.data.redis.host", redis::getHost);
    registry.add("spring.data.redis.port", () -> String.valueOf(redis.getMappedPort(6379)));
  }

  @Autowired NamedParameterJdbcTemplate jdbc;
  @Autowired GuestAccountSweeper sweeper;

  @Test
  void sweeperDoesNotDeleteGuestsWithNullLastOnlineAt() {
    UUID guestId = UUID.randomUUID();
    jdbc.update(
        """
        INSERT INTO accounts (id, password_hash, type, status)
        VALUES (:id, 'hash', 'guest', 'active')
        """,
        Map.of("id", guestId));

    sweeper.sweep();

    String status =
        jdbc.queryForObject(
            "SELECT status FROM accounts WHERE id = :id", Map.of("id", guestId), String.class);
    assertThat(status).isEqualTo("active");
  }

  @Test
  void sweeperDeletesGuestsInactiveForThirtyDays() {
    UUID guestId = UUID.randomUUID();
    Instant stale = Instant.now().minus(Duration.ofDays(30)).minusSeconds(60);
    jdbc.update(
        """
        INSERT INTO accounts (id, password_hash, type, status, last_online_at)
        VALUES (:id, 'hash', 'guest', 'active', :lastOnline)
        """,
        Map.of("id", guestId, "lastOnline", java.sql.Timestamp.from(stale)));

    sweeper.sweep();

    String status =
        jdbc.queryForObject(
            "SELECT status FROM accounts WHERE id = :id", Map.of("id", guestId), String.class);
    assertThat(status).isEqualTo("deleted");
  }
}
