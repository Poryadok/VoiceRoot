package voice.backend.auth;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.Map;
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
import voice.backend.auth.support.JdbcUserContractTestConfiguration;

/**
 * T-049b RED integration proof that Flyway retains recoverable guest conversion work in Auth.
 *
 * <p>The operation is Auth-owned: User changes its guest flag through RPC, but only Auth can
 * atomically retain the account promotion and the pending event/retry state. This runs the actual
 * Flyway path against PostgreSQL; the file-level lockstep test covers the optional golang-migrate
 * path separately.
 */
@SpringBootTest
@ActiveProfiles("integration")
@Testcontainers(disabledWithoutDocker = true)
@Import(JdbcUserContractTestConfiguration.class)
class GuestConversionDurabilityJdbcIntegrationTest {
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

  @Test
  void flywayCreatesAnAuthOwnedDurableGuestConversionOperationTable() {
    Integer tables =
        jdbc.queryForObject(
            """
            SELECT COUNT(*)::int
            FROM information_schema.tables
            WHERE table_schema = 'public' AND table_name = 'guest_conversion_operations'
            """,
            Map.of(),
            Integer.class);

    assertThat(tables)
        .as("verified guest conversion must retain retryable work across an Auth restart")
        .isEqualTo(1);
  }
}
