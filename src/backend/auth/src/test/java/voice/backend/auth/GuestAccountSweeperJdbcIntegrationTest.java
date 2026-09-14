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
  void flywayV5AddsNullableLastOnlineAtAndActiveGuestPartialIndex() {
    Map<String, Object> column =
        jdbc.queryForMap(
            """
            SELECT data_type, is_nullable
            FROM information_schema.columns
            WHERE table_schema = current_schema()
              AND table_name = 'accounts'
              AND column_name = 'last_online_at'
            """,
            Map.of());

    assertThat(column)
        .containsEntry("data_type", "timestamp with time zone")
        .containsEntry("is_nullable", "YES");

    Map<String, Object> migration =
        jdbc.queryForMap(
            """
            SELECT version, script, success
            FROM flyway_schema_history
            WHERE version = '5'
            """,
            Map.of());
    assertThat(migration)
        .containsEntry("version", "5")
        .containsEntry("script", "V5__accounts_last_online_at.sql")
        .containsEntry("success", true);

    String indexDefinition =
        jdbc.queryForObject(
            """
            SELECT pg_get_indexdef(indexrelid)
            FROM pg_index
            JOIN pg_class ON pg_class.oid = indexrelid
            WHERE relname = 'accounts_guest_last_online_idx'
            """,
            Map.of(),
            String.class);
    String predicate =
        jdbc.queryForObject(
            """
            SELECT pg_get_expr(indpred, indrelid)
            FROM pg_index
            JOIN pg_class ON pg_class.oid = indexrelid
            WHERE relname = 'accounts_guest_last_online_idx'
            """,
            Map.of(),
            String.class);

    assertThat(indexDefinition).contains("last_online_at");
    assertThat(predicate).containsPattern("(?is)type.*guest.*\\bAND\\b.*status.*active");
  }

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
    assertThat(
            jdbc.queryForObject(
                "SELECT last_online_at FROM accounts WHERE id = :id",
                Map.of("id", guestId),
                java.sql.Timestamp.class))
        .isNull();
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
    assertThat(
            jdbc.queryForObject(
                "SELECT deleted_at FROM accounts WHERE id = :id",
                Map.of("id", guestId),
                java.sql.Timestamp.class))
        .isNotNull();
  }

  @Test
  void repeatedSweepDoesNotModifyAlreadyDeletedGuest() {
    UUID guestId = UUID.randomUUID();
    Instant stale = Instant.now().minus(Duration.ofDays(30)).minusSeconds(60);
    jdbc.update(
        """
        INSERT INTO accounts (id, password_hash, type, status, last_online_at)
        VALUES (:id, 'hash', 'guest', 'active', :lastOnline)
        """,
        Map.of("id", guestId, "lastOnline", java.sql.Timestamp.from(stale)));

    sweeper.sweep();
    java.sql.Timestamp deletedAtAfterFirstSweep =
        jdbc.queryForObject(
            "SELECT deleted_at FROM accounts WHERE id = :id",
            Map.of("id", guestId),
            java.sql.Timestamp.class);
    assertThat(deletedAtAfterFirstSweep).isNotNull();

    sweeper.sweep();

    String status =
        jdbc.queryForObject(
            "SELECT status FROM accounts WHERE id = :id", Map.of("id", guestId), String.class);
    java.sql.Timestamp deletedAtAfterSecondSweep =
        jdbc.queryForObject(
            "SELECT deleted_at FROM accounts WHERE id = :id",
            Map.of("id", guestId),
            java.sql.Timestamp.class);
    assertThat(status).isEqualTo("deleted");
    assertThat(deletedAtAfterSecondSweep).isEqualTo(deletedAtAfterFirstSweep);
  }
}
