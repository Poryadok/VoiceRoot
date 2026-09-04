package voice.backend.auth.config;

import static org.assertj.core.api.Assertions.assertThat;

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
import voice.backend.auth.repository.GuestConversionOperationRepository;
import voice.backend.auth.repository.GuestConversionState;
import voice.backend.auth.repository.InMemoryAccountRepository;
import voice.backend.auth.repository.InMemoryGuestConversionOperationRepository;
import voice.backend.auth.service.InMemoryGuestConversionLocalPromotion;
import voice.backend.auth.service.GuestConversionPendingUserRecoveryRunner;
import voice.backend.auth.service.GuestConversionLocalPromotion;
import voice.backend.auth.service.GuestConversionPendingUserWorker;
import voice.backend.auth.userdb.PrimaryProfileProvisioner;
import static org.mockito.Mockito.mock;

class GuestConversionPendingUserRecoveryWiringTest {
  private final ApplicationContextRunner jdbcRuntime =
      new ApplicationContextRunner()
          .withConfiguration(
              AutoConfigurations.of(
                  JdbcPersistenceConfiguration.class, GuestLifecycleConfiguration.class))
          .withUserConfiguration(JdbcSupport.class)
          .withPropertyValues(
              "auth.persistence=jdbc",
              "auth.guest-conversion.pending-user.enabled=true",
              "auth.guest-conversion.pending-user.batch-size=9",
              "auth.guest-conversion.pending-user.lease-duration=PT45S",
              "auth.guest-conversion.pending-user.interval=PT7S");

  private final ApplicationContextRunner memoryRuntime =
      new ApplicationContextRunner()
          .withConfiguration(
              AutoConfigurations.of(
                  MemoryPersistenceConfiguration.class, GuestLifecycleConfiguration.class))
          .withUserConfiguration(MemorySupport.class)
          .withPropertyValues(
              "auth.persistence=memory",
              "auth.guest-conversion.pending-user.enabled=true",
              "auth.guest-conversion.pending-user.batch-size=4",
              "auth.guest-conversion.pending-user.lease-duration=PT30S",
              "auth.guest-conversion.pending-user.interval=PT5S");

  @Test
  void jdbcRuntimeWiresAnEnabledRecoveryRunnerWithConfiguredBounds() {
    jdbcRuntime.run(
        context -> {
          assertThat(context).hasNotFailed();
          assertThat(context).hasSingleBean(GuestConversionPendingUserRecoveryRunner.class);
          GuestConversionPendingUserRecoveryProperties properties =
              context.getBean(GuestConversionPendingUserRecoveryProperties.class);
          assertThat(properties.isEnabled()).isTrue();
          assertThat(properties.getBatchSize()).isEqualTo(9);
          assertThat(properties.getLeaseDuration()).isEqualTo(Duration.ofSeconds(45));
          assertThat(properties.getInterval()).isEqualTo(Duration.ofSeconds(7));
        });
  }

  @Test
  void disabledRecoveryDoesNotRegisterAnAccidentalSchedulerFallback() {
    jdbcRuntime
        .withPropertyValues("auth.guest-conversion.pending-user.enabled=false")
        .run(
            context -> {
              assertThat(context).hasNotFailed();
              assertThat(context).doesNotHaveBean(GuestConversionPendingUserRecoveryRunner.class);
            });
  }

  @Test
  void memoryRuntimeWiresTheSameRecoveryPathAsJdbc() {
    memoryRuntime.run(
        context -> {
          assertThat(context).hasNotFailed();
          assertThat(context).hasSingleBean(GuestConversionOperationRepository.class);
          assertThat(context).hasSingleBean(InMemoryGuestConversionOperationRepository.class);
          assertThat(context).hasSingleBean(GuestConversionLocalPromotion.class);
          assertThat(context).hasSingleBean(InMemoryGuestConversionLocalPromotion.class);
          assertThat(context).hasSingleBean(GuestConversionPendingUserWorker.class);
          assertThat(context).hasSingleBean(GuestConversionPendingUserRecoveryRunner.class);
          GuestConversionPendingUserRecoveryProperties properties =
              context.getBean(GuestConversionPendingUserRecoveryProperties.class);
          assertThat(properties.isEnabled()).isTrue();
          assertThat(properties.getBatchSize()).isEqualTo(4);
          assertThat(properties.getLeaseDuration()).isEqualTo(Duration.ofSeconds(30));
          assertThat(properties.getInterval()).isEqualTo(Duration.ofSeconds(5));
        });
  }

  @Test
  void memoryRunnerUsesTheConcreteMemoryPromotionToAdvanceARealPendingUserOperation() {
    memoryRuntime.run(
        context -> {
          InMemoryAccountRepository accounts = context.getBean(InMemoryAccountRepository.class);
          InMemoryGuestConversionOperationRepository operations =
              context.getBean(InMemoryGuestConversionOperationRepository.class);
          var guest = accounts.create("wired-memory@example.com", null, "hash", "guest");
          Instant now = Instant.now(context.getBean(Clock.class));
          operations.createOrResume(guest.id(), UUID.randomUUID(), now);

          context.getBean(GuestConversionPendingUserRecoveryRunner.class).tick();

          assertThat(context.getBean(RecordingPrimaryProfiles.class).clearedAccountIds)
              .containsExactly(guest.id());
          assertThat(accounts.findById(guest.id().toString()).orElseThrow().type())
              .isEqualTo("regular");
          assertThat(operations.findByAccountId(guest.id()).orElseThrow().state())
              .isEqualTo(GuestConversionState.PENDING_EVENT);
        });
  }

  @Test
  void recoveryDefaultsAreEnabledAndBoundedWhileInvalidBoundsFailContextStartup() {
    new ApplicationContextRunner()
        .withConfiguration(
            AutoConfigurations.of(MemoryPersistenceConfiguration.class, GuestLifecycleConfiguration.class))
        .withUserConfiguration(MemorySupport.class)
        .withPropertyValues("auth.persistence=memory")
        .run(
            context -> {
              assertThat(context).hasNotFailed();
              GuestConversionPendingUserRecoveryProperties properties =
                  context.getBean(GuestConversionPendingUserRecoveryProperties.class);
              assertThat(properties.isEnabled()).isTrue();
              assertThat(properties.getBatchSize()).isPositive();
              assertThat(properties.getLeaseDuration()).isPositive();
              assertThat(properties.getInterval()).isPositive();
            });

    memoryRuntime
        .withPropertyValues("auth.guest-conversion.pending-user.batch-size=0")
        .run(context -> assertThat(context).hasFailed());
    memoryRuntime
        .withPropertyValues("auth.guest-conversion.pending-user.lease-duration=PT0S")
        .run(context -> assertThat(context).hasFailed());
    memoryRuntime
        .withPropertyValues("auth.guest-conversion.pending-user.interval=PT0S")
        .run(context -> assertThat(context).hasFailed());
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
      return new PrimaryProfileProvisioner() {
        @Override
        public String ensurePrimaryProfile(UUID accountId, String displayHint, boolean guestAccount) {
          throw new UnsupportedOperationException();
        }

        @Override
        public void clearGuestAccountFlag(UUID accountId) {
          throw new UnsupportedOperationException();
        }
      };
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
    RecordingPrimaryProfiles primaryProfiles() {
      return new RecordingPrimaryProfiles();
    }
  }

  static final class RecordingPrimaryProfiles implements PrimaryProfileProvisioner {
    private final java.util.List<UUID> clearedAccountIds = new java.util.ArrayList<>();

    @Override
    public String ensurePrimaryProfile(UUID accountId, String displayHint, boolean guestAccount) {
      throw new UnsupportedOperationException();
    }

    @Override
    public void clearGuestAccountFlag(UUID accountId) {
      clearedAccountIds.add(accountId);
    }
  }
}
