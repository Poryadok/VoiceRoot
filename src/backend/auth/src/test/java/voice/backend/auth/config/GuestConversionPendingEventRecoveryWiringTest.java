package voice.backend.auth.config;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.UUID;
import org.junit.jupiter.api.Test;
import org.springframework.boot.autoconfigure.AutoConfigurations;
import org.springframework.boot.test.context.runner.ApplicationContextRunner;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.jdbc.core.namedparam.NamedParameterJdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.jdbc.datasource.DriverManagerDataSource;
import voice.backend.auth.repository.GuestConversionOperation;
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.repository.InMemoryGuestConversionOperationRepository;
import voice.backend.auth.service.GuestConversionEventPublisher;
import voice.backend.auth.service.GuestConversionPendingEventRecoveryRunner;
import voice.backend.auth.service.GuestConversionPendingEventWorker;
import voice.backend.auth.service.UnavailableGuestConversionEventPublisher;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;

class GuestConversionPendingEventRecoveryWiringTest {
  private final ApplicationContextRunner jdbcRuntime =
      new ApplicationContextRunner()
          .withConfiguration(
              AutoConfigurations.of(
                  JdbcPersistenceConfiguration.class,
                  GuestLifecycleConfiguration.class,
                  AuthEventsConfiguration.class))
          .withUserConfiguration(JdbcSupport.class)
          .withPropertyValues(
              "auth.persistence=jdbc",
              "auth.guest-conversion.pending-event.enabled=true",
              "auth.guest-conversion.pending-event.batch-size=9",
              "auth.guest-conversion.pending-event.lease-duration=PT45S",
              "auth.guest-conversion.pending-event.interval=PT7S");

  private final ApplicationContextRunner testMemoryRuntime =
      new ApplicationContextRunner()
          .withInitializer(context -> context.getEnvironment().setActiveProfiles("test"))
          .withConfiguration(
              AutoConfigurations.of(
                  MemoryPersistenceConfiguration.class,
                  GuestLifecycleConfiguration.class,
                  AuthEventsConfiguration.class))
          .withUserConfiguration(MemorySupport.class)
          .withPropertyValues(
              "auth.persistence=memory",
              "auth.guest-conversion.pending-event.enabled=true",
              "auth.guest-conversion.pending-event.batch-size=4",
              "auth.guest-conversion.pending-event.lease-duration=PT30S",
              "auth.guest-conversion.pending-event.interval=PT5S");

  @Test
  void jdbcRuntimeAlwaysWiresAnEnabledRunnerWithTheExplicitUnavailablePublisherWithoutNats() {
    jdbcRuntime.run(
        context -> {
          assertThat(context).hasNotFailed();
          assertThat(context).hasSingleBean(GuestConversionPendingEventWorker.class);
          assertThat(context).hasSingleBean(GuestConversionPendingEventRecoveryRunner.class);
          assertThat(context).hasSingleBean(GuestConversionEventPublisher.class);
          assertThat(context.getBean(GuestConversionEventPublisher.class))
              .isInstanceOf(UnavailableGuestConversionEventPublisher.class);
          GuestConversionPendingEventRecoveryProperties properties =
              context.getBean(GuestConversionPendingEventRecoveryProperties.class);
          assertThat(properties.isEnabled()).isTrue();
          assertThat(properties.getBatchSize()).isEqualTo(9);
          assertThat(properties.getLeaseDuration()).isEqualTo(Duration.ofSeconds(45));
          assertThat(properties.getInterval()).isEqualTo(Duration.ofSeconds(7));
        });
  }

  @Test
  void disabledEventRecoveryDoesNotRegisterAnAccidentalSchedulerFallback() {
    jdbcRuntime
        .withPropertyValues("auth.guest-conversion.pending-event.enabled=false")
        .run(
            context -> {
              assertThat(context).hasNotFailed();
              assertThat(context).doesNotHaveBean(GuestConversionPendingEventRecoveryRunner.class);
            });
  }

  @Test
  void defaultsArePositiveAndInvalidEventRecoveryBoundsFailContextStartup() {
    new ApplicationContextRunner()
        .withConfiguration(
            AutoConfigurations.of(
                JdbcPersistenceConfiguration.class,
                GuestLifecycleConfiguration.class,
                AuthEventsConfiguration.class))
        .withUserConfiguration(JdbcSupport.class)
        .withPropertyValues("auth.persistence=jdbc")
        .run(
            context -> {
              assertThat(context).hasNotFailed();
              GuestConversionPendingEventRecoveryProperties properties =
                  context.getBean(GuestConversionPendingEventRecoveryProperties.class);
              assertThat(properties.isEnabled()).isTrue();
              assertThat(properties.getBatchSize()).isPositive();
              assertThat(properties.getLeaseDuration()).isPositive();
              assertThat(properties.getInterval()).isPositive();
            });

    jdbcRuntime
        .withPropertyValues("auth.guest-conversion.pending-event.batch-size=0")
        .run(context -> assertThat(context).hasFailed());
    jdbcRuntime
        .withPropertyValues("auth.guest-conversion.pending-event.lease-duration=PT0S")
        .run(context -> assertThat(context).hasFailed());
    jdbcRuntime
        .withPropertyValues("auth.guest-conversion.pending-event.interval=PT0S")
        .run(context -> assertThat(context).hasFailed());
  }

  @Test
  void memoryRuntimeWiresTheConcreteRepositoryAndRunnerThroughToCompleted() {
    testMemoryRuntime.run(
        context -> {
          assertThat(context).hasNotFailed();
          assertThat(context).hasSingleBean(InMemoryGuestConversionOperationRepository.class);
          assertThat(context).hasSingleBean(GuestConversionPendingEventWorker.class);
          assertThat(context).hasSingleBean(GuestConversionPendingEventRecoveryRunner.class);

          InMemoryGuestConversionOperationRepository operations =
              context.getBean(InMemoryGuestConversionOperationRepository.class);
          Instant now = Instant.now(context.getBean(Clock.class));
          UUID accountId = UUID.randomUUID();
          GuestConversionOperation pendingUser =
              operations.createOrResume(accountId, UUID.randomUUID(), now);
          GuestConversionOperation leased =
              operations
                  .leaseDue(GuestConversionState.PENDING_USER, 1, now, now.plusSeconds(30))
                  .getFirst();
          operations.advance(
              pendingUser.operationId(),
              GuestConversionState.PENDING_USER,
              leased.lockedUntil(),
              now);

          context.getBean(GuestConversionPendingEventRecoveryRunner.class).tick();

          assertThat(operations.findByAccountId(accountId).orElseThrow().state())
              .isEqualTo(GuestConversionState.COMPLETED);
        });
  }

  @Test
  void testProfileWithNonBlankNatsHasExactlyOneGuestConversionPublisher() {
    testMemoryRuntime
        .withPropertyValues("auth.nats.url=nats://127.0.0.1:4222")
        .run(
            context -> {
              assertThat(context).hasNotFailed();
              assertThat(context).hasSingleBean(GuestConversionEventPublisher.class);
            });
  }

  @Configuration(proxyBeanMethods = false)
  static class JdbcSupport {
    @Bean
    DriverManagerDataSource dataSource() {
      return new DriverManagerDataSource("jdbc:postgresql://localhost:5432/auth_db", "voice", "voice");
    }

    @Bean
    NamedParameterJdbcTemplate jdbc(DriverManagerDataSource dataSource) {
      return new NamedParameterJdbcTemplate(dataSource);
    }

    @Bean
    DataSourceTransactionManager transactions(DriverManagerDataSource dataSource) {
      return new DataSourceTransactionManager(dataSource);
    }

    @Bean
    Clock clock() {
      return Clock.systemUTC();
    }

    @Bean
    PrimaryProfileProvisioner primaryProfiles() {
      return new NoopPrimaryProfiles();
    }

    @Bean
    StringRedisTemplate redis() {
      return mock(StringRedisTemplate.class);
    }

    @Bean
    AuthProperties authProperties() {
      return new AuthProperties();
    }
  }

  @Configuration(proxyBeanMethods = false)
  static class MemorySupport {
    @Bean
    Clock clock() {
      return Clock.systemUTC();
    }

    @Bean
    PrimaryProfileProvisioner primaryProfiles() {
      return new NoopPrimaryProfiles();
    }

    @Bean
    AuthProperties authProperties() {
      return new AuthProperties();
    }
  }

  static final class NoopPrimaryProfiles implements PrimaryProfileProvisioner {
    @Override
    public String ensurePrimaryProfile(UUID accountId, String displayHint, boolean guestAccount) {
      throw new UnsupportedOperationException();
    }

    @Override
    public void clearGuestAccountFlag(UUID accountId) {
    }
  }
}
