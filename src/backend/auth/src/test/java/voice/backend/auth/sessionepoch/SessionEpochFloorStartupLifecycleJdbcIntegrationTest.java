package voice.backend.auth.sessionepoch;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.charset.StandardCharsets;
import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.PreparedStatement;
import java.sql.SQLException;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;
import org.flywaydb.core.Flyway;
import org.flywaydb.core.api.callback.Callback;
import org.flywaydb.core.api.callback.Context;
import org.flywaydb.core.api.callback.Event;
import org.junit.jupiter.api.Test;
import org.springframework.boot.autoconfigure.AutoConfigurations;
import org.springframework.boot.autoconfigure.data.redis.RedisAutoConfiguration;
import org.springframework.boot.autoconfigure.flyway.FlywayAutoConfiguration;
import org.springframework.boot.autoconfigure.jdbc.DataSourceAutoConfiguration;
import org.springframework.boot.autoconfigure.jdbc.DataSourceTransactionManagerAutoConfiguration;
import org.springframework.boot.autoconfigure.jdbc.JdbcTemplateAutoConfiguration;
import org.springframework.boot.autoconfigure.flyway.FlywayConfigurationCustomizer;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.beans.BeanWrapperImpl;
import org.springframework.context.SmartLifecycle;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Import;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import voice.backend.auth.config.AuthProperties;
import voice.backend.auth.config.JdbcPersistenceConfiguration;

/** Proves that durable epoch seeding is a startup admission gate, before lifecycle services. */
@Testcontainers(disabledWithoutDocker = true)
class SessionEpochFloorStartupLifecycleJdbcIntegrationTest {
  private static final long EPOCH = 7L;
  private static final ThreadLocal<Scenario> SCENARIO = new ThreadLocal<>();

  @Container
  static final PostgreSQLContainer<?> POSTGRES =
      new PostgreSQLContainer<>(DockerImageName.parse("postgres:16-alpine"))
          .withDatabaseName("auth_startup")
          .withUsername("voice")
          .withPassword("voice");

  @Container
  static final GenericContainer<?> REDIS =
      new GenericContainer<>(DockerImageName.parse("redis:7-alpine")).withExposedPorts(6379);

  @Test
  void enabledFlywaySeedsTheCanonicalRedisFloorBeforeLifecycleStarts() {
    Scenario scenario = new Scenario();

    runWithScenario(
        scenario,
        runnerFor(newSchema(), true, REDIS.getHost(), REDIS.getMappedPort(6379)),
        context -> {
          assertThat(context).hasNotFailed();
          assertThat(scenario.events).containsExactly("migration", "lifecycle-attempt", "lifecycle-start");
          assertThat(context.getBean(SessionEpochFloorStore.class).requireFloor(scenario.accountId))
              .isEqualTo(EPOCH);
        });
  }

  @Test
  void externallyMigratedDatabaseSeedsAndStartsWithoutFlywayInitializer() {
    String schema = newSchema();
    Scenario scenario = new Scenario();
    migrateExternally(schema);
    insertAccount(schema, scenario.accountId);

    runWithScenario(
        scenario,
        runnerFor(schema, false, REDIS.getHost(), REDIS.getMappedPort(6379)),
        context -> {
          assertThat(context).hasNotFailed();
          assertThat(scenario.events).containsExactly("lifecycle-attempt", "lifecycle-start");
          assertThat(context.getBean(SessionEpochFloorStore.class).requireFloor(scenario.accountId))
              .isEqualTo(EPOCH);
        });
  }

  @Test
  void unmigratedExternalDatabaseFailsBeforeLifecycleStarts() {
    Scenario scenario = new Scenario();

    runWithScenario(
        scenario,
        runnerFor(newSchema(), false, REDIS.getHost(), REDIS.getMappedPort(6379)),
        context -> {
          assertThat(context).hasFailed();
          assertThat(scenario.lifecycleAttempts).isZero();
          assertThat(findCause(context.getStartupFailure(), SessionEpochFloorUnavailableException.class))
              .isNotNull();
        });
  }

  @Test
  void unavailableRedisFailsBeforeLifecycleStarts() {
    Scenario scenario = new Scenario();

    runWithScenario(
        scenario,
        runnerFor(newSchema(), true, "127.0.0.1", 1),
        context -> {
          assertThat(context).hasFailed();
          assertThat(scenario.lifecycleAttempts).isZero();
          assertThat(findCause(context.getStartupFailure(), SessionEpochFloorUnavailableException.class))
              .isNotNull();
        });
  }

  @Test
  void defaultSeedPageSizeIs256() {
    Scenario defaultScenario = new Scenario();
    runWithScenario(
        defaultScenario,
        runnerFor(newSchema(), true, REDIS.getHost(), REDIS.getMappedPort(6379)),
        context -> {
          assertThat(context).hasNotFailed();
          assertThat(new BeanWrapperImpl(context.getBean(AuthProperties.class))
                  .getPropertyValue("sessionEpoch.seed.pageSize"))
              .isEqualTo(256);
          assertThat(defaultScenario.events)
              .containsExactly("migration", "lifecycle-attempt", "lifecycle-start");
        });
  }

  @Test
  void zeroSeedPageSizeFailsValidationBeforeLifecycleStarts() {
    assertInvalidPageSize(0);
  }

  @Test
  void negativeSeedPageSizeFailsValidationBeforeLifecycleStarts() {
    assertInvalidPageSize(-1);
  }

  private void assertInvalidPageSize(int pageSize) {
    Scenario scenario = new Scenario();
    runWithScenario(
        scenario,
        runnerFor(newSchema(), true, REDIS.getHost(), REDIS.getMappedPort(6379))
            .withPropertyValues("auth.session-epoch.seed.page-size=" + pageSize),
        context -> {
          assertThat(context).hasFailed();
          assertThat(scenario.lifecycleAttempts).isZero();
          assertThat(findCause(context.getStartupFailure(), IllegalArgumentException.class)).isNotNull();
        });
  }

  private ApplicationContextRunner runnerFor(String schema, boolean flywayEnabled, String redisHost, int redisPort) {
    return new ApplicationContextRunner()
        .withConfiguration(
            AutoConfigurations.of(
                DataSourceAutoConfiguration.class,
                DataSourceTransactionManagerAutoConfiguration.class,
                JdbcTemplateAutoConfiguration.class,
                FlywayAutoConfiguration.class,
                RedisAutoConfiguration.class))
        .withUserConfiguration(StartupTestConfiguration.class)
        .withPropertyValues(
            "auth.persistence=jdbc",
            "spring.datasource.url=" + jdbcUrl(schema),
            "spring.datasource.username=" + POSTGRES.getUsername(),
            "spring.datasource.password=" + POSTGRES.getPassword(),
            "spring.flyway.enabled=" + flywayEnabled,
            "spring.flyway.schemas=" + schema,
            "spring.flyway.default-schema=" + schema,
            "spring.data.redis.host=" + redisHost,
            "spring.data.redis.port=" + redisPort);
  }

  private static void runWithScenario(
      Scenario scenario,
      ApplicationContextRunner runner,
      java.util.function.Consumer<org.springframework.boot.test.context.assertj.AssertableApplicationContext>
          assertion) {
    SCENARIO.set(scenario);
    try {
      runner.run(context -> assertion.accept(context));
    } finally {
      SCENARIO.remove();
    }
  }

  private static String newSchema() {
    return "epoch_startup_" + UUID.randomUUID().toString().replace("-", "");
  }

  private static String jdbcUrl(String schema) {
    return POSTGRES.getJdbcUrl() + "&currentSchema=" + schema;
  }

  private static void migrateExternally(String schema) {
    Flyway.configure()
        .dataSource(jdbcUrl(schema), POSTGRES.getUsername(), POSTGRES.getPassword())
        .schemas(schema)
        .defaultSchema(schema)
        .load()
        .migrate();
  }

  private static void insertAccount(String schema, UUID accountId) {
    String sql =
        "INSERT INTO "
            + schema
            + ".accounts (id, email, password_hash, type, status, totp_enabled, session_epoch) "
            + "VALUES (?, ?, 'hash', 'regular', 'active', false, ?)";
    try (Connection connection = DriverManager.getConnection(jdbcUrl(schema), POSTGRES.getUsername(), POSTGRES.getPassword());
        PreparedStatement statement = connection.prepareStatement(sql)) {
      statement.setObject(1, accountId);
      statement.setString(2, "startup-" + schema + "@example.test");
      statement.setLong(3, EPOCH);
      statement.executeUpdate();
    } catch (SQLException ex) {
      throw new AssertionError("could not prepare external Auth schema", ex);
    }
  }

  @Configuration(proxyBeanMethods = false)
  @EnableConfigurationProperties(AuthProperties.class)
  @Import({JdbcPersistenceConfiguration.class, SessionEpochFloorConfiguration.class})
  static class StartupTestConfiguration {
    @Bean
    FlywayConfigurationCustomizer insertFixedAccountAfterMigrate() {
      return configuration -> configuration.callbacks(new InsertAccountAfterMigrateCallback());
    }

    @Bean
    SmartLifecycle representativeLifecycle(SessionEpochFloorStore floors) {
      return new RepresentativeLifecycle(floors);
    }
  }

  private static final class InsertAccountAfterMigrateCallback implements Callback {
    @Override
    public boolean supports(Event event, Context context) {
      return event == Event.AFTER_MIGRATE;
    }

    @Override
    public boolean canHandleInTransaction(Event event, Context context) {
      return true;
    }

    @Override
    public void handle(Event event, Context context) {
      try (PreparedStatement statement =
          context
              .getConnection()
              .prepareStatement(
                  "INSERT INTO accounts (id, email, password_hash, type, status, totp_enabled, session_epoch) "
                      + "VALUES (?, ?, 'hash', 'regular', 'active', false, ?)")) {
        statement.setObject(1, scenario().accountId);
        statement.setString(2, "startup-flyway@example.test");
        statement.setLong(3, EPOCH);
        statement.executeUpdate();
        scenario().events.add("migration");
      } catch (SQLException ex) {
        throw new IllegalStateException("could not insert migrated epoch fixture", ex);
      }
    }

    @Override
    public String getCallbackName() {
      return "insertFixedAccountAfterMigrate";
    }
  }

  private static final class RepresentativeLifecycle implements SmartLifecycle {
    private final SessionEpochFloorStore floors;
    private boolean running;

    private RepresentativeLifecycle(SessionEpochFloorStore floors) {
      this.floors = floors;
    }

    @Override
    public void start() {
      Scenario scenario = scenario();
      scenario.lifecycleAttempts++;
      scenario.events.add("lifecycle-attempt");
      long floor = floors.requireFloor(scenario.accountId);
      if (floor != EPOCH) {
        throw new IllegalStateException("epoch floor was not seeded before lifecycle start");
      }
      scenario.events.add("lifecycle-start");
      running = true;
    }

    @Override
    public void stop() {
      running = false;
    }

    @Override
    public boolean isRunning() {
      return running;
    }

    @Override
    public boolean isAutoStartup() {
      return true;
    }
  }

  private static Scenario scenario() {
    Scenario scenario = SCENARIO.get();
    if (scenario == null) {
      throw new IllegalStateException("startup scenario was not installed");
    }
    return scenario;
  }

  private static <T extends Throwable> T findCause(Throwable failure, Class<T> type) {
    for (Throwable candidate = failure; candidate != null; candidate = candidate.getCause()) {
      if (type.isInstance(candidate)) {
        return type.cast(candidate);
      }
    }
    return null;
  }

  private static final class Scenario {
    private final UUID accountId = UUID.randomUUID();
    private final List<String> events = new ArrayList<>();
    private int lifecycleAttempts;
  }
}
